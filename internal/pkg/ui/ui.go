package ui

import (
	"fmt"
	"os"
	"os/user"
	"time"

	sso "github.com/JorgeReus/aws-sso-creds/internal/app"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/bus"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/util"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	te "github.com/muesli/termenv"
)

type UI struct {
	CreateStatic  bool
	PopulateRoles bool
	ForceLogin    bool
	NoBrowser     bool
	UsePreviewer  bool
	Org           config.Organization
}

var (
	c                 config.Config
	errorColor        te.Color
	informationColor  te.Color
	warningColor      te.Color
	focusColor        te.Color
	spinnerColor      te.Color
	flow              *sso.SSOFlow
	hasFinished       bool
	needsUserApproval = false
	displayMsg        string
	color             = te.ColorProfile().Color
	msgBus            = bus.NewBus()
)

func printErr(err error) {
	fmt.Println(te.String("Error: " + err.Error()).Foreground(errorColor).String())
}

func printWarning(warn string) {
	fmt.Println(te.String("Warning: " + warn).Foreground(warningColor).String())
}

type model struct {
	spinner spinner.Model
}

func initialModel() model {
	s := spinner.NewModel()
	s.Spinner = spinner.Dot
	return model{spinner: s}
}

func (u *UI) Start() error {
	c := config.GetInstance()
	errorColor = color(c.ErrorColor)
	informationColor = color(c.InformationColor)
	warningColor = color(c.WarningColor)
	focusColor = color(c.FocusColor)
	spinnerColor = color(c.SpinnerColor)

	user, err := user.Current()
	if err != nil {
		printErr(err)
		os.Exit(1)
	}

	home := config.GetInstance().Home
	credentialsPath := fmt.Sprintf("%s/.aws/credentials", home)
	configFilePath := fmt.Sprintf("%s/.aws/config", home)

	// Validate if the credentials file is owned by root
	if u.CreateStatic {
		util.ValidateSuperuserFile(credentialsPath, user)
	}

	// Validate if the config file is owned by root
	if u.PopulateRoles {
		util.ValidateSuperuserFile(configFilePath, user)
	}

	// Preview de credentials & profiles
	if u.UsePreviewer {
		fp, err := NewFuzzyPreviewer(credentialsPath, configFilePath)
		if err != nil {
			fmt.Println(fmt.Sprintf("Error starting program: %s", err))
			os.Exit(1)
		}
		selectedEntry, err := fp.Preview()
		if err != nil {
			fmt.Println(fmt.Sprintf("Error Selecting entry: %s", err))
			os.Exit(1)
		}
		fmt.Println(*selectedEntry)
		return nil
	}

	m := initialModel()
	p := tea.NewProgram(m)
	go u.handleFlow()
	if err := p.Start(); err != nil {
		fmt.Println(fmt.Sprintf("Error starting program: %s", err))
		os.Exit(1)
	}
	return nil
}

func (m model) View() string {
	s := te.String(m.spinner.View()).Foreground(spinnerColor).String() + displayMsg
	return s
}

func (m model) Init() tea.Cmd {
	return spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if needsUserApproval {
				msgBus.Send(bus.BusMsg{
					MsgType:  bus.MSG_TYPE_CONT,
					Contents: "",
				})
			}
			return m, nil
		default:
			return m, nil
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		if hasFinished {
			return m, tea.Quit
		}
		return m, cmd
	default:
		return m, cmd
	}
}

func channelSubscriber() {
	for {
		msg := msgBus.Recv()
		switch msg.MsgType {
		case bus.MSG_TYPE_INFO:
			fmt.Println(te.String(msg.Contents).Foreground(informationColor).String())
			break
		case bus.MSG_TYPE_ERR:
			printWarning(msg.Contents)
			displayMsg = te.String("Continue in your browser and press ENTER").
				Foreground(informationColor).
				String()
			needsUserApproval = true
			break
		default:
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
}

func (u *UI) handleFlow() {
	displayMsg = te.String(fmt.Sprintf("Logging in into AWS SSO org: %s", u.Org.Name)).
		Foreground(informationColor).
		String()
	var err error
	go channelSubscriber()
	flow, err = sso.Login(
		u.Org,
		u.ForceLogin,
		u.NoBrowser,
		msgBus,
	)
	if err != nil {
		hasFinished = true
		printErr(err)
		return
	}

	isNewRun := files.ConfigFileSSOEmpty(config.GetInstance().Home, u.Org.Name)

	if u.PopulateRoles || isNewRun {
		displayMsg = "Updating roles"
		fmt.Println("Synced roles in ~/.aws/config")
		roles, err := flow.PopulateRoles()
		if err != nil {
			hasFinished = true
			printErr(err)
			return
		}
		for _, role := range roles {
			if role != "DEFAULT" {
				s := fmt.Sprintf("  SSO Role %s", te.String(role).Foreground(focusColor).String())
				fmt.Println(s)
			}
		}
		fmt.Println()
	}

	if u.CreateStatic {
		displayMsg = te.String("Getting static credentials").Foreground(informationColor).String()
		roles, err := flow.GetCredentials()
		if err != nil {
			hasFinished = true
			printErr(err)
			return
		}
		displayMsg = "Updating static credentials"
		fmt.Println("Added temporary credentials in ~/.aws/credentials")
		for _, role := range roles {
			if role.WasSuccesful {
				s := fmt.Sprintf(
					"  %s, will expire at %s",
					te.String(role.ProfileName).Foreground(focusColor).String(),
					te.String(role.ExpiresAt).Foreground(focusColor).String(),
				)
				fmt.Println(s)
			} else {
				s := fmt.Sprintf("Entry error %s in ~/.aws/config, try to update your roles with -r", te.String(role.ProfileName).Foreground(errorColor).String())
				fmt.Println(s)
			}
		}
		fmt.Println()
	}

	if (u.PopulateRoles && u.CreateStatic) || isNewRun {
		exportRolesString := fmt.Sprintf("You can use aws-sso-creds -l to select your profile")
		fmt.Println(exportRolesString)
	}
	hasFinished = true
	return
}

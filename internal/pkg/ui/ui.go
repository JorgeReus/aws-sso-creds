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

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	te "github.com/muesli/termenv"
)

type UI struct {
	CreateStatic  bool
	PopulateRoles bool
	SsoURL        string
	SsoRegion     string
	ForceLogin    bool
	UsePreviewer  bool
}

var (
	flow              *sso.SSOFlow
	hasFinished       bool
	needsUserApproval = false
	displayMsg        string
	color             = te.ColorProfile().Color
	errorColor        = color("9")
	informationColor  = color("20")
	warningColor      = color("22")
	focusColor        = color("10")
	spinnerColor      = color("205")
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

	user, err := user.Current()
	if err != nil {
		printErr(err)
		os.Exit(1)
	}

	credentialsPath := fmt.Sprintf("%s/.aws/credentials", user.HomeDir)
	configFilePath := fmt.Sprintf("%s/.aws/config", user.HomeDir)

	// Validate if the credentials file is owned by root
	if u.CreateStatic {
		util.ValidateSuperuserFile(credentialsPath, user)
	}

	// Validate if the config file is owned by root
	if u.PopulateRoles {
		util.ValidateSuperuserFile(configFilePath, user)
	}

	if u.UsePreviewer {
		selectedEntry := fuzzyPreviewer(credentialsPath, configFilePath)
		fmt.Println(selectedEntry)
	} else {
		m := initialModel()
		p := tea.NewProgram(m)
		go u.handleFlow()
		if err := p.Start(); err != nil {
			fmt.Printf("Error starting program: %s\n", err)
			os.Exit(1)
		}
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
			displayMsg = te.String("Continue in your browser and press ENTER").Foreground(informationColor).String()
			needsUserApproval = true
			break
		default:
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
}

func (u *UI) handleFlow() {
	displayMsg = te.String("Logging in into AWS SSO").Foreground(informationColor).String()
	var err error
	go channelSubscriber()
	flow, err = sso.Login(u.SsoURL, u.SsoRegion, u.ForceLogin, msgBus)
	if err != nil {
		hasFinished = true
		printErr(err)
		return
	}

	isNewRun := files.ConfigFileSSOEmpty()

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
				s := fmt.Sprintf("  %s, will expire at %s", te.String(role.ProfileName).Foreground(focusColor).String(), te.String(role.ExpiresAt).Foreground(focusColor).String())
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

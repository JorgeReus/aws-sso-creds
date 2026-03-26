package ui

import (
	"fmt"
	"os/user"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	te "github.com/muesli/termenv"

	sso "github.com/JorgeReus/aws-sso-creds/internal/app"
	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/bus"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/util"
)

type UI struct {
	CreateStatic  bool
	PopulateRoles bool
	ForceLogin    bool
	NoBrowser     bool
	Org           config.Organization
}

type flowAPI interface {
	PopulateRoles() ([]string, error)
	GetCredentials() ([]sso.CredentialsResult, error)
}

type programRunner interface {
	Start() error
}

type uiDeps struct {
	currentUser           func() (*user.User, error)
	validateSuperuserFile func(string, *user.User) string
	newProgram            func(tea.Model) programRunner
	login                 func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error)
	configFileSSOEmpty    func(string, string) bool
	println               func(...interface{}) (int, error)
	sleep                 func(time.Duration)
	startSubscriber       func(uiDeps)
	startFlow             func(UI, uiDeps)
}

var (
	errorColor       te.Color
	informationColor te.Color
	warningColor     te.Color
	focusColor       te.Color
	spinnerColor     te.Color
	flow             flowAPI
	hasFinished      atomic.Bool

	needsUserApproval atomic.Bool
	displayMsgMu      sync.RWMutex
	displayMsg        string
	outputLines       []string
	outputLinesMu     sync.RWMutex
	color             = te.ColorProfile().Color
	msgBus            = bus.NewBus()
)

var uiDepsFactory = defaultUIDeps

func defaultUIDeps() uiDeps {
	deps := uiDeps{
		currentUser:           user.Current,
		validateSuperuserFile: util.ValidateSuperuserFile,
		newProgram: func(m tea.Model) programRunner {
			return tea.NewProgram(m)
		},
		login: func(org config.Organization, forceLogin, noBrowser bool, msgBus *bus.Bus) (flowAPI, error) {
			return sso.Login(org, forceLogin, noBrowser, msgBus)
		},
		configFileSSOEmpty: files.ConfigFileSSOEmpty,
		println:            fmt.Println,
		sleep:              time.Sleep,
	}
	deps.startSubscriber = channelSubscriberWithDeps
	deps.startFlow = handleFlowWithDeps
	return deps
}

func resetUIStateForTest() {
	hasFinished.Store(false)
	needsUserApproval.Store(false)
	setDisplayMsg("")
	outputLinesMu.Lock()
	outputLines = nil
	outputLinesMu.Unlock()
	msgBus = bus.NewBus()
	flow = nil
}

func setDisplayMsg(msg string) {
	displayMsgMu.Lock()
	defer displayMsgMu.Unlock()
	displayMsg = msg
}

func getDisplayMsg() string {
	displayMsgMu.RLock()
	defer displayMsgMu.RUnlock()
	return displayMsg
}

func appendOutputLine(line string) {
	outputLinesMu.Lock()
	defer outputLinesMu.Unlock()
	outputLines = append(outputLines, line)
}

func renderedOutputLines() string {
	outputLinesMu.RLock()
	defer outputLinesMu.RUnlock()
	return strings.Join(outputLines, "\n")
}

func printErr(err error) string {
	return te.String("Error: " + err.Error()).Foreground(errorColor).String()
}

func printWarning(warn string) string {
	return te.String("Warning: " + warn).Foreground(warningColor).String()
}

type model struct {
	spinner spinner.Model
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{spinner: s}
}

func (u *UI) Start() error {
	return startWithDeps(*u, uiDepsFactory())
}

func startWithDeps(u UI, deps uiDeps) error {
	cfg := config.GetInstance()
	errorColor = color(cfg.ErrorColor)
	informationColor = color(cfg.InformationColor)
	warningColor = color(cfg.WarningColor)
	focusColor = color(cfg.FocusColor)
	spinnerColor = color(cfg.SpinnerColor)

	currentUser, err := deps.currentUser()
	if err != nil {
		_, _ = deps.println(printErr(err))
		return err
	}

	home := config.GetInstance().Home
	credentialsPath := fmt.Sprintf("%s/.aws/credentials", home)
	configFilePath := fmt.Sprintf("%s/.aws/config", home)

	if u.CreateStatic {
		deps.validateSuperuserFile(credentialsPath, currentUser)
	}

	if u.PopulateRoles {
		deps.validateSuperuserFile(configFilePath, currentUser)
	}

	m := initialModel()
	p := deps.newProgram(m)
	if deps.startFlow != nil {
		go deps.startFlow(u, deps)
	}
	if err := p.Start(); err != nil {
		_, _ = deps.println(fmt.Sprintf("Error starting program: %s", err))
		return err
	}
	return nil
}

func (m model) View() string {
	if hasFinished.Load() {
		return renderedOutputLines()
	}

	output := renderedOutputLines()
	msg := getDisplayMsg()
	if output == "" {
		return te.String(m.spinner.View()).Foreground(spinnerColor).String() + msg
	}
	return te.String(m.spinner.View()).Foreground(spinnerColor).String() + msg + "\n" + output
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if needsUserApproval.Load() {
				msgBus.Send(bus.BusMsg{MsgType: bus.MSG_TYPE_CONT, Contents: ""})
			}
			return m, nil
		default:
			return m, nil
		}
	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		if hasFinished.Load() {
			return m, tea.Quit
		}
		return m, cmd
	default:
		return m, cmd
	}
}

func handleBusMessage(msg bus.BusMsg, deps uiDeps) {
	switch msg.MsgType {
	case bus.MSG_TYPE_INFO:
		appendOutputLine(te.String(msg.Contents).Foreground(informationColor).String())
	case bus.MSG_TYPE_ERR:
		appendOutputLine(printWarning(msg.Contents))
		setDisplayMsg(
			te.String("Continue in your browser and press ENTER").
				Foreground(informationColor).
				String(),
		)
		needsUserApproval.Store(true)
	default:
	}
}

func channelSubscriberWithDeps(deps uiDeps) {
	for {
		msg := msgBus.Recv()
		handleBusMessage(msg, deps)
		deps.sleep(time.Millisecond * 200)
	}
}

func (u *UI) handleFlow() {
	handleFlowWithDeps(*u, uiDepsFactory())
}

func handleFlowWithDeps(u UI, deps uiDeps) {
	setDisplayMsg(te.String(fmt.Sprintf("Logging in into AWS SSO org: %s", u.Org.Name)).
		Foreground(informationColor).
		String())

	if deps.startSubscriber != nil {
		go deps.startSubscriber(deps)
	}

	var err error
	flow, err = deps.login(u.Org, u.ForceLogin, u.NoBrowser, msgBus)
	if err != nil {
		appendOutputLine(printErr(err))
		hasFinished.Store(true)
		return
	}

	isNewRun := deps.configFileSSOEmpty(config.GetInstance().Home, u.Org.Name)

	if u.PopulateRoles || isNewRun {
		setDisplayMsg("Updating roles")
		appendOutputLine("Synced roles in ~/.aws/config")
		roles, err := flow.PopulateRoles()
		if err != nil {
			appendOutputLine(printErr(err))
			hasFinished.Store(true)
			return
		}
		for _, role := range roles {
			if role != "DEFAULT" {
				appendOutputLine(
					fmt.Sprintf("  SSO Role %s", te.String(role).Foreground(focusColor).String()),
				)
			}
		}
		appendOutputLine("")
	}

	if u.CreateStatic {
		setDisplayMsg(te.String("Getting static credentials").Foreground(informationColor).String())
		roles, err := flow.GetCredentials()
		if err != nil {
			appendOutputLine(printErr(err))
			hasFinished.Store(true)
			return
		}
		setDisplayMsg("Updating static credentials")
		appendOutputLine("Added temporary credentials in ~/.aws/credentials")
		for _, role := range roles {
			if role.WasSuccesful {
				appendOutputLine(fmt.Sprintf(
					"  %s, will expire at %s",
					te.String(role.ProfileName).Foreground(focusColor).String(),
					te.String(role.ExpiresAt).Foreground(focusColor).String(),
				))
			} else {
				appendOutputLine(fmt.Sprintf(
					"Entry error %s in ~/.aws/config, try to update your roles with -r",
					te.String(role.ProfileName).Foreground(errorColor).String(),
				))
			}
		}
		appendOutputLine("")
	}

	if (u.PopulateRoles && u.CreateStatic) || isNewRun {
		appendOutputLine("You can use aws-sso-creds -l to select your profile")
	}
	hasFinished.Store(true)
}

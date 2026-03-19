package ui

import (
	"errors"
	"fmt"
	"os/user"
	"reflect"
	"strings"
	"testing"
	"time"

	sso "github.com/JorgeReus/aws-sso-creds/internal/app"
	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/bus"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeProgram struct {
	startErr error
	started  bool
}

func (f *fakeProgram) Start() error {
	f.started = true
	return f.startErr
}

type fakeFlow struct {
	populateRolesResult []string
	populateRolesErr    error
	credentialsResult   []sso.CredentialsResult
	credentialsErr      error
}

func (f fakeFlow) PopulateRoles() ([]string, error) {
	return f.populateRolesResult, f.populateRolesErr
}

func (f fakeFlow) GetCredentials() ([]sso.CredentialsResult, error) {
	return f.credentialsResult, f.credentialsErr
}

func TestStartWithDepsInitializesAndRunsProgram(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	prog := &fakeProgram{}
	validateCalls := 0
	err := startWithDeps(UI{CreateStatic: true, PopulateRoles: true, Org: config.Organization{Name: "dev"}}, uiDeps{
		currentUser: func() (*user.User, error) { return &user.User{Uid: "1000"}, nil },
		validateSuperuserFile: func(string, *user.User) string {
			validateCalls++
			return ""
		},
		newProgram:         func(tea.Model) programRunner { return prog },
		login:              func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error) { return fakeFlow{}, nil },
		configFileSSOEmpty: func(string, string) bool { return false },
		println:            func(...interface{}) (int, error) { return 0, nil },
		sleep:              func(time.Duration) {},
	})
	if err != nil {
		t.Fatalf("startWithDeps() error = %v", err)
	}
	if !prog.started {
		t.Fatal("program was not started")
	}
	if validateCalls != 2 {
		t.Fatalf("validateCalls = %d, want 2", validateCalls)
	}
}

func TestModelViewIncludesDisplayMessage(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	displayMsg = "continue"
	view := initialModel().View()
	if !strings.Contains(view, "continue") {
		t.Fatalf("View() = %q, want display message", view)
	}
}

func TestModelInitReturnsSpinnerTick(t *testing.T) {
	cmd := initialModel().Init()
	if cmd == nil {
		t.Fatal("Init() returned nil")
	}
	if _, ok := cmd().(spinner.TickMsg); !ok {
		t.Fatalf("Init() command returned %T, want spinner.TickMsg", cmd())
	}
}

func TestStartWithDepsReturnsCurrentUserError(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	wantErr := errors.New("no user")
	err := startWithDeps(UI{}, uiDeps{
		currentUser:           func() (*user.User, error) { return nil, wantErr },
		validateSuperuserFile: func(string, *user.User) string { return "" },
		newProgram:            func(tea.Model) programRunner { return &fakeProgram{} },
		login:                 func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error) { return fakeFlow{}, nil },
		configFileSSOEmpty:    func(string, string) bool { return false },
		println:               func(...interface{}) (int, error) { return 0, nil },
		sleep:                 func(time.Duration) {},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("startWithDeps() error = %v, want %v", err, wantErr)
	}
}

func TestModelUpdateSendsContinueMessageOnEnterWhenApprovalNeeded(t *testing.T) {
	resetUIStateForTest()
	needsUserApproval = true
	msgBus = &bus.Bus{Channel: make(chan bus.BusMsg, 1)}

	m := initialModel()
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	msg := <-msgBus.Channel
	if msg.MsgType != bus.MSG_TYPE_CONT {
		t.Fatalf("msg.MsgType = %d, want %d", msg.MsgType, bus.MSG_TYPE_CONT)
	}
}

func TestModelUpdateQuitsOnCtrlC(t *testing.T) {
	resetUIStateForTest()

	_, cmd := initialModel().Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Update() returned nil command for ctrl+c")
	}
	if got, want := reflect.TypeOf(cmd()), reflect.TypeOf(tea.Quit()); got != want {
		t.Fatalf("Update() command type = %v, want %v", got, want)
	}
}

func TestModelUpdateQuitsOnFinishedSpinnerTick(t *testing.T) {
	resetUIStateForTest()
	hasFinished = true

	_, cmd := initialModel().Update(spinner.TickMsg{})
	if cmd == nil {
		t.Fatal("Update() returned nil command for finished spinner tick")
	}
	if got, want := reflect.TypeOf(cmd()), reflect.TypeOf(tea.Quit()); got != want {
		t.Fatalf("Update() command type = %v, want %v", got, want)
	}
}

func TestHandleBusMessagePrintsInfoMessage(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	var printed string
	handleBusMessage(bus.BusMsg{MsgType: bus.MSG_TYPE_INFO, Contents: "hello"}, uiDeps{
		println: func(args ...interface{}) (int, error) {
			printed = args[0].(string)
			return 0, nil
		},
		sleep: func(time.Duration) {},
	})
	if !strings.Contains(printed, "hello") {
		t.Fatalf("printed = %q, want info message", printed)
	}
}

func TestHandleBusMessageSetsApprovalState(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	var printed string
	handleBusMessage(bus.BusMsg{MsgType: bus.MSG_TYPE_ERR, Contents: "open browser"}, uiDeps{
		println: func(args ...interface{}) (int, error) {
			printed = args[0].(string)
			return 0, nil
		},
		sleep: func(time.Duration) {},
	})
	if !needsUserApproval {
		t.Fatal("needsUserApproval = false, want true")
	}
	if !strings.Contains(printed, "Warning: open browser") {
		t.Fatalf("printed = %q, want warning message", printed)
	}
}

func TestHandleFlowWithDepsPopulatesRolesAndCredentials(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	var lines []string
	handleFlowWithDeps(UI{
		CreateStatic:  true,
		PopulateRoles: true,
		Org:           config.Organization{Name: "dev"},
	}, uiDeps{
		currentUser:           func() (*user.User, error) { return &user.User{Uid: "1000"}, nil },
		validateSuperuserFile: func(string, *user.User) string { return "" },
		newProgram:            func(tea.Model) programRunner { return &fakeProgram{} },
		login: func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error) {
			return fakeFlow{
				populateRolesResult: []string{"DEFAULT", "dev:Admin"},
				credentialsResult: []sso.CredentialsResult{{
					ProfileName:  "tmp:dev:Admin",
					ExpiresAt:    "tomorrow",
					WasSuccesful: true,
				}},
			}, nil
		},
		configFileSSOEmpty: func(string, string) bool { return true },
		println: func(args ...interface{}) (int, error) {
			lines = append(lines, strings.TrimSpace(fmt.Sprint(args...)))
			return 0, nil
		},
		sleep: func(time.Duration) {},
	})
	if !hasFinished {
		t.Fatal("hasFinished = false, want true")
	}
	if len(lines) == 0 {
		t.Fatal("expected printed output")
	}
}

func TestHandleFlowWithDepsPrintsLoginError(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)

	var printed string
	handleFlowWithDeps(UI{Org: config.Organization{Name: "dev"}}, uiDeps{
		currentUser:           func() (*user.User, error) { return &user.User{Uid: "1000"}, nil },
		validateSuperuserFile: func(string, *user.User) string { return "" },
		newProgram:            func(tea.Model) programRunner { return &fakeProgram{} },
		login: func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error) {
			return nil, errors.New("boom")
		},
		configFileSSOEmpty: func(string, string) bool { return false },
		println: func(args ...interface{}) (int, error) {
			printed = args[0].(string)
			return 0, nil
		},
		sleep: func(time.Duration) {},
	})
	if !strings.Contains(printed, "Error: boom") {
		t.Fatalf("printed = %q, want login error", printed)
	}
}

func TestStartWrapperUsesFactoryDeps(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)
	origFactory := uiDepsFactory
	defer func() { uiDepsFactory = origFactory }()

	prog := &fakeProgram{}
	uiDepsFactory = func() uiDeps {
		return uiDeps{
			currentUser:           func() (*user.User, error) { return &user.User{Uid: "1000"}, nil },
			validateSuperuserFile: func(string, *user.User) string { return "" },
			newProgram:            func(tea.Model) programRunner { return prog },
			login:                 func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error) { return fakeFlow{}, nil },
			configFileSSOEmpty:    func(string, string) bool { return false },
			println:               func(...interface{}) (int, error) { return 0, nil },
			sleep:                 func(time.Duration) {},
		}
	}

	if err := (&UI{Org: config.Organization{Name: "dev"}}).Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !prog.started {
		t.Fatal("program was not started")
	}
}

func TestHandleFlowWrapperUsesFactoryDeps(t *testing.T) {
	resetUIStateForTest()
	setupUIConfig(t)
	origFactory := uiDepsFactory
	defer func() { uiDepsFactory = origFactory }()

	uiDepsFactory = func() uiDeps {
		return uiDeps{
			currentUser:           func() (*user.User, error) { return &user.User{Uid: "1000"}, nil },
			validateSuperuserFile: func(string, *user.User) string { return "" },
			newProgram:            func(tea.Model) programRunner { return &fakeProgram{} },
			login:                 func(config.Organization, bool, bool, *bus.Bus) (flowAPI, error) { return fakeFlow{}, nil },
			configFileSSOEmpty:    func(string, string) bool { return false },
			println:               func(...interface{}) (int, error) { return 0, nil },
			sleep:                 func(time.Duration) {},
		}
	}

	(&UI{Org: config.Organization{Name: "dev"}}).handleFlow()
	if !hasFinished {
		t.Fatal("hasFinished = false, want true")
	}
}

func setupUIConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	if err := config.ResetAndSetTestConfig(home); err != nil {
		t.Fatalf("ResetAndSetTestConfig() error = %v", err)
	}
	errorColor = color("#fa0718")
	informationColor = color("#05fa5f")
	warningColor = color("#f29830")
	focusColor = color("#4287f5")
	spinnerColor = color("#42f551")
}

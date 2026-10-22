package main

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/artipop/pourquoipas/internal/acp"
	"github.com/artipop/pourquoipas/internal/boardadapter"
)

// What the install decides rather than a person, kept in system_settings: the
// agents run on this machine, so a desktop attached to a server must not
// switch off somebody else's. On a server these are the server's.
// A person's UI settings are preferences (userSettings.ts).
const (
	settingAIEnabled          = "aiEnabled"
	settingAgentNotifications = "agentNotifications"
	settingPrefillCardFolder  = "prefillCardFolder"

	// The language the page last showed. Not a setting: what the next launch
	// installs the templates in, or they go into English and straight back.
)

// MachineSettings: every switch is on unless it has been turned off.
type MachineSettings struct {
	AIEnabled          bool `json:"aiEnabled"`
	AgentNotifications bool `json:"agentNotifications"`
	PrefillCardFolder  bool `json:"prefillCardFolder"`
}

type settingsStore interface {
	GetSystemSettings() (map[string]string, error)
	SetSystemSetting(key, value string) error
}

// In memory, because the tray reads the switches on every wait.
type machineSettings struct {
	mu     sync.Mutex
	store  settingsStore
	values map[string]string
}

// Until main hands it the store it answers the defaults.
var machine = &machineSettings{}

func (m *machineSettings) use(store settingsStore) error {
	values, err := store.GetSystemSettings()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
	m.values = values
	return nil
}

func (m *machineSettings) get(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.values[key]
}

func (m *machineSettings) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.values[key]
	return ok
}

func (m *machineSettings) set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.store != nil {
		if err := m.store.SetSystemSetting(key, value); err != nil {
			return err
		}
	}
	if m.values == nil {
		m.values = map[string]string{}
	}
	m.values[key] = value
	return nil
}

func (m *machineSettings) on(key string) bool { return m.get(key) != "false" }

// aiEnabled is the app-wide AI switch. Off, the agent manager starts nothing by
// itself (acp.Manager.SetSwitchedOff) and the OS is told about no wait.
func aiEnabled() bool { return machine.on(settingAIEnabled) }

// agentNotificationsEnabled is the one switch for "tell me when an agent
// waits": the page's notification and the OS one (alerts.go) answer the same
// question. The dot in the menu bar does not consult it — it interrupts nobody.
func agentNotificationsEnabled() bool { return machine.on(settingAgentNotifications) }

// GetMachineSettings is what the page reads before its first render, beside
// the person's own settings.
func (a *App) GetMachineSettings() MachineSettings {
	return MachineSettings{
		AIEnabled:          machine.on(settingAIEnabled),
		AgentNotifications: machine.on(settingAgentNotifications),
		PrefillCardFolder:  machine.on(settingPrefillCardFolder),
	}
}

var errNotTheOwner = errors.New("machine settings: only whoever runs this install may change them")

// SetMachineSetting turns one switch. Whoever runs the install may: on a
// desktop that is the one person there, on a server an administrator
// (ownsTheInstall).
func (a *App) SetMachineSetting(ctx context.Context, key string, on bool) error {
	switch key {
	case settingAIEnabled, settingAgentNotifications, settingPrefillCardFolder:
	default:
		return errors.New("machine settings: unknown setting " + strings.TrimSpace(key))
	}
	if !ownsTheInstall(ctx, a.owners) {
		return errNotTheOwner
	}
	was := machine.on(key)
	value := "true"
	if !on {
		value = "false"
	}
	if err := machine.set(key, value); err != nil {
		return err
	}
	// The tray's dot and the OS notifications re-read the waits on this event
	// (alerts_desktop.go); without it the dot of a switched-off AI stays lit
	// until the next wait comes or goes.
	if key == settingAIEnabled && was != on && a.emitter != nil {
		go a.emitter.Emit(acp.EventAttention, nil)
	}
	return nil
}

// TemplateWords is what the page translates the install's own templates and
// the inbox with on display (internal/boardadapter/templatetext.go): they are
// stored in English, and each person reads them in their own language.
func (a *App) TemplateWords(lang string) boardadapter.TemplateCatalogue {
	return boardadapter.TemplateCatalogueFor(lang)
}

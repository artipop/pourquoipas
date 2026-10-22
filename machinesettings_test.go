package main

import (
	"context"
	"testing"

	"github.com/artipop/pourquoipas/server/model"
)

type settingsTable map[string]string

func (s settingsTable) GetSystemSettings() (map[string]string, error) {
	out := map[string]string{}
	for k, v := range s {
		out[k] = v
	}
	return out, nil
}

func (s settingsTable) SetSystemSetting(key, value string) error {
	s[key] = value
	return nil
}

// anOwner runs the install in either build.
type anOwner struct{}

func (anOwner) GetSession(string) (*model.Session, error) { return &model.Session{UserID: "owner"}, nil }

func (anOwner) HasPermissionTo(string, *model.Permission) bool { return true }

// freshMachine gives the test its own settings table in place of the process's.
func freshMachine(t *testing.T, table settingsTable) {
	t.Helper()
	was := machine
	machine = &machineSettings{}
	if err := machine.use(table); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { machine = was })
}

// An install that has always run agents keeps running them until somebody
// turns them off, and a switch turned off is still off after a restart.
func TestTheMachinesSwitchesAreOnUntilTurnedOffAndStayOff(t *testing.T) {
	table := settingsTable{}
	freshMachine(t, table)
	app := &App{owners: anOwner{}}

	if got := app.GetMachineSettings(); !got.AIEnabled || !got.AgentNotifications || !got.PrefillCardFolder {
		t.Fatalf("a fresh install answered %+v", got)
	}
	if err := app.SetMachineSetting(withSessionToken(context.Background(), "owner"), settingAIEnabled, false); err != nil {
		t.Fatal(err)
	}
	if aiEnabled() {
		t.Error("the AI is still on")
	}

	freshMachine(t, table)
	if aiEnabled() || !agentNotificationsEnabled() {
		t.Errorf("after a restart: ai %v, notifications %v", aiEnabled(), agentNotificationsEnabled())
	}
}

// The page can name only the switches there are: the binding is callable by
// anything the page runs.
func TestAMachineSettingThatDoesNotExistIsRefused(t *testing.T) {
	table := settingsTable{}
	freshMachine(t, table)
	if err := (&App{}).SetMachineSetting(context.Background(), "TelemetryID", false); err == nil {
		t.Error("an arbitrary system setting was written")
	}
	if len(table) != 0 {
		t.Errorf("the table got %v", table)
	}
}

// The templates follow the language the page shows, and the same answer again
// — which every launch gives — reinstalls nothing.
func TestTheTemplatesFollowTheLanguageThePageShows(t *testing.T) {
	freshMachine(t, settingsTable{})
	var installed []string
	app := &App{templatesLanguage: "en", relanguage: func(lang string) error {
		installed = append(installed, lang)
		return nil
	}}

	if err := app.UseLanguage("en"); err != nil {
		t.Fatal(err)
	}
	if len(installed) != 0 {
		t.Fatalf("the language they were already in reinstalled them: %v", installed)
	}
	if err := app.UseLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if err := app.UseLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if len(installed) != 1 || installed[0] != "ru" {
		t.Errorf("installed %v, want once in ru", installed)
	}
}

// A launch installs the templates before the page has said anything, so it
// has to guess what the page is about to say: the owner's pick, else what the
// page showed last time. Guessing English put them into English and straight
// back at every launch.
func TestALaunchInstallsTheTemplatesInTheLanguageThePageWillAskFor(t *testing.T) {
	freshMachine(t, settingsTable{})
	if err := (&App{}).UseLanguage("ru"); err != nil {
		t.Fatal(err)
	}
	if got := startupLanguage(""); got != "ru" {
		t.Errorf("with nothing picked a launch installs %q", got)
	}
	if got := startupLanguage("de"); got != "de" {
		t.Errorf("with de picked a launch installs %q", got)
	}
}

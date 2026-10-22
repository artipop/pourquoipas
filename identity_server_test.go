//go:build server

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artipop/pourquoipas/server/model"
)

type ownersOf map[string]string // token -> user id; "admin" administers

func (o ownersOf) GetSession(token string) (*model.Session, error) {
	if id, ok := o[token]; ok {
		return &model.Session{UserID: id}, nil
	}
	return nil, errors.New("no session")
}

func (o ownersOf) HasPermissionTo(userID string, p *model.Permission) bool {
	return userID == "admin" && p == model.PermissionManageSystem
}

// On a server the machine's switches are the server's: a teammate who is not
// its administrator must not turn its AI off for everybody.
func TestOnAServerOnlyAnAdministratorTurnsTheMachinesSwitches(t *testing.T) {
	freshMachine(t, settingsTable{})
	app := &App{owners: ownersOf{"t-admin": "admin", "t-member": "member"}}

	// The binding is reached through the guard, which hands it the session.
	call := func(token string) error {
		var err error
		handler := requireSession(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			err = app.SetMachineSetting(r.Context(), settingAIEnabled, false)
		}), func(string) bool { return true })
		req := httptest.NewRequest(http.MethodPost, "/wails/runtime", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		handler.ServeHTTP(httptest.NewRecorder(), req)
		return err
	}

	if err := call("t-member"); err == nil || !aiEnabled() {
		t.Errorf("a member switched the AI: %v", err)
	}
	if err := (&App{owners: app.owners}).SetMachineSetting(context.Background(), settingAIEnabled, false); err == nil {
		t.Error("a call with no session switched the AI")
	}
	if err := call("t-admin"); err != nil || aiEnabled() {
		t.Errorf("the administrator could not: %v", err)
	}
}

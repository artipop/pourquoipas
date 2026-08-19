package model

import (
	"encoding/json"
	"io"
)

const (
	SingleUser                    = "single-user"
	GlobalTeamID                  = "0"
	SystemUserID                  = "system"
	PreferencesCategoryFocalboard = "focalboard"

	// SingleUserName is the username the API synthesizes for the single-user
	// session. The id above stays the identity everything checks; this is the
	// display half — the board prints the username wherever it names an
	// author, an assignee or a group header, and "single-user" was the one
	// name on those screens no person chose.
	//
	// The page translates it (webapp/src/userDisplay.ts): a constant here
	// cannot be in the language somebody is reading in, and this session is
	// synthesized per request rather than stored, so there is nothing to
	// localize at the source. What is left here is the fallback for a client
	// that does not.
	SingleUserName = "Вы"
)

// User is a user
// swagger:model
type User struct {
	// The user ID
	// required: true
	ID string `json:"id"`

	// The user name
	// required: true
	Username string `json:"username"`

	// The user's email
	// required: true
	Email string `json:"-"`

	// The user's nickname
	Nickname string `json:"nickname"`
	// The user's first name
	FirstName string `json:"firstname"`
	// The user's last name
	LastName string `json:"lastname"`

	// swagger:ignore
	Password string `json:"-"`

	// swagger:ignore
	MfaSecret string `json:"-"`

	// swagger:ignore
	AuthService string `json:"-"`

	// swagger:ignore
	AuthData string `json:"-"`

	// Created time in miliseconds since the current epoch
	// required: true
	CreateAt int64 `json:"create_at,omitempty"`

	// Updated time in miliseconds since the current epoch
	// required: true
	UpdateAt int64 `json:"update_at,omitempty"`

	// Deleted time in miliseconds since the current epoch, set to indicate user is deleted
	// required: true
	DeleteAt int64 `json:"delete_at"`

	// If the user is a bot or not
	// required: true
	IsBot bool `json:"is_bot"`

	// If the user is a guest or not
	// required: true
	IsGuest bool `json:"is_guest"`

	// Special Permissions the user may have
	Permissions []string `json:"permissions,omitempty"`

	// Role is who administers this install: RoleAdmin, or empty and RoleMember
	// reading the same. Install-wide, and a different question from what a
	// person may do on one board — that is board_members.minimum_role.
	//
	// It replaced a dead `Roles string`, the fork's Mattermost-shaped
	// space-separated list of role *names*: a free-text set that decides
	// something can carry no constraint, and this one can.
	Role string `json:"role,omitempty"`

	// Kind tells a person from the accounts this app makes under their own
	// names — one per agent, one per source. There was no column for it, so
	// nothing could ask.
	Kind string `json:"kind,omitempty"`
}

// The two roles an install has. A third would be a schema change, which is the
// point of the CHECK behind them.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// What an account is. Open on purpose — a delegated runner will want one of its
// own — so the column carries no CHECK.
const (
	UserKindPerson = "person"
	UserKindAgent  = "agent"
	UserKindSource = "source"
)

// UserPreferencesPatch is a user property patch
// swagger:model
type UserPreferencesPatch struct {
	// The user preference updated fields
	// required: false
	UpdatedFields map[string]string `json:"updatedFields"`

	// The user preference removed fields
	// required: false
	DeletedFields []string `json:"deletedFields"`
}

type Session struct {
	ID          string                 `json:"id"`
	Token       string                 `json:"token"`
	UserID      string                 `json:"user_id"`
	AuthService string                 `json:"authService"`
	Props       map[string]interface{} `json:"props"`
	CreateAt    int64                  `json:"create_at,omitempty"`
	UpdateAt    int64                  `json:"update_at,omitempty"`
}

func UserFromJSON(data io.Reader) (*User, error) {
	var user User
	if err := json.NewDecoder(data).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *User) Sanitize(options map[string]bool) {
	u.Password = ""
	u.MfaSecret = ""

	if len(options) != 0 && !options["email"] {
		u.Email = ""
	}
	if len(options) != 0 && !options["fullname"] {
		u.FirstName = ""
		u.LastName = ""
	}
}

// Preference is one thing a person chose, kept per (user, category, name) —
// the `preferences` table, and mattermost's own shape for it: this is what the
// board's REST API answers with and what the page has always read.
//
// Ours rather than mattermost's because the type is four strings and the
// package it came from is a server this application does not have. The JSON
// names are unchanged, so nothing on the page notices.
type Preference struct {
	UserId   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

// Preferences is what a person chose, in no particular order.
type Preferences []Preference

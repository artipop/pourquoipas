package app

import (
	"github.com/artipop/pourquoipas/server/model"
	"github.com/artipop/pourquoipas/server/services/auth"
	"github.com/artipop/pourquoipas/server/utils"

	"github.com/artipop/pourquoipas/server/mlog"

	"github.com/pkg/errors"
)

const (
	DaysPerMonth     = 30
	DaysPerWeek      = 7
	HoursPerDay      = 24
	MinutesPerHour   = 60
	SecondsPerMinute = 60
)

// GetSession Get a user active session and refresh the session if is needed.
func (a *App) GetSession(token string) (*model.Session, error) {
	return a.auth.GetSession(token)
}

// IsValidReadToken validates the read token for a block.
func (a *App) IsValidReadToken(boardID string, readToken string) (bool, error) {
	return a.auth.IsValidReadToken(boardID, readToken)
}

// GetRegisteredUserCount returns the number of registered users.
func (a *App) GetRegisteredUserCount() (int, error) {
	return a.store.GetRegisteredUserCount()
}

// GetAdminCount returns how many accounts administer this install. It is what
// "has anybody claimed this server yet" asks — the registered-user count cannot
// answer it, because this app makes an account per agent and per source.
func (a *App) GetAdminCount() (int, error) {
	return a.store.GetAdminCount()
}

// SetUserRole makes somebody an admin, or stops them being one.
func (a *App) SetUserRole(userID, role string) error {
	return a.store.SetUserRole(userID, role)
}

// SetUserActive turns an account off and on again. Turning it off also drops
// its sessions: every user read filters on delete_at, so login stops working by
// itself, but a session already open would go on working for three days.
func (a *App) SetUserActive(userID string, active bool) error {
	if err := a.store.SetUserActive(userID, active); err != nil {
		return err
	}
	if !active {
		return a.store.DeleteSessionsForUser(userID)
	}
	return nil
}

// GetUsersForAdmin lists every account, deactivated ones included.
func (a *App) GetUsersForAdmin() ([]*model.User, error) {
	return a.store.GetUsersForAdmin()
}

// GetDailyActiveUsers returns the number of daily active users.
func (a *App) GetDailyActiveUsers() (int, error) {
	secondsAgo := int64(SecondsPerMinute * MinutesPerHour * HoursPerDay)
	return a.store.GetActiveUserCount(secondsAgo)
}

// GetWeeklyActiveUsers returns the number of weekly active users.
func (a *App) GetWeeklyActiveUsers() (int, error) {
	secondsAgo := int64(SecondsPerMinute * MinutesPerHour * HoursPerDay * DaysPerWeek)
	return a.store.GetActiveUserCount(secondsAgo)
}

// GetMonthlyActiveUsers returns the number of monthly active users.
func (a *App) GetMonthlyActiveUsers() (int, error) {
	secondsAgo := int64(SecondsPerMinute * MinutesPerHour * HoursPerDay * DaysPerMonth)
	return a.store.GetActiveUserCount(secondsAgo)
}

// GetUser gets an existing active user by id.
func (a *App) GetUser(id string) (*model.User, error) {
	if len(id) < 1 {
		return nil, errors.New("no user ID")
	}

	user, err := a.store.GetUserByID(id)
	if err != nil {
		return nil, errors.Wrap(err, "unable to find user")
	}
	return user, nil
}

// GetUsersList returns the users with the given ids. When some ids have no
// user, the ones that do are returned together with a not-found error, so the
// caller can decide whether a partial answer is good enough.
func (a *App) GetUsersList(userIDs []string) ([]*model.User, error) {
	if len(userIDs) == 0 {
		return nil, errors.New("No User IDs")
	}

	users, err := a.store.GetUsersList(userIDs, a.config.ShowEmailAddress, a.config.ShowFullName)
	if err != nil {
		if !model.IsErrNotFound(err) {
			return nil, errors.Wrap(err, "unable to find users")
		}
		return users, err
	}
	return users, nil
}

// GetUserByUsername returns the user with the given username, or nil when there
// is none. Used to look an account up before creating it.
func (a *App) GetUserByUsername(username string) (*model.User, error) {
	if username == "" {
		return nil, errors.New("no username")
	}

	user, err := a.store.GetUserByUsername(username)
	if err != nil {
		if model.IsErrNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "unable to find user by username")
	}
	return user, nil
}

// Login create a new user session if the authentication data is valid.
func (a *App) Login(username, email, password, mfaToken string) (string, error) {
	var user *model.User
	if username != "" {
		var err error
		user, err = a.store.GetUserByUsername(username)
		if err != nil && !model.IsErrNotFound(err) {
			return "", errors.Wrap(err, "invalid username or password")
		}
	}

	if user == nil && email != "" {
		var err error
		user, err = a.store.GetUserByEmail(email)
		if err != nil && model.IsErrNotFound(err) {
			return "", errors.Wrap(err, "invalid username or password")
		}
	}

	if user == nil {
		return "", errors.New("invalid username or password")
	}

	if !auth.ComparePassword(user.Password, password) {
		a.logger.Debug("Invalid password for user", mlog.String("userID", user.ID))
		return "", errors.New("invalid username or password")
	}

	authService := user.AuthService
	if authService == "" {
		authService = "native"
	}

	session := model.Session{
		ID:          utils.NewID(utils.IDTypeSession),
		Token:       utils.NewID(utils.IDTypeToken),
		UserID:      user.ID,
		AuthService: authService,
		Props:       map[string]interface{}{},
	}
	err := a.store.CreateSession(&session)
	if err != nil {
		return "", errors.Wrap(err, "unable to create session")
	}

	// TODO: MFA verification
	return session.Token, nil
}

// Logout invalidates the user session.
func (a *App) Logout(sessionID string) error {
	err := a.store.DeleteSession(sessionID)
	if err != nil {
		return errors.Wrap(err, "unable to delete the session")
	}

	return nil
}

// RegisterUser registers a person: somebody who logs in. The first one to exist
// administers the install.
func (a *App) RegisterUser(username, email, password string) error {
	return a.RegisterUserWithID(utils.NewID(utils.IDTypeUser), username, email, password)
}

// RegisterUserWithID is the same registration under an id the caller names, for
// a caller that has to make an account and an identity agree.
func (a *App) RegisterUserWithID(id, username, email, password string) error {
	return a.registerUser(id, username, email, password, model.UserKindPerson)
}

// RegisterMachineUser makes an account for something that never logs in — an
// agent, a source. It is a separate door because the rule above must not apply
// to it: an install where the first agent to be provisioned becomes the admin
// is an install a person cannot claim, and on a hosted board a desktop attaches
// and provisions agents before anybody has registered.
func (a *App) RegisterMachineUser(username, email, password, kind string) error {
	if kind == model.UserKindPerson {
		return errors.New("a machine account cannot be a person")
	}
	return a.registerUser(utils.NewID(utils.IDTypeUser), username, email, password, kind)
}

func (a *App) registerUser(id, username, email, password, kind string) error {
	var user *model.User
	if username != "" {
		var err error
		user, err = a.store.GetUserByUsername(username)
		if err != nil && !model.IsErrNotFound(err) {
			return err
		}
		if user != nil {
			return errors.New("The username already exists")
		}
	}

	if user == nil && email != "" {
		var err error
		user, err = a.store.GetUserByEmail(email)
		if err != nil && !model.IsErrNotFound(err) {
			return err
		}
		if user != nil {
			return errors.New("The email already exists")
		}
	}

	// TODO: Move this into the config
	passwordSettings := auth.PasswordSettings{
		MinimumLength: 6,
	}

	err := auth.IsPasswordValid(password, passwordSettings)
	if err != nil {
		return errors.Wrap(err, "Invalid password")
	}

	// The first person to exist administers the install — the shape every
	// self-hosted board has, and the only one that needs no setup step outside
	// the product. Counted in admins rather than in users, because this app
	// makes an account per agent and per source: "somebody has registered" and
	// "somebody has claimed this server" are not the same question.
	role := model.RoleMember
	if kind == model.UserKindPerson {
		adminCount, err2 := a.store.GetAdminCount()
		if err2 != nil {
			return errors.Wrap(err2, "Unable to tell whether this install has an admin")
		}
		if adminCount == 0 {
			role = model.RoleAdmin
		}
	}

	_, err = a.store.CreateUser(&model.User{
		ID:          id,
		Username:    username,
		Email:       email,
		Password:    auth.HashPassword(password),
		MfaSecret:   "",
		AuthService: a.config.AuthMode,
		AuthData:    "",
		Role:        role,
		Kind:        kind,
	})
	if err != nil {
		return errors.Wrap(err, "Unable to create the new user")
	}

	return nil
}

func (a *App) UpdateUserPassword(username, password string) error {
	err := a.store.UpdateUserPassword(username, auth.HashPassword(password))
	if err != nil {
		return err
	}

	return nil
}

func (a *App) ChangePassword(userID, oldPassword, newPassword string) error {
	var user *model.User
	if userID != "" {
		var err error
		user, err = a.store.GetUserByID(userID)
		if err != nil {
			return errors.Wrap(err, "invalid username or password")
		}
	}

	if user == nil {
		return errors.New("invalid username or password")
	}

	if !auth.ComparePassword(user.Password, oldPassword) {
		a.logger.Debug("Invalid password for user", mlog.String("userID", user.ID))
		return errors.New("invalid username or password")
	}

	err := a.store.UpdateUserPasswordByID(userID, auth.HashPassword(newPassword))
	if err != nil {
		return errors.Wrap(err, "unable to update password")
	}

	return nil
}

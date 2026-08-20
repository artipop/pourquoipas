package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/web"
)

// An account for something that never logs in: one per registered agent, one
// per source. A card names its agent by being assigned to it, so the name needs
// a row in the users table — and until a desktop could attach to a hosted board,
// the only way to make one was to be the process holding the database.
//
// Any member may call it: registering an agent is what a person does in order to
// work here, and the account has no password anybody knows and no authority.
// What it may not make is a person, which is the door /register owns.

type machineUserRequest struct {
	Username string `json:"username"`
	Kind     string `json:"kind"`
}

func (a *API) registerMachineUsersRoutes(r *web.Router) {
	r.HandleFunc("POST /teams/{teamID}/machine-users", a.sessionRequired(a.handleEnsureMachineUser))
}

func (a *API) handleEnsureMachineUser(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /teams/{teamID}/machine-users ensureMachineUser
	//
	// Finds or creates the account an agent or a source is named by
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: teamID
	//   in: path
	//   description: Team ID
	//   required: true
	//   type: string
	// security:
	// - BearerAuth: []
	// responses:
	//   '200':
	//     description: success
	//   '400':
	//     description: no username, or a kind this route does not make
	//   '403':
	//     description: not a member of the team
	//   default:
	//     description: internal error
	//     schema:
	//       "$ref": "#/definitions/ErrorResponse"
	teamID := r.PathValue("teamID")
	if !a.permissions.HasPermissionToTeam(getUserID(r), teamID, model.PermissionViewTeam) {
		a.errorResponse(w, r, model.NewErrPermission("access denied to team"))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	var want machineUserRequest
	if err = json.Unmarshal(body, &want); err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}
	if want.Username == "" {
		a.errorResponse(w, r, model.NewErrBadRequest("username is required"))
		return
	}
	if want.Kind != model.UserKindAgent && want.Kind != model.UserKindSource {
		a.errorResponse(w, r, model.NewErrBadRequest("kind must be agent or source"))
		return
	}

	// Idempotent by username: several machines register the same agent and the
	// board wants one account for the name.
	user, err := a.app.GetUserByUsername(want.Username)
	if err != nil && !model.IsErrNotFound(err) {
		a.errorResponse(w, r, err)
		return
	}
	if user == nil {
		// A password nobody knows: the account exists to be named.
		buf := make([]byte, machineUserPasswordBytes)
		if _, pErr := rand.Read(buf); pErr != nil {
			a.errorResponse(w, r, pErr)
			return
		}
		password := hex.EncodeToString(buf)
		if err = a.app.RegisterMachineUser(want.Username, want.Username+"@"+machineUserDomain, password, want.Kind); err != nil {
			a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
			return
		}
		if user, err = a.app.GetUserByUsername(want.Username); err != nil || user == nil {
			a.errorResponse(w, r, model.NewErrNotFound("the account was created and cannot be read back"))
			return
		}
	}

	data, err := json.Marshal(map[string]string{"userId": user.ID, "username": user.Username})
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	jsonBytesResponse(w, http.StatusOK, data)
}

const (
	// A reserved TLD (RFC 2606), so an address nobody reads is never one
	// somebody else owns.
	machineUserDomain        = "agents.invalid"
	machineUserPasswordBytes = 24
)

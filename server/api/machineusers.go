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
// per source. The board names an agent by a card being assigned to it, so the
// name needs a row in the users table, and until now the only way to make one
// was to be the process holding the database.
//
// A desktop attached to a hosted board is not that process, and this is the one
// thing in the whole seam that REST could not already do — /register wants the
// signup token and cannot say what kind of account it is making.
//
// Any member may call it. Registering an agent is what a person does in order
// to work here, and the account it makes has no password anybody knows and no
// authority: role=member, kind=agent. What it is *not* allowed to make is a
// person, which is the door /register owns.

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

	// Idempotent by username: several machines register the same agent, and the
	// board wants one account for the name. That they are three different
	// installations behind it is the machine's business and not the board's.
	user, err := a.app.GetUserByUsername(want.Username)
	if err != nil && !model.IsErrNotFound(err) {
		a.errorResponse(w, r, err)
		return
	}
	if user == nil {
		// A password nobody knows: the account exists to be named, and there is
		// no door it opens.
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
	// A reserved TLD, so an address nobody reads can never be an address
	// somebody else owns (RFC 2606).
	machineUserDomain = "agents.invalid"
	// Long enough that the server's own password rules pass and short enough to
	// be one line in a log nobody should ever see it in.
	machineUserPasswordBytes = 24
)

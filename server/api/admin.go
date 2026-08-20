package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/web"
)

// Administering the install, as opposed to administering a board: board roles
// are membership and answer "what may this person do here", these answer "who
// runs this server".
//
// Nobody is an admin on a desktop, so every one of these answers 403 there —
// right rather than a gap, since an install of one person has nobody to
// administer against.

func (a *API) registerAdminRoutes(r *web.Router) {
	r.HandleFunc("GET /admin/users", a.adminRequired(a.handleAdminGetUsers))
	r.HandleFunc("PUT /admin/users/{userID}/role", a.adminRequired(a.handleAdminSetRole))
	r.HandleFunc("POST /admin/users/{userID}/deactivate", a.adminRequired(a.handleAdminDeactivate))
	r.HandleFunc("POST /admin/users/{userID}/activate", a.adminRequired(a.handleAdminActivate))
}

// adminRequired is sessionRequired plus the one permission this file is about.
func (a *API) adminRequired(handler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return a.sessionRequired(func(w http.ResponseWriter, r *http.Request) {
		if !a.permissions.HasPermissionTo(getUserID(r), model.PermissionManageSystem) {
			a.errorResponse(w, r, model.NewErrPermission("access denied to the install"))
			return
		}
		handler(w, r)
	})
}

func (a *API) handleAdminGetUsers(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /admin/users adminGetUsers
	//
	// Returns every account, deactivated ones included
	//
	// ---
	// produces:
	// - application/json
	// security:
	// - BearerAuth: []
	// responses:
	//   '200':
	//     description: success
	//   '403':
	//     description: not an admin
	//   default:
	//     description: internal error
	//     schema:
	//       "$ref": "#/definitions/ErrorResponse"
	users, err := a.app.GetUsersForAdmin()
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}

	// Addresses are shown here and nowhere else: it is how you tell two people
	// apart.
	data, err := json.Marshal(users)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	jsonBytesResponse(w, http.StatusOK, data)
}

func (a *API) handleAdminSetRole(w http.ResponseWriter, r *http.Request) {
	// swagger:operation PUT /admin/users/{userID}/role adminSetRole
	//
	// Makes an account an admin of this install, or stops it being one
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: userID
	//   in: path
	//   description: User ID
	//   required: true
	//   type: string
	// security:
	// - BearerAuth: []
	// responses:
	//   '200':
	//     description: success
	//   '400':
	//     description: the last admin, or an unknown role
	//   '403':
	//     description: not an admin
	//   default:
	//     description: internal error
	//     schema:
	//       "$ref": "#/definitions/ErrorResponse"
	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.errorResponse(w, r, err)
		return
	}
	var want struct {
		Role string `json:"role"`
	}
	if err = json.Unmarshal(body, &want); err != nil {
		a.errorResponse(w, r, model.NewErrBadRequest(err.Error()))
		return
	}
	if want.Role != model.RoleAdmin && want.Role != model.RoleMember {
		a.errorResponse(w, r, model.NewErrBadRequest("role must be admin or member"))
		return
	}

	userID := r.PathValue("userID")
	if want.Role == model.RoleMember {
		if err = a.refuseLosingTheLastAdmin(userID); err != nil {
			a.errorResponse(w, r, err)
			return
		}
	}

	if err = a.app.SetUserRole(userID, want.Role); err != nil {
		a.errorResponse(w, r, err)
		return
	}
	jsonStringResponse(w, http.StatusOK, "{}")
}

func (a *API) handleAdminDeactivate(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /admin/users/{userID}/deactivate adminDeactivate
	//
	// Turns an account off: it can no longer log in, and its open sessions end
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: userID
	//   in: path
	//   description: User ID
	//   required: true
	//   type: string
	// security:
	// - BearerAuth: []
	// responses:
	//   '200':
	//     description: success
	//   '400':
	//     description: the last admin
	//   '403':
	//     description: not an admin
	//   default:
	//     description: internal error
	//     schema:
	//       "$ref": "#/definitions/ErrorResponse"
	userID := r.PathValue("userID")
	if err := a.refuseLosingTheLastAdmin(userID); err != nil {
		a.errorResponse(w, r, err)
		return
	}
	if err := a.app.SetUserActive(userID, false); err != nil {
		a.errorResponse(w, r, err)
		return
	}
	jsonStringResponse(w, http.StatusOK, "{}")
}

func (a *API) handleAdminActivate(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /admin/users/{userID}/activate adminActivate
	//
	// Turns an account back on
	//
	// ---
	// produces:
	// - application/json
	// parameters:
	// - name: userID
	//   in: path
	//   description: User ID
	//   required: true
	//   type: string
	// security:
	// - BearerAuth: []
	// responses:
	//   '200':
	//     description: success
	//   '403':
	//     description: not an admin
	//   default:
	//     description: internal error
	//     schema:
	//       "$ref": "#/definitions/ErrorResponse"
	if err := a.app.SetUserActive(r.PathValue("userID"), true); err != nil {
		a.errorResponse(w, r, err)
		return
	}
	jsonStringResponse(w, http.StatusOK, "{}")
}

// refuseLosingTheLastAdmin: an install nobody can administer cannot be repaired
// from inside the product, so the API refuses to make one.
func (a *API) refuseLosingTheLastAdmin(userID string) error {
	user, err := a.app.GetUser(userID)
	if err != nil {
		return err
	}
	if user == nil || user.Role != model.RoleAdmin {
		return nil
	}
	admins, err := a.app.GetAdminCount()
	if err != nil {
		return err
	}
	if admins <= 1 {
		return model.NewErrBadRequest("this is the only admin left; make somebody else an admin first")
	}
	return nil
}

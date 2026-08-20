package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/artipop/xciii/server/app"
	"github.com/artipop/xciii/server/model"
	"github.com/artipop/xciii/server/services/permissions"
	"github.com/artipop/xciii/server/web"

	"github.com/artipop/xciii/server/mlog"
)

const (
	HeaderRequestedWith    = "X-Requested-With"
	HeaderRequestedWithXML = "XMLHttpRequest"
	UploadFormFileKey      = "file"
	True                   = "true"

	ErrorNoTeamCode    = 1000
	ErrorNoTeamMessage = "No team"
)

var (
	ErrHandlerPanic = errors.New("http handler panic")
)

// ----------------------------------------------------------------------------------------------------
// REST APIs

type API struct {
	app             *app.App
	authService     string
	permissions     permissions.PermissionsService
	singleUserToken string
	logger          mlog.LoggerIFace
}

func NewAPI(
	app *app.App,
	singleUserToken string,
	authService string,
	permissions permissions.PermissionsService,
	logger mlog.LoggerIFace,
) *API {
	return &API{
		app:             app,
		singleUserToken: singleUserToken,
		authService:     authService,
		permissions:     permissions,
		logger:          logger,
	}
}

// synthesizesSingleUser reports whether the id `single-user` names a session
// this API makes up rather than an account somebody registered.
//
// It is a desktop's answer and only a desktop's (identity.go). A hosted server
// holds no such token, so nothing there is synthesized and every id is looked
// up. The guard matters because the synthesizer would otherwise answer every
// question about that id with a made-up user called the "you" name and shadow a real row,
// name, permissions and all.
func (a *API) synthesizesSingleUser() bool {
	return len(a.singleUserToken) > 0
}

// RegisterRoutes mounts the board's API.
//
// The version in the path is v0, and it says what is true: this API has never
// been released and is changed freely — routes were added to it in the same
// week this comment was written. It was v2, which is the version Focalboard
// shipped, and carrying somebody else's number is a promise about compatibility
// with a product this one is not.
//
// A v1 that nothing ever served, and a v3 that upstream left as a commented-out
// group and a "migrate these when ready" note, went with it.
func (a *API) RegisterRoutes(r *web.Router) {
	api := r.Group("/api/v0", a.panicHandler, a.requireCSRFToken)

	a.registerUsersRoutes(api)
	a.registerAuthRoutes(api)
	a.registerMembersRoutes(api)
	a.registerCategoriesRoutes(api)
	a.registerSharingRoutes(api)
	a.registerTeamsRoutes(api)
	a.registerAdminRoutes(api)
	a.registerMachineUsersRoutes(api)
	a.registerAchivesRoutes(api)
	a.registerSubscriptionsRoutes(api)
	a.registerFilesRoutes(api)
	a.registerSearchRoutes(api)
	a.registerConfigRoutes(api)
	a.registerBoardsAndBlocksRoutes(api)
	a.registerTemplatesRoutes(api)
	a.registerBoardsRoutes(api)
	a.registerBlocksRoutes(api)
	a.registerContentBlocksRoutes(api)

	a.registerCardsRoutes(api)

	// System routes are outside the /api/v0 path
	a.registerSystemRoutes(r)
}

func getUserID(r *http.Request) string {
	ctx := r.Context()
	session, ok := ctx.Value(sessionContextKey).(*model.Session)
	if !ok {
		return ""
	}
	return session.UserID
}

func (a *API) panicHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				a.logger.Error("Http handler panic",
					mlog.Any("panic", p),
					mlog.String("stack", string(debug.Stack())),
					mlog.String("uri", r.URL.Path),
				)
				a.errorResponse(w, r, ErrHandlerPanic)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (a *API) requireCSRFToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !a.checkCSRFToken(r) {
			a.logger.Error("checkCSRFToken FAILED")
			a.errorResponse(w, r, model.NewErrBadRequest("checkCSRFToken FAILED"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *API) checkCSRFToken(r *http.Request) bool {
	token := r.Header.Get(HeaderRequestedWith)
	return token == HeaderRequestedWithXML
}

func (a *API) hasValidReadTokenForBoard(r *http.Request, boardID string) bool {
	query := r.URL.Query()
	readToken := query.Get("read_token")

	if len(readToken) < 1 {
		return false
	}

	isValid, err := a.app.IsValidReadToken(boardID, readToken)
	if err != nil {
		a.logger.Error("IsValidReadTokenForBoard ERROR", mlog.Err(err))
		return false
	}

	return isValid
}

func (a *API) userIsGuest(userID string) (bool, error) {
	if a.singleUserToken != "" {
		return false, nil
	}
	return a.app.UserIsGuest(userID)
}

// Response helpers

func (a *API) errorResponse(w http.ResponseWriter, r *http.Request, err error) {
	errorResponse := model.ErrorResponse{Error: err.Error()}

	// What the caller did wrong is the caller's news, not this server's. Every
	// answer used to be logged at ERROR, so an ordinary page sitting on /login
	// — which asks for /users/me without a token — printed one error line per
	// request, and a client holding a session that no longer exists printed a
	// wall of them. The one case that is genuinely ours keeps ERROR, below.
	a.logger.Debug("API refused a request", mlog.Err(err), mlog.String("api", r.URL.Path))

	switch {
	case model.IsErrBadRequest(err):
		errorResponse.ErrorCode = http.StatusBadRequest
	case model.IsErrUnauthorized(err):
		errorResponse.ErrorCode = http.StatusUnauthorized
	case model.IsErrForbidden(err):
		errorResponse.ErrorCode = http.StatusForbidden
	case model.IsErrNotFound(err):
		errorResponse.ErrorCode = http.StatusNotFound
	case model.IsErrRequestEntityTooLarge(err):
		errorResponse.ErrorCode = http.StatusRequestEntityTooLarge
	case model.IsErrNotImplemented(err):
		errorResponse.ErrorCode = http.StatusNotImplemented
	default:
		a.logger.Error("API ERROR",
			mlog.Int("code", http.StatusInternalServerError),
			mlog.Err(err),
			mlog.String("api", r.URL.Path),
		)
		errorResponse.Error = "internal server error"
		errorResponse.ErrorCode = http.StatusInternalServerError
	}

	setResponseHeader(w, "Content-Type", "application/json")
	data, err := json.Marshal(errorResponse)
	if err != nil {
		data = []byte("{}")
	}

	w.WriteHeader(errorResponse.ErrorCode)
	_, _ = w.Write(data)
}

func stringResponse(w http.ResponseWriter, message string) {
	setResponseHeader(w, "Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, message)
}

func jsonStringResponse(w http.ResponseWriter, code int, message string) { //nolint:unparam
	setResponseHeader(w, "Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprint(w, message)
}

func jsonBytesResponse(w http.ResponseWriter, code int, json []byte) { //nolint:unparam
	setResponseHeader(w, "Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(json)
}

func setResponseHeader(w http.ResponseWriter, key string, value string) { //nolint:unparam
	header := w.Header()
	if header == nil {
		return
	}
	header.Set(key, value)
}

package model

// A capability is something this server answers that Focalboard's API did not,
// named so that a client asks for it by name. A desktop joining a hosted board
// is shipped apart from the server it joins, and the two used to find out they
// disagreed from a 404 deep in the stack — or never, where an older client just
// read a field it did not know about as absent.
//
// Names rather than a version number, because a number stops meaning anything
// the first time a feature is backported; a name means exactly the thing it
// names. /ping lists them (server_metadata.go), and the desktop refuses to join
// a server that lacks one it needs (attach.go).
const (
	// POST /teams/{teamID}/machine-users: the accounts agents and sources are
	// named by on the board.
	CapabilityMachineUsers = "machine-users"
	// POST /cards/{cardID}/move: a card carried to another board keeping its id.
	CapabilityCardMove = "card-move"
	// GET /cards/{cardID}/history: who changed what on a card.
	CapabilityCardHistory = "card-history"
	// A user's role and kind in its own representation, and PUT
	// /admin/users/{userID}/role: the People panel.
	CapabilityUserRoles = "user-roles"
)

// Capabilities is what this build of the server answers, in the order /ping
// lists them.
var Capabilities = []string{
	CapabilityMachineUsers,
	CapabilityCardMove,
	CapabilityCardHistory,
	CapabilityUserRoles,
}

// ServerInfo is the part of /ping a client reads: which application, and what
// it can do.
type ServerInfo struct {
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// Missing is the capabilities of want the server did not list.
func (s ServerInfo) Missing(want []string) []string {
	have := make(map[string]bool, len(s.Capabilities))
	for _, c := range s.Capabilities {
		have[c] = true
	}
	var out []string
	for _, c := range want {
		if !have[c] {
			out = append(out, c)
		}
	}
	return out
}

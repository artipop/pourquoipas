package model

// ClientConfig is the client configuration
// swagger:model
type ClientConfig struct {
	// Is public shared boards enabled
	// required: true
	EnablePublicSharedBoards bool `json:"enablePublicSharedBoards"`

	// Is public shared boards enabled
	// required: true
	TeammateNameDisplay string `json:"teammateNameDisplay"`

	// The server feature flags
	// required: true
	FeatureFlags map[string]string `json:"featureFlags"`

	// Required for file upload to check the size of the file
	// required: true
	MaxFileSize int64 `json:"maxFileSize"`

	// Whether this install has more than one person on it
	// required: true
	TeamMode bool `json:"teamMode"`

	// Whether anybody may still register without an invitation — true only on
	// a hosted server nobody has claimed yet. It is what lets the first person
	// through the door: without it a fresh server offers a login nobody has an
	// account for, and the way in is a URL you have to know.
	// required: true
	RegistrationOpen bool `json:"registrationOpen"`
}

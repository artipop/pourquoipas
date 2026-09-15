package config

const (
	DefaultServerRoot = "http://localhost:8000"
	DefaultPort       = 8000
	DBPingAttempts    = 5
)

type AmazonS3Config struct {
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PathPrefix      string
	Region          string
	Endpoint        string
	SSL             bool
	SignV2          bool
	SSE             bool
	Trace           bool
	Timeout         int64
}

// Configuration is the app configuration stored in a json file.
type Configuration struct {
	ServerRoot               string         `json:"serverRoot" mapstructure:"serverRoot"`
	Port                     int            `json:"port" mapstructure:"port"`
	DBType                   string         `json:"dbtype" mapstructure:"dbtype"`
	DBConfigString           string         `json:"dbconfig" mapstructure:"dbconfig"`
	DBPingAttempts           int            `json:"dbpingattempts" mapstructure:"dbpingattempts"`
	UseSSL                   bool           `json:"useSSL" mapstructure:"useSSL"`
	SecureCookie             bool           `json:"secureCookie" mapstructure:"secureCookie"`
	WebPath                  string         `json:"webpath" mapstructure:"webpath"`
	FilesDriver              string         `json:"filesdriver" mapstructure:"filesdriver"`
	FilesS3Config            AmazonS3Config `json:"filess3config" mapstructure:"filess3config"`
	FilesPath                string         `json:"filespath" mapstructure:"filespath"`
	MaxFileSize              int64          `json:"maxfilesize" mapstructure:"maxfilesize"`
	Secret                   string         `json:"secret" mapstructure:"secret"`
	SessionExpireTime        int64          `json:"session_expire_time" mapstructure:"session_expire_time"`
	SessionRefreshTime       int64          `json:"session_refresh_time" mapstructure:"session_refresh_time"`
	LocalOnly                bool           `json:"localonly" mapstructure:"localonly"`
	EnablePublicSharedBoards bool           `json:"enablePublicSharedBoards" mapstructure:"enablePublicSharedBoards"`

	// TeamMode says this install has accounts on it. Set by the application
	// rather than read from a file — it is the same answer the single-user
	// token is (identity.go) — and passed to the page in the client config,
	// which is how the page knows whether a comment has anybody to be
	// addressed to.
	TeamMode            bool              `json:"teamMode" mapstructure:"teamMode"`
	FeatureFlags        map[string]string `json:"featureFlags" mapstructure:"featureFlags"`
	TeammateNameDisplay string            `json:"teammate_name_display" mapstructure:"teammateNameDisplay"`
	ShowEmailAddress    bool              `json:"show_email_address" mapstructure:"showEmailAddress"`
	ShowFullName        bool              `json:"show_full_name" mapstructure:"showFullName"`

	AuthMode string `json:"authMode" mapstructure:"authMode"`

	LoggingCfgFile string `json:"logging_cfg_file" mapstructure:"logging_cfg_file"`
	LoggingCfgJSON string `json:"logging_cfg_json" mapstructure:"logging_cfg_json"`

	NotifyFreqCardSeconds  int `json:"notify_freq_card_seconds" mapstructure:"notify_freq_card_seconds"`
	NotifyFreqBoardSeconds int `json:"notify_freq_board_seconds" mapstructure:"notify_freq_board_seconds"`
}

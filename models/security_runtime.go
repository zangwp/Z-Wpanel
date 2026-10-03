package models

type SecurityServiceStatus struct {
	Active  bool `json:"active"`
	Enabled bool `json:"enabled"`
}

type SecurityTimerStatus struct {
	Active  bool   `json:"active"`
	Enabled bool   `json:"enabled"`
	NextRun string `json:"next_run"`
}

type SecurityRuntimeStatus struct {
	Fail2ban            SecurityServiceStatus `json:"fail2ban"`
	Nftables            SecurityServiceStatus `json:"nftables"`
	WhitelistTimer      SecurityTimerStatus   `json:"whitelist_timer"`
	ActiveBans          int                   `json:"active_bans"`
	OfficialWhitelist   int                   `json:"official_whitelist_count"`
	WebWhitelist        int                   `json:"web_whitelist_count"`
	SSHWhitelist        int                   `json:"ssh_whitelist_count"`
	LastWhitelistUpdate string                `json:"last_whitelist_update"`
	CDNEnabledGroups    int                   `json:"cdn_enabled_groups"`
	CDNProtectedSites   int                   `json:"cdn_protected_sites"`
	CDNInvalidBindings  int                   `json:"cdn_invalid_bindings"`
}

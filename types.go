package main

import "gopkg.in/ldap.v3"

func (o *GiteaOrganization) String() string {
	return o.Name
}

type GiteaOrganization struct {
	ID          int    `json:"id"`
	AvatarURL   string `json:"avatar_url"`
	Description string `json:"description"`
	FullName    string `json:"full_name"`
	Location    string `json:"location"`
	Name        string `json:"username"`
	Visibility  string `json:"visibility"`
	Website     string `json:"website"`
}

type GiteaTeam struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Permission  string `json:"permission"`
	// CanCreateOrgRepo        bool   `json:"can_create_org_repo"`
	// IncludesAllRepositories bool   `json:"includes_all_repositories"`
	// Units                   string
	// UnitsMap                string
}

type GiteaCreateTeamOpts struct {
	Permission              string
	CanCreateOrgRepo        bool
	IncludesAllRepositories bool
	Units                   string
	UnitsMap                string
}

type GiteaUser struct {
	ID        int    `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Created   string `json:"created"`
	Email     string `json:"email"`
	FullName  string `json:"full_name"`
	IsAdmin   bool   `json:"is_admin"`
	Language  string `json:"language"`
	LastLogin string `json:"last_login"`
	Login     string `json:"login"`
}

type GiteaAccount struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Login    string `json:"login"`
}

type SearchResults struct {
	Data []GiteaUser `json:"data"`
	Ok   bool        `json:"ok"`
}

type GiteaClient struct {
	Token           []string `mapstructure:"token"`
	BaseURL         string   `mapstructure:"base_url"`
	Command         string
	BruteforceToken int
	ClientTimeout   int `mapstructure:"client_timeout"`
}

// Config describes the settings of the application. This structure is used in the settings-import process.
type Config struct {
	GiteaClient GiteaClient `mapstructure:"gitea"`
	LDAP        LDAP        `mapstructure:"ldap"`
	CronTimer   string      `mapstructure:"cron_timer"`
	SyncConfig  SyncConfig  `mapstructure:"sync_config"`
}

type SyncConfig struct {
	CreateGroups bool `mapstructure:"create_groups"`
	FullSync     bool `mapstructure:"full_sync"`
	Defaults     struct {
		Organization struct {
			RepoAdminChangeTeamAccess bool   `mapstructure:"repo_admin_change_team_access"`
			Visibility                string `mapstructure:"visibility"`
		} `mapstructure:"organization"`
		Team struct {
			CanCreateOrgRepo        bool   `mapstructure:"can_create_org_repo"`
			IncludesAllRepositories bool   `mapstructure:"includes_all_repositories"`
			Permission              string `mapstructure:"permission"`
			Units                   string `mapstructure:"units"`
			UnitsMap                string `mapstructure:"units_map"`
		} `mapstructure:"team"`
	} `mapstructure:"defaults"`
}

type LDAP struct {
	URL                   string   `mapstructure:"url"`
	Port                  int      `mapstructure:"port"`
	UseTLS                bool     `mapstructure:"use_tls"`
	AllowInsecureTLS      bool     `mapstructure:"allow_insecure_tls"`
	BindDN                string   `mapstructure:"bind_dn"`
	BindPassword          string   `mapstructure:"bind_password"`
	UserFilter            string   `mapstructure:"user_filter"`
	UserSearchBase        string   `mapstructure:"user_search_base"`
	UserIdentityAttribute string   `mapstructure:"user_identity_attribute"`
	UserFullName          string   `mapstructure:"user_fullname"`
	GroupSearchBase       string   `mapstructure:"group_search_base"`
	GroupFilter           string   `mapstructure:"group_filter"`
	SubGroupSearchBase    string   `mapstructure:"subgroup_search_base"`
	SubGroupFilter        string   `mapstructure:"subgroup_filter"`
	ExcludeGroups         []string `mapstructure:"exclude_groups"`
	ExcludeGroupsRegex    string   `mapstructure:"exclude_groups_regex"`
	ExcludeSubgroups      []string `mapstructure:"exclude_subgroups"`
	ExcludeSubgroupsRegex string   `mapstructure:"exclude_subgroups_regex"`
	ldap.Client
}

type Directory struct {
	Organizations map[string]*LDAPOrganization
}

type LDAPOrganization struct {
	Name string
	*ldap.Entry
	Teams map[string]*LDAPTeam
}

type LDAPTeam struct {
	Name string
	*ldap.Entry
	Users map[string]*LDAPUser
}

type LDAPUser struct {
	Name string
	*ldap.Entry
}

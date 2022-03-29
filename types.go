package main

import (
	"code.gitea.io/sdk/gitea"
	"gopkg.in/ldap.v3"
)

func (o *GiteaOrganization) String() string {
	return o.UserName
}

type GiteaOrganization struct {
	*gitea.Organization

	RepoAdminChangeTeamAccess bool `json:"repoAdminChangeTeamAccess"`
}

type GiteaTeam struct {
	*gitea.Team
}

type GiteaCreateTeamOpts struct {
	Permission              gitea.AccessMode
	CanCreateOrgRepo        bool
	IncludesAllRepositories bool
	Units                   []gitea.RepoUnitType
}

type GiteaUser struct {
	*gitea.User
}

type GiteaAccount struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Login    string `json:"login"`
	Email    string `json:"email"`
}

type GiteaClient struct {
	*gitea.Client
	Token   []string `mapstructure:"token"`
	BaseURL string   `mapstructure:"base_url"`
}

// Config describes the settings of the application. This structure is used in the settings-import process.
type Config struct {
	GiteaClient *GiteaClient `mapstructure:"gitea"`
	LDAP        LDAP         `mapstructure:"ldap"`
	CronTimer   string       `mapstructure:"cron_timer"`
	SyncConfig  SyncConfig   `mapstructure:"sync_config"`
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
			CanCreateOrgRepo        bool                 `mapstructure:"can_create_org_repo"`
			IncludesAllRepositories bool                 `mapstructure:"includes_all_repositories"`
			Permission              gitea.AccessMode     `mapstructure:"permission"`
			Units                   []gitea.RepoUnitType `mapstructure:"units"`
		} `mapstructure:"team"`
	} `mapstructure:"defaults"`
}

type LDAP struct {
	URL              string `mapstructure:"url"`
	Port             int    `mapstructure:"port"`
	UseTLS           bool   `mapstructure:"use_tls"`
	AllowInsecureTLS bool   `mapstructure:"allow_insecure_tls"`
	BindDN           string `mapstructure:"bind_dn"`
	BindPassword     string `mapstructure:"bind_password"`
	UserFilter       string `mapstructure:"user_filter"`
	UserSearchBase   string `mapstructure:"user_search_base"`

	UserUsernameAttribute     string `mapstructure:"user_username_attribute"`
	UserFullNameAttribute     string `mapstructure:"user_fullname_attribute"`
	UserFirstNameAttribute    string `mapstructure:"user_first_name_attribute"`
	UserSurnameAttribute      string `mapstructure:"user_surname_attribute"`
	UserEmailAttribute        string `mapstructure:"user_email_attribute"`
	UserPublicSSHKeyAttribute string `mapstructure:"user_public_ssh_key_attribute"`
	UserAvatarAttribute       string `mapstructure:"user_avatar_attribute"`

	ExcludeUsers      []string `mapstructure:"exclude_users"`
	ExcludeUsersRegex string   `mapstructure:"exclude_users_regex"`

	AdminFilter      string `mapstructure:"admin_filter"`
	RestrictedFilter string `mapstructure:"restricted_filter"`

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
	Users         map[string]*LDAPUser
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
	Name       string
	Restricted *bool
	Admin      *bool
	*ldap.Entry
}

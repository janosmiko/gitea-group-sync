package config

import (
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"

	"github.com/janosmiko/gitea-ldap-sync/internal/logger"
)

// Config describes the settings of the application. This structure is used in the settings-import process.
type Config struct {
	Gitea       *GiteaConfig `mapstructure:"gitea"`
	LDAP        *LDAPConfig  `mapstructure:"ldap"`
	SyncConfig  *SyncConfig  `mapstructure:"sync_config"`
	CronTimer   string       `mapstructure:"cron_timer"`
	CronEnabled bool         `mapstructure:"cron_enabled"`
}

type GiteaConfig struct {
	User         string `mapstructure:"user"`
	Token        string `mapstructure:"token"`
	BaseURL      string `mapstructure:"base_url"`
	AuthSourceID int64  `mapstructure:"auth_source_id"`
}

type LDAPConfig struct {
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

	GroupSearchBase           string `mapstructure:"group_search_base"`
	GroupFilter               string `mapstructure:"group_filter"`
	GroupNameAttribute        string `mapstructure:"group_name_attribute"`
	GroupFullNameAttribute    string `mapstructure:"group_fullname_attribute"`
	GroupDescriptionAttribute string `mapstructure:"group_description_attribute"`

	SubgroupSearchBase           string `mapstructure:"subgroup_search_base"`
	SubgroupFilter               string `mapstructure:"subgroup_filter"`
	SubgroupNameAttribute        string `mapstructure:"subgroup_name_attribute"`
	SubgroupDescriptionAttribute string `mapstructure:"subgroup_description_attribute"`

	ExcludeGroups         []string `mapstructure:"exclude_groups"`
	ExcludeGroupsRegex    string   `mapstructure:"exclude_groups_regex"`
	ExcludeSubgroups      []string `mapstructure:"exclude_subgroups"`
	ExcludeSubgroupsRegex string   `mapstructure:"exclude_subgroups_regex"`

	TrimParentName    bool   `mapstructure:"trim_parent_name"`
	SubgroupSeparator string `mapstructure:"subgroup_separator"`
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
		User struct {
			AllowCreateOrganization bool   `mapstructure:"allow_create_organization"`
			MaxRepoCreation         int    `mapstructure:"max_repo_creation"`
			Visibility              string `mapstructure:"visibility"`
		} `mapstructure:"user"`
	} `mapstructure:"defaults"`
}

func New() (*Config, error) {
	logger.Configure()

	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/gitea-ldap-sync")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// viper.SetTypeByDefaultValue(true)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	viper.SetTypeByDefaultValue(true)

	setDefaults()
	setBinds()

	for _, v := range viper.AllKeys() {
		log.Debug().Str("tag", "[config]").Msgf("%s: %s", v, viper.GetString(v))
	}

	cfg := &Config{}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	if err := cfg.check(); err != nil {
		return nil, err
	}

	return cfg, nil
}

//nolint:funlen
func setBinds() {
	_ = viper.BindEnv("gitea.base_url")
	_ = viper.BindEnv("gitea.user")
	_ = viper.BindEnv("gitea.token")
	_ = viper.BindEnv("gitea.auth_source_id")
	_ = viper.BindEnv("ldap.url")
	_ = viper.BindEnv("ldap.port")
	_ = viper.BindEnv("ldap.use_tls")
	_ = viper.BindEnv("ldap.allow_insecure_tls")
	_ = viper.BindEnv("ldap.bind_dn")
	_ = viper.BindEnv("ldap.bind_password")
	_ = viper.BindEnv("ldap.user_filter")
	_ = viper.BindEnv("ldap.user_search_base")
	_ = viper.BindEnv("ldap.user_username_attribute")
	_ = viper.BindEnv("ldap.user_fullname_attribute")
	_ = viper.BindEnv("ldap.user_first_name_attribute")
	_ = viper.BindEnv("ldap.user_surname_attribute")
	_ = viper.BindEnv("ldap.user_email_attribute")
	_ = viper.BindEnv("ldap.user_public_ssh_key_attribute")
	_ = viper.BindEnv("ldap.user_avatar_attribute")
	_ = viper.BindEnv("ldap.exclude_users")
	_ = viper.BindEnv("ldap.group_filter")
	_ = viper.BindEnv("ldap.group_search_base")
	_ = viper.BindEnv("ldap.group_name_attribute")
	_ = viper.BindEnv("ldap.group_fullname_attribute")
	_ = viper.BindEnv("ldap.group_description_attribute")
	_ = viper.BindEnv("ldap.subgroup_filter")
	_ = viper.BindEnv("ldap.subgroup_search_base")
	_ = viper.BindEnv("ldap.subgroup_name_attribute")
	_ = viper.BindEnv("ldap.subgroup_description_attribute")
	_ = viper.BindEnv("ldap.exclude_groups")
	_ = viper.BindEnv("ldap.exclude_groups_regex")
	_ = viper.BindEnv("ldap.exclude_subgroups")
	_ = viper.BindEnv("ldap.exclude_subgroups_regex")
	_ = viper.BindEnv("ldap.trim_parent_name")
	_ = viper.BindEnv("ldap.subgroup_separator")
	_ = viper.BindEnv("cron_timer")
	_ = viper.BindEnv("cron_enabled")
	_ = viper.BindEnv("sync_config.create_groups")
	_ = viper.BindEnv("sync_config.full_sync")
	_ = viper.BindEnv("sync_config.defaults.user.allow_create_organization")
	_ = viper.BindEnv("sync_config.defaults.user.max_repo_creation")
	_ = viper.BindEnv("sync_config.defaults.user.visibility")
	_ = viper.BindEnv("sync_config.defaults.organization.repo_admin_change_team_access")
	_ = viper.BindEnv("sync_config.defaults.organization.visibility")
	_ = viper.BindEnv("sync_config.defaults.team.can_create_org_repo")
	_ = viper.BindEnv("sync_config.defaults.team.includes_all_repositories")
	_ = viper.BindEnv("sync_config.defaults.team.permission")
	_ = viper.BindEnv("sync_config.defaults.team.units")
}

//nolint:funlen
func setDefaults() {
	viper.SetDefault("gitea.user", "")
	viper.SetDefault("gitea.token", "")
	viper.SetDefault("gitea.client_timeout", 10) //nolint:mnd
	viper.SetDefault("ldap.exclude_users", []string{"root"})
	viper.SetDefault("ldap.exclude_groups", []string{""})
	viper.SetDefault("ldap.exclude_subgroups", []string{""})
	viper.SetDefault("ldap.port", "389")
	viper.SetDefault("ldap.use_tls", true)
	viper.SetDefault("ldap.allow_insecure_tls", true)
	viper.SetDefault("ldap.user_username_attribute", "sAMAccountName")
	viper.SetDefault("ldap.user_fullname_attribute", "cn")
	viper.SetDefault("ldap.user_first_name_attribute", "name")
	viper.SetDefault("ldap.user_surname_attribute", "")
	viper.SetDefault("ldap.user_email_attribute", "mail")
	viper.SetDefault("ldap.user_public_ssh_key_attribute", "sshPublicKey")
	viper.SetDefault("ldap.user_avatar_attribute", "avatar")
	viper.SetDefault("ldap.admin_filter", "")
	viper.SetDefault("ldap.restricted_filter", "")
	viper.SetDefault("ldap.trim_parent_name", false)
	viper.SetDefault("ldap.group_name_attribute", "cn")
	viper.SetDefault("ldap.group_fullname_attribute", "cn")
	viper.SetDefault("ldap.group_description_attribute", "cn")
	viper.SetDefault("ldap.subgroup_name_attribute", "cn")
	viper.SetDefault("ldap.subgroup_description_attribute", "cn")
	viper.SetDefault("ldap.subgroup_separator", "/")
	viper.SetDefault("ldap.exclude_users_regex", "")
	viper.SetDefault("ldap.exclude_groups_regex", "")
	viper.SetDefault("ldap.exclude_subgroups_regex", "")
	viper.SetDefault("cron_timer", "@every 1m")
	viper.SetDefault("cron_enabled", true)
	viper.SetDefault("sync_config.create_groups", true)
	viper.SetDefault("sync_config.full_sync", false)
	viper.SetDefault("sync_config.defaults.user.allow_create_organization", false)
	viper.SetDefault("sync_config.defaults.user.max_repo_creation", 0)
	viper.SetDefault("sync_config.defaults.user.visibility", "private")
	viper.SetDefault("sync_config.defaults.organization.repo_admin_change_team_access", false)
	viper.SetDefault("sync_config.defaults.organization.visibility", "private")
	viper.SetDefault("sync_config.defaults.team.can_create_org_repo", false)
	viper.SetDefault("sync_config.defaults.team.includes_all_repositories", false)
	viper.SetDefault("sync_config.defaults.team.permission", "read")
	viper.SetDefault(
		"sync_config.defaults.team.units",
		`repo.code,repo.issues,repo.ext_issues,repo.wiki,repo.pulls,repo.releases,repo.projects,repo.ext_wiki`,
	)
}

func (c *Config) check() error {
	var missing []string

	if c.Gitea.User == "" {
		missing = append(missing, "GITEA_USER")
	}

	if c.Gitea.Token == "" {
		missing = append(missing, "GITEA_TOKEN")
	}

	if c.Gitea.BaseURL == "" {
		missing = append(missing, "GITEA_BASE_URL")
	}

	if c.Gitea.AuthSourceID == 0 {
		missing = append(missing, "GITEA_AUTH_SOURCE_ID")
	}

	if c.LDAP.URL == "" {
		missing = append(missing, "LDAP_URL")
	}

	if c.LDAP.BindDN == "" && c.LDAP.BindPassword == "" {
		missing = append(missing, "LDAP_BIND_DN", "LDAP_BIND_PASSWORD")
	}

	if c.LDAP.UserFilter == "" {
		missing = append(missing, "LDAP_USER_FILTER")
	}

	if c.LDAP.UserSearchBase == "" {
		missing = append(missing, "LDAP_USER_SEARCH_BASE")
	}

	if c.LDAP.GroupFilter == "" {
		missing = append(missing, "LDAP_GROUP_FILTER")
	}

	if c.LDAP.GroupSearchBase == "" {
		missing = append(missing, "LDAP_GROUP_SEARCH_BASE")
	}

	if c.LDAP.SubgroupFilter == "" {
		missing = append(missing, "LDAP_SUBGROUP_FILTER")
	}

	if c.LDAP.SubgroupSearchBase == "" {
		missing = append(missing, "LDAP_SUBGROUP_SEARCH_BASE")
	}

	if len(missing) != 0 {
		return errors.Errorf("required attribute is missing: %s", strings.Join(missing, ", "))
	}

	return nil
}

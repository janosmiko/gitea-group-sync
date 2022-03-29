package main

import (
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func initConfig() (*Config, error) {
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/gitea-ldap-sync")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// viper.SetTypeByDefaultValue(true)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	viper.SetTypeByDefaultValue(true)
	viper.SetDefault("gitea.token", []string{""})
	viper.SetDefault("ldap.exclude_users", []string{"root"})
	viper.SetDefault("ldap.exclude_groups", []string{""})
	viper.SetDefault("ldap.exclude_subgroups", []string{""})
	viper.SetDefault("gitea.client_timeout", 10) // nolint:gomnd
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
	viper.SetDefault("ldap.subgroup_separator", "/")
	viper.SetDefault("ldap.exclude_users_regex", "")
	viper.SetDefault("ldap.exclude_groups_regex", "")
	viper.SetDefault("ldap.exclude_subgroups_regex", "")
	viper.SetDefault("cron_timer", "@every 1m")
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
		"sync_config.defaults.team.units", `[
        "repo.code",
        "repo.issues",
        "repo.ext_issues",
        "repo.wiki",
        "repo.pulls",
        "repo.releases",
        "repo.projects",
        "repo.ext_wiki"
        ]`,
	)

	_ = viper.BindEnv("gitea.base_url")
	_ = viper.BindEnv("gitea.token")
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
	_ = viper.BindEnv("ldap.subgroup_filter")
	_ = viper.BindEnv("ldap.subgroup_search_base")
	_ = viper.BindEnv("ldap.exclude_groups")
	_ = viper.BindEnv("ldap.exclude_groups_regex")
	_ = viper.BindEnv("ldap.exclude_subgroups")
	_ = viper.BindEnv("ldap.exclude_subgroups_regex")
	_ = viper.BindEnv("ldap.trim_parent_name")
	_ = viper.BindEnv("ldap.subgroup_separator")
	_ = viper.BindEnv("cron_timer")
	_ = viper.BindEnv("sync_config.create_groups")
	_ = viper.BindEnv("sync_config.full_sync")
	_ = viper.BindEnv("sync_config.defaults.user.allow_create_organization")
	_ = viper.BindEnv("sync_config.defaults.user.max_repo_creation")
	_ = viper.BindEnv("sync_config.defaults.user.visibility")
	_ = viper.BindEnv("sync_config.defaults.organization.repo_admin_change_team_access")
	_ = viper.BindEnv("sync_config.defaults.organization.visibility")
	_ = viper.BindEnv("sync_config.team.can_create_org_repo")
	_ = viper.BindEnv("sync_config.team.includes_all_repositories")
	_ = viper.BindEnv("sync_config.team.permission")
	_ = viper.BindEnv("sync_config.team.units")

	for _, v := range viper.AllKeys() {
		zap.S().Debug(v, ": ", viper.GetString(v))
	}

	var cfg Config

	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}

	v := viper.GetViper()
	_ = v

	return &cfg, nil
}

func (c Config) checkConfig() {
	var missing bool
	if len(c.GiteaClient.Token) == 0 {
		zap.L().Info("GITEA_TOKEN is empty or invalid.")
		missing = true
	}
	if len(c.GiteaClient.BaseURL) == 0 {
		zap.L().Info("GITEA_BASE_URL is empty")
		missing = true
	}
	if len(c.LDAP.URL) == 0 {
		zap.L().Info("LDAP_URL is empty")
		missing = true
	}
	if c.LDAP.Port <= 0 {
		zap.L().Info("LDAP_PORT is empty, using default: 389.")
	}
	if len(c.LDAP.BindDN) > 0 && len(c.LDAP.BindPassword) == 0 {
		zap.L().Info("LDAP_BIND_DN supplied, but BIND_PASSWORD is empty")
		missing = true
	}
	if len(c.LDAP.UserFilter) == 0 {
		zap.L().Info("LDAP_USER_FILTER is empty")
		missing = true
	}
	if len(c.LDAP.UserSearchBase) == 0 {
		zap.L().Info("LDAP_USER_SEARCH_BASE is empty")
		missing = true
	}
	if len(c.LDAP.UserUsernameAttribute) == 0 {
		zap.L().Info("LDAP_USER_USERNAME_ATTRIBUTE is empty, using default: 'sAMAccountName'")
	}
	if len(c.LDAP.UserFullNameAttribute) == 0 {
		zap.L().Info("LDAP_USER_FULLNAME is empty, using default: 'cn'")
	}
	if len(c.LDAP.GroupFilter) == 0 {
		zap.L().Info("LDAP_GROUP_FILTER is empty")
		missing = true
	}
	if len(c.LDAP.GroupSearchBase) == 0 {
		zap.L().Info("LDAP_GROUP_SEARCH_BASE is empty")
		missing = true
	}
	if len(c.LDAP.SubGroupFilter) == 0 {
		zap.L().Info("LDAP_SUBGROUP_FILTER is empty")
		missing = true
	}
	if len(c.LDAP.SubGroupSearchBase) == 0 {
		zap.L().Info("LDAP_SUBGROUP_SEARCH_BASE is empty")
		missing = true
	}

	if missing {
		zap.L().Fatal("Required attribute is missing.")
	}
}

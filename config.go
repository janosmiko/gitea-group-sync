package main

import (
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func initConfig() (*Config, error) {
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/gitea-group-sync")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// viper.SetTypeByDefaultValue(true)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()

	viper.SetTypeByDefaultValue(true)
	viper.SetDefault("gitea.token", []string{""})
	viper.SetDefault("ldap.exclude_groups", []string{""})
	viper.SetDefault("ldap.exclude_subgroups", []string{""})
	viper.SetDefault("gitea.client_timeout", 10) // nolint:gomnd
	viper.SetDefault("ldap.port", "389")
	viper.SetDefault("ldap.use_tls", true)
	viper.SetDefault("ldap.allow_insecure_tls", true)
	viper.SetDefault("ldap.user_identity_attribute", "sAMAccountName")
	viper.SetDefault("ldap.user_fullname", "cn")
	viper.SetDefault("cron_timer", "@every 1m")
	viper.SetDefault("sync_config.create_groups", true)
	viper.SetDefault("sync_config.full_sync", false)
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
	// nolint:lll
	viper.SetDefault(
		"sync_config.defaults.team.units_map",
		`{"repo.code":"write","repo.issues":"write","repo.ext_issues":"none","repo.wiki":"write","repo.pulls":"owner","repo.releases":"none","repo.projects":"none","repo.ext_wiki":"none"}`,
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
	_ = viper.BindEnv("ldap.user_identity_attribute")
	_ = viper.BindEnv("ldap.user_fullname")
	_ = viper.BindEnv("ldap.group_filter")
	_ = viper.BindEnv("ldap.group_search_base")
	_ = viper.BindEnv("ldap.subgroup_filter")
	_ = viper.BindEnv("ldap.subgroup_search_base")
	_ = viper.BindEnv("ldap.exclude_groups")
	_ = viper.BindEnv("ldap.exclude_groups_regex")
	_ = viper.BindEnv("ldap.exclude_subgroups")
	_ = viper.BindEnv("ldap.exclude_subgroups_regex")
	_ = viper.BindEnv("cron_timer")
	_ = viper.BindEnv("sync_config.create_groups")
	_ = viper.BindEnv("sync_config.full_sync")
	_ = viper.BindEnv("sync_config.defaults.organization.repo_admin_change_team_access")
	_ = viper.BindEnv("sync_config.defaults.organization.visibility")
	_ = viper.BindEnv("sync_config.team.can_create_org_repo")
	_ = viper.BindEnv("sync_config.team.includes_all_repositories")
	_ = viper.BindEnv("sync_config.team.permission")
	_ = viper.BindEnv("sync_config.team.units")
	_ = viper.BindEnv("sync_config.team.units_map")

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
	if len(c.LDAP.UserIdentityAttribute) == 0 {
		zap.L().Info("LDAP_USER_IDENTITY_ATTRIBUTE is empty, using default: 'sAMAccountName'")
	}
	if len(c.LDAP.UserFullName) == 0 {
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

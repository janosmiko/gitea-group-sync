package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"syscall"

	"code.gitea.io/sdk/gitea"
	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/ldap.v3"
)

func main() {
	conf := zap.NewDevelopmentConfig()
	conf.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	if os.Getenv("DEBUG") == "true" {
		conf.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		conf.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := conf.Build()
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	zap.ReplaceGlobals(logger)

	mainJob() // First run for check settings

	c := cron.New(
		cron.WithChain(
			cron.SkipIfStillRunning(cron.VerbosePrintfLogger(log.New(os.Stdout, "cron: ", log.LstdFlags))),
		),
	)
	_, _ = c.AddFunc(viper.GetString("cron_timer"), mainJob)
	fmt.Println(c.Entries())
	go c.Start()

	sig := make(chan os.Signal, 1)

	signal.Notify(
		sig,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	<-sig
}

func mainJob() {
	var c *Config
	var err error

	c, err = initConfig()
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	// Checks Config
	c.checkConfig()
	zap.L().Info("Configuration is valid.")

	gtc, err := NewGiteaClient()
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	c.GiteaClient = gtc

	c.LDAP.initLDAP()
	defer c.LDAP.closeLDAP()

	ldapDirectory := c.LDAP.getLDAPDirectory()

	// Check organizations and teams in Gitea, add users to them.
	giteaOrgs, err := c.GiteaClient.ListOrganizations()
	if err != nil {
		zap.L().Error(err.Error())
	}
	zap.S().Infof("%d Organization Groups were found in the LDAP server.", len(ldapDirectory.Organizations))
	zap.S().Infof("%d Organizations were found in Gitea.", len(giteaOrgs))

	giteaUsers, err := c.GiteaClient.AdminListUsers()
	if err != nil {
		zap.L().Error(err.Error())
	}
	zap.S().Infof("%d Users were found in the LDAP server.", len(ldapDirectory.Users))
	zap.S().Infof("%d Users were found in Gitea.", len(giteaUsers))

	if c.SyncConfig.CreateGroups {
		if err = c.syncLDAPUsersToGitea(ldapDirectory, giteaUsers); err != nil {
			zap.L().Error(err.Error())
		}

		if err = c.syncLDAPGroupsToGitea(ldapDirectory, giteaOrgs); err != nil {
			zap.L().Error(err.Error())
		}
	}

	if err = c.syncGiteaUsersByLDAP(giteaUsers, ldapDirectory); err != nil {
		zap.L().Error(err.Error())
	}
	if err = c.syncGiteaGroupsByLDAP(giteaOrgs, ldapDirectory); err != nil {
		zap.L().Error(err.Error())
	}

	zap.L().Info("Main process finished...")
}

func (c *Config) syncLDAPUsersToGitea(ldapDirectory *Directory, giteaUsers []*gitea.User) error {
	zap.L().Info("=======================================")
	zap.L().Info("Syncing Users from LDAP.")

	for _, u := range ldapDirectory.Users {
		if len(c.LDAP.ExcludeUsersRegex) > 0 {
			r := regexp.MustCompile(c.LDAP.ExcludeUsersRegex)
			if r.MatchString(u.Name) {
				zap.S().Infof(
					`Syncing LDAP User "%v" is skipped. Reason: it's on the regex exclude list.`,
					u.Name,
				)
				continue
			}
		}
		if contains(c.LDAP.ExcludeUsers, u.Name) {
			zap.S().Infof(`Syncing LDAP User "%v" is skipped. Reason: it's on the exclude list.`, u.Name)
			continue
		}

		zap.S().Infof(`Syncing LDAP User "%v" to Gitea.`, u.Name)

		exists, err := existInSlice(u.Name, giteaUsers)
		if err != nil {
			return err
		}

		if !exists {
			zap.S().Infof(`User "%v" does not exist in Gitea, creating...`, u.Name)

			fn := u.GetAttributeValue(viper.GetString("ldap.user_first_name_attribute"))
			sn := u.GetAttributeValue(viper.GetString("ldap.user_surname_attribute"))
			fullname := ""
			if len(fn) > 0 && len(sn) > 0 {
				fullname = fmt.Sprintf("%v %v", fn, sn)
			} else {
				fullname = u.GetAttributeValue(viper.GetString("ldap.user_fullname_attribute"))
			}

			user := GiteaUser{
				User: &gitea.User{
					UserName:   u.GetAttributeValue(viper.GetString("ldap.user_username_attribute")),
					FullName:   fullname,
					Email:      u.GetAttributeValue(viper.GetString("ldap.user_email_attribute")),
					AvatarURL:  u.GetAttributeValue(viper.GetString("ldap.user_avatar_attribute")),
					IsAdmin:    *u.Admin,
					Restricted: *u.Restricted,
					Visibility: gitea.VisibleTypePrivate,
				},
			}
			if err := c.GiteaClient.CreateUser(user); err != nil {
				return err
			}
		} else {
			zap.S().Infof(`User "%v" already exist in Gitea...`, u.Name)
		}
	}

	zap.L().Info("Syncing Groups and Subgroups from LDAP finished.")

	return nil
}

// syncLDAPToGitea iterates through the ldapDirectory.
// Creates Gitea Organizations based on the LDAP Groups and creates Gitea Teams based on the LDAP Subgroups.
// nolint: gocognit
func (c *Config) syncLDAPGroupsToGitea(ldapDirectory *Directory, giteaOrganizations []*gitea.Organization) error {
	zap.L().Info("=======================================")
	zap.L().Info("Syncing Groups and Subgroups from LDAP.")

	for _, o := range ldapDirectory.Organizations {
		if len(c.LDAP.ExcludeGroupsRegex) > 0 {
			r := regexp.MustCompile(c.LDAP.ExcludeGroupsRegex)
			if r.MatchString(o.Name) {
				zap.S().Infof(
					`Syncing LDAP Group "%v" is skipped. Reason: it's on the regex exclude list.`,
					o.Name,
				)
				continue
			}
		}
		if contains(c.LDAP.ExcludeGroups, o.Name) {
			zap.S().Infof(`Syncing LDAP Group "%v" is skipped. Reason: it's on the exclude list.`, o.Name)
			continue
		}

		zap.S().Infof(`Syncing LDAP Group "%v" to Gitea as an Organization.`, o.Name)

		exists, err := existInSlice(o.Name, giteaOrganizations)
		if err != nil {
			return err
		}

		if !exists {
			zap.S().Infof(`Group "%v" does not exist in Gitea, creating...`, o.Name)
			org := GiteaOrganization{
				Organization: &gitea.Organization{
					UserName:    o.Name,
					FullName:    o.GetAttributeValue(viper.GetString("ldap.group_fullname_attribute")),
					Description: o.GetAttributeValue(viper.GetString("ldap.group_description_attribute")),
					Visibility:  viper.GetString("sync_config.defaults.organization.visibility"),
				},
			}
			zap.S().Infof(
				`Organization name: "%v", full name: "%v", description: "%v"`, org.UserName,
				org.FullName,
				org.Description,
			)

			if err := c.GiteaClient.CreateOrganization(org); err != nil {
				return err
			}
		}

		giteaTeams, err := c.GiteaClient.ListTeams(o.Name)
		if err != nil {
			return err
		}
		for _, t := range o.Teams {
			if len(c.LDAP.ExcludeSubgroupsRegex) > 0 {
				r := regexp.MustCompile(c.LDAP.ExcludeSubgroupsRegex)
				if r.MatchString(t.Name) {
					zap.S().Infof(
						`Syncing LDAP Subroup "%v" is skipped. Reason: it's on the regex exclude list.`,
						t.Name,
					)
					continue
				}
			}
			if contains(c.LDAP.ExcludeSubgroups, o.Name) {
				zap.S().Infof(
					`Syncing LDAP Subroup "%v" is skipped. Reason: it's on the exclude list.`,
					t.Name,
				)
				continue
			}
			zap.S().Infof(`Syncing LDAP Subgroup "%v" to Gitea as a Team.`, t.Name)

			exists, err = existInSlice(t.Name, giteaTeams)
			if err != nil {
				return err
			}

			if !exists {
				zap.S().Infof(`Team "%v" does not exist in Gitea, creating...`, t.Name)
				team := GiteaTeam{
					Team: &gitea.Team{
						Name:        t.Name,
						Description: t.GetAttributeValue(viper.GetString("ldap.subgroup_description_attribute")),
					},
				}
				opts := GiteaCreateTeamOpts{
					Permission:              c.SyncConfig.Defaults.Team.Permission,
					CanCreateOrgRepo:        c.SyncConfig.Defaults.Team.CanCreateOrgRepo,
					IncludesAllRepositories: c.SyncConfig.Defaults.Team.IncludesAllRepositories,
					Units:                   c.SyncConfig.Defaults.Team.Units,
					// UnitsMap:                c.SyncConfig.Defaults.Team.UnitsMap,
				}

				zap.S().Infof(`Team name: "%v", description: "%v"`, team.Name, team.Description)
				if err = c.GiteaClient.CreateTeam(o.Name, team, opts); err != nil {
					return err
				}
			}
		}
	}
	zap.L().Info("Syncing Groups and Subgroups from LDAP finished.")

	return nil
}

func (c *Config) syncGiteaUsersByLDAP(giteaUsers []*gitea.User, ldapDirectory *Directory) error {
	zap.L().Info("=======================================")
	zap.L().Info("Syncing Users in Gitea.")

	for _, u := range giteaUsers {
		zap.S().Infof("Begin user review: %v", u.UserName)

		if len(c.LDAP.ExcludeUsersRegex) > 0 {
			r := regexp.MustCompile(c.LDAP.ExcludeUsersRegex)
			if r.MatchString(u.UserName) {
				zap.S().Infof(
					`Syncing LDAP User "%v" is skipped. Reason: it's on the regex exclude list.`,
					u.UserName,
				)
				continue
			}
		}
		if contains(c.LDAP.ExcludeUsers, u.UserName) {
			zap.S().Infof(
				`Syncing LDAP User "%v" is skipped. Reason: it's on the exclude list.`, u.UserName,
			)
			continue
		}

		user := ldapDirectory.Users[u.UserName]

		zap.L().Info("Checking user in LDAP: " + u.UserName)
		switch {
		case user != nil:
			zap.S().Infof(`User "%v" exist in LDAP.`, u.UserName)
		case c.SyncConfig.FullSync:
			zap.S().Infof(
				`User "%v" does not exist in LDAP. Full Sync is enabled. Deleting from Gitea...`,
				u.UserName,
			)
			if err := c.GiteaClient.DeleteUser(u.UserName); err != nil {
				return err
			}
			continue
		default:
			zap.S().Infof(
				`User "%v" does not exist in LDAP. Full Sync is not enabled. Continue...`,
				u.UserName,
			)
			continue
		}
	}

	return nil
}

// syncGiteaByLDAP iterates through all Gitea Organizations and all Teams inside the organizations.
// It is going to attach the Gitea Users to Gitea Teams (if the LDAP users are members of the LDAP Subgroups and if
// those LDAP Subgroups are members of LDAP Groups).
// nolint: gocognit
func (c *Config) syncGiteaGroupsByLDAP(
	giteaOrganizations []*gitea.Organization, ldapDirectory *Directory,
) error {
	zap.L().Info("=======================================")
	zap.L().Info("Syncing Users to Teams in Gitea.")

	orgnames := make([]string, len(giteaOrganizations))
	for i, v := range giteaOrganizations {
		orgnames[i] = v.UserName
	}

	zap.S().Infof("Organizations in Gitea: %v", orgnames)

	for _, o := range giteaOrganizations {
		zap.S().Infof(
			"Begin an organization review: OrganizationName: %v, OrganizationId: %d.",
			o.UserName,
			o.ID,
		)

		teamList, err := c.GiteaClient.ListTeams(o.UserName)
		if err != nil {
			return err
		}
		zap.S().Infof("%d teams were found in %s organization", len(teamList), o.UserName)

		var org *LDAPOrganization
		var team *LDAPTeam

		org = ldapDirectory.Organizations[o.UserName]
		zap.L().Info("Checking organization in LDAP: " + o.UserName)
		switch {
		case org != nil:
			zap.S().Infof(`Organization "%v" exist in LDAP.`, o.UserName)
		case c.SyncConfig.FullSync:
			zap.S().Infof(
				`Organization "%v" does not exist in LDAP. Full Sync is enabled. Deleting from Gitea...`,
				o.UserName,
			)
			if err = c.GiteaClient.DeleteOrganization(o.UserName); err != nil {
				return err
			}
			continue
		default:
			zap.S().Infof(
				`Organization "%v" does not exist in LDAP. Full Sync is not enabled. Continue...`,
				o.UserName,
			)
			continue
		}

		for _, t := range teamList {
			team = org.Teams[t.Name]

			if t.Name == "Owners" {
				zap.S().Infof(`Syncing "%v" team is skipped.`, t.Name)
				continue
			}
			zap.S().Infof(`Syncing "%v" team.`, t.Name)

			var accountsInGitea map[string]GiteaAccount
			var addUserToTeamList []GiteaAccount
			var delUserFromTeamList []GiteaAccount

			accountsInGitea, err = c.GiteaClient.ListTeamUsers(t.ID)
			if err != nil {
				return err
			}
			zap.S().Infof(
				"Gitea has %d users corresponding to Team (name: %s, id=%d).",
				len(accountsInGitea), t.Name, t.ID,
			)

			switch {
			case team != nil:
				zap.S().Infof("Checking team in LDAP: %v.", team.Name)
				for _, u := range team.Users {
					zap.S().Infof("Processing user: %v.", u.Name)
					if accountsInGitea[u.Name].Login != u.Entry.GetAttributeValue(c.LDAP.UserUsernameAttribute) {
						acc := GiteaAccount{
							Login:    u.Entry.GetAttributeValue(c.LDAP.UserUsernameAttribute),
							FullName: u.Entry.GetAttributeValue(c.LDAP.UserFullNameAttribute),
						}

						addUserToTeamList = append(addUserToTeamList, acc)
					}
				}
				zap.S().Infof(`Users %v can be added to Team "%v".`, addUserToTeamList, team.Name)
				if err = c.GiteaClient.AddUsersToTeam(addUserToTeamList, t.ID); err != nil {
					return err
				}

				for _, v := range accountsInGitea {
					exists, err := existInSlice(v.Login, team.Users)
					if err != nil {
						return err
					}

					if !exists {
						delUserFromTeamList = append(delUserFromTeamList, v)
					}
				}

				zap.S().Infof(`Users %v can be deleted from Team "%v".`, delUserFromTeamList, team.Name)
				if err = c.GiteaClient.DelUsersFromTeam(
					delUserFromTeamList, t.ID,
				); err != nil {
					return err
				}

			case c.SyncConfig.FullSync:
				zap.L().Info(`Organization "" does not exist in LDAP. Full sync is enabled. Deleting from Gitea...`)
				if err = c.GiteaClient.DeleteTeam(t.ID); err != nil {
					return err
				}
				continue
			}
		}
	}

	zap.L().Info("Syncing Users to Teams in Gitea finished.")

	return nil
}

func difference(a, b []*ldap.Entry) []*ldap.Entry {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x.GetAttributeValue("cn")] = struct{}{}
	}
	var diff []*ldap.Entry
	for _, x := range a {
		if _, found := mb[x.GetAttributeValue("cn")]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func existInSlice(s string, slice interface{}) (bool, error) { // nolint:gocognit
	switch t := slice.(type) {
	case []GiteaOrganization:
		for _, v := range t {
			if v.UserName == s {
				return true, nil
			}
		}
		return false, nil
	case []*gitea.Organization:
		for _, v := range t {
			if v.UserName == s {
				return true, nil
			}
		}
		return false, nil
	case []GiteaTeam:
		for _, v := range t {
			if v.Name == s {
				return true, nil
			}
		}
		return false, nil
	case []*gitea.Team:
		for _, v := range t {
			if v.Name == s {
				return true, nil
			}
		}
		return false, nil
	case map[string]*LDAPUser:
		for _, v := range t {
			if v.Name == s {
				return true, nil
			}
		}
		return false, nil
	case map[string]*gitea.User:
		for _, v := range t {
			if v.UserName == s {
				return true, nil
			}
		}
		return false, nil
	case []*gitea.User:
		for _, v := range t {
			if v.UserName == s {
				return true, nil
			}
		}
		return false, nil
	}

	return false, errors.New("unknown type") // nolint: goerr113
}

func contains(s []string, searchterm string) bool {
	sort.Strings(s)
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

func OptionalBool(b bool) *bool {
	return &b
}

func OptionalVisibility(visibleType gitea.VisibleType) *gitea.VisibleType {
	v := &visibleType

	return v
}

func OptionalInt(v int) *int {
	return &v
}

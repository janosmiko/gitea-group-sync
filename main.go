package main

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/ldap.v3"
)

func main() {
	conf := zap.NewDevelopmentConfig()
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

	c := cron.New()
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

	c.LDAP.initLDAP()
	defer c.LDAP.closeLDAP()

	ldapDirectory := c.LDAP.getLDAPDirectory()

	// Check organizations and teams in Gitea, add users to them.
	giteaOrgs := c.GiteaClient.RequestOrganizationList()
	zap.S().Infof("%d Groups were found in the LDAP server: %s", len(ldapDirectory.Organizations), c.LDAP.URL)
	zap.S().Infof("%d Organizations were found in the Gitea server: %s", len(giteaOrgs), c.GiteaClient.BaseURL)

	if c.SyncConfig.CreateGroups {
		c.syncLDAPToGitea(ldapDirectory, giteaOrgs)
	}

	c.syncGiteaByLDAP(giteaOrgs, ldapDirectory)
}

// syncLDAPToGitea iterates through the ldapDirectory.
// Creates Gitea Organizations based on the LDAP Groups and creates Gitea Teams based on the LDAP Subgroups.
func (c *Config) syncLDAPToGitea(ldapDirectory *Directory, giteaOrganizations []GiteaOrganization) {
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

		if !existInSlice(o.Name, giteaOrganizations) {
			zap.S().Infof(`Group "%v" does not exist in Gitea, creating...`, o.Name)
			org := GiteaOrganization{
				Description: o.Name,
				FullName:    o.Name,
				Name:        o.Name,
				Visibility:  "private",
			}
			c.GiteaClient.CreateOrganization(org)
		}

		giteaTeams := c.GiteaClient.RequestTeamList(o.Name)
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
			if !existInSlice(t.Name, giteaTeams) {
				zap.S().Infof(`Team "%v" does not exist in Gitea, creating...`, t.Name)
				team := GiteaTeam{
					Name:        t.Name,
					Description: t.Name,
				}
				opts := GiteaCreateTeamOpts{
					Permission:              c.SyncConfig.Defaults.Team.Permission,
					CanCreateOrgRepo:        c.SyncConfig.Defaults.Team.CanCreateOrgRepo,
					IncludesAllRepositories: c.SyncConfig.Defaults.Team.IncludesAllRepositories,
					Units:                   c.SyncConfig.Defaults.Team.Units,
					UnitsMap:                c.SyncConfig.Defaults.Team.UnitsMap,
				}
				c.GiteaClient.CreateTeam(o.Name, team, opts)
			}
		}
	}
	zap.L().Info("Syncing Groups and Subgroups from LDAP finished.")
}

// syncGiteaByLDAP iterates through all Gitea Organizations and all Teams inside the organizations.
// It is going to attach the Gitea Users to Gitea Teams (if the LDAP users are members of the LDAP Subgroups and if
// those LDAP Subgroups are members of LDAP Groups).
func (c *Config) syncGiteaByLDAP(
	giteaOrganizations []GiteaOrganization, ldapDirectory *Directory,
) {
	zap.L().Info("=======================================")
	zap.L().Info("Syncing Users to Teams in Gitea.")

	orgNames := make([]string, len(giteaOrganizations))
	for i, v := range giteaOrganizations {
		orgNames[i] = v.Name
	}

	zap.S().Infof("Organizations in Gitea: %v", orgNames)

	for i := 0; i < len(giteaOrganizations); i++ {
		zap.S().Infof(
			"Begin an organization review: OrganizationName: %v, OrganizationId: %d \n",
			giteaOrganizations[i].Name,
			giteaOrganizations[i].ID,
		)

		teamList := c.GiteaClient.RequestTeamList(giteaOrganizations[i].Name)
		zap.S().Infof("%d teams were found in %s organization", len(teamList), giteaOrganizations[i].Name)
		// c.GiteaClient.BruteforceTokenKey = 0

		var org *LDAPOrganization
		var team *LDAPTeam

		org = ldapDirectory.Organizations[giteaOrganizations[i].Name]
		zap.L().Info("Checking organization in LDAP: " + giteaOrganizations[i].Name)
		switch {
		case org != nil:
			zap.S().Infof(`Organization "%v" exist in LDAP.`, giteaOrganizations[i].Name)
		case c.SyncConfig.FullSync:
			zap.S().Infof(
				`Organization "%v" does not exist in LDAP. Deleting from Gitea...`,
				giteaOrganizations[i].Name,
			)
			c.GiteaClient.DeleteOrganization(giteaOrganizations[i].Name)
			continue
		default:
			zap.S().Infof(
				`Organization "%v" does not exist in LDAP.`,
				giteaOrganizations[i].Name,
			)
			continue
		}

		for j := 0; j < len(teamList); j++ {
			team = org.Teams[teamList[j].Name]

			if teamList[j].Name == "Owners" {
				zap.S().Infof(`Syncing "%v" team is skipped.`, teamList[j].Name)
				continue
			}
			zap.S().Infof(`Syncing "%v" team.`, teamList[j].Name)

			var accountsInGitea map[string]GiteaAccount
			var addUserToTeamList []GiteaAccount
			var delUserFromTeamList []GiteaAccount

			accountsInGitea, c.GiteaClient.BruteforceToken = c.GiteaClient.RequestUsersList(teamList[j].ID)
			zap.S().Infof(
				"Gitea has %d users corresponding to Team (name: %s, id=%d)",
				len(accountsInGitea), teamList[j].Name, teamList[j].ID,
			)

			switch {
			case team != nil:
				zap.L().Info("Checking team in LDAP: " + team.Name)
				for _, u := range team.Users {
					zap.S().Info("Processing user: " + u.Name)
					if accountsInGitea[u.Name].Login != u.Entry.GetAttributeValue(c.LDAP.UserIdentityAttribute) {
						acc := GiteaAccount{
							FullName: u.Entry.GetAttributeValue(c.LDAP.UserFullName),
							Login:    u.Entry.GetAttributeValue(c.LDAP.UserIdentityAttribute),
						}

						addUserToTeamList = append(addUserToTeamList, acc)
					}
				}
				zap.S().Infof(`Users %v can be added to Team "%v"`, addUserToTeamList, team.Name)
				c.GiteaClient.AddUsersToTeam(addUserToTeamList, teamList[j].ID)

				for _, v := range accountsInGitea {
					if !existInSlice(v.Login, team.Users) {
						delUserFromTeamList = append(delUserFromTeamList, v)
					}
				}

				zap.S().Infof(`Users %v can be deleted from Team "%v"`, delUserFromTeamList, team.Name)
				c.GiteaClient.DelUsersFromTeam(delUserFromTeamList, teamList[j].ID)

			case c.SyncConfig.FullSync:
				zap.L().Info(`Organization "" does not exist in LDAP. Deleting from Gitea...`)
				c.GiteaClient.DeleteTeam(teamList[j].ID)
				continue
			}
		}
	}

	zap.L().Info("Syncing Users to Teams in Gitea finished.")
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

func existInSlice(s string, slice interface{}) bool {
	switch t := slice.(type) {
	case []GiteaOrganization:
		for _, v := range t {
			if v.Name == s {
				return true
			}
		}
	case []GiteaTeam:
		for _, v := range t {
			if v.Name == s {
				return true
			}
		}
	case map[string]*LDAPUser:
		for _, v := range t {
			if v.Name == s {
				return true
			}
		}
	}

	return false
}

func contains(s []string, searchterm string) bool {
	sort.Strings(s)
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}

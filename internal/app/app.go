package app

import (
	"regexp"

	giteapkg "code.gitea.io/sdk/gitea"
	"github.com/pkg/errors"

	"github.com/janosmiko/gitea-ldap-sync/internal/config"
	"github.com/janosmiko/gitea-ldap-sync/internal/gitea"
	"github.com/janosmiko/gitea-ldap-sync/internal/ldap"
	"github.com/janosmiko/gitea-ldap-sync/internal/logger"
	"github.com/janosmiko/gitea-ldap-sync/internal/stringslice"
)

type Client struct {
	Config *config.Config
	LDAP   *ldap.Client
	Gitea  *gitea.Client
	log    logger.Logger
}

func New(cfg *config.Config) (*Client, error) {
	ldapClient, err := ldap.New(cfg)
	if err != nil {
		return nil, err
	}

	giteaClient, err := gitea.New(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{
		Config: cfg,
		LDAP:   ldapClient,
		Gitea:  giteaClient,
		log:    logger.New().Tag("app"),
	}, nil
}

func (c *Client) Close() {
	c.LDAP.Close()
}

func (c *Client) Run() error {
	ldapDirectory, err := c.LDAP.GetDirectory()
	if err != nil {
		return err
	}

	if c.Config.SyncConfig.CreateGroups {
		if err = c.syncLDAPUsersToGitea(ldapDirectory); err != nil {
			return err
		}

		if err = c.syncLDAPGroupsToGitea(ldapDirectory); err != nil {
			return err
		}
	}

	if err = c.removeGiteaUsersNotInLDAP(ldapDirectory); err != nil {
		return err
	}

	if err = c.syncGiteaGroupHierarchyWithLDAP(ldapDirectory); err != nil {
		return err
	}

	return nil
}

func (c *Client) syncLDAPUsersToGitea(ldapDirectory *ldap.Directory) error {
	c.log.Tag("sync-users-to-gitea")
	c.log.Info().Msg("Syncing users from ldap to gitea")

	for _, u := range ldapDirectory.Users {
		c.log.Info().Msgf("Processing ldap user: %s", u.Name)

		if len(c.Config.LDAP.ExcludeUsersRegex) > 0 {
			r := regexp.MustCompile(c.Config.LDAP.ExcludeUsersRegex)
			if r.MatchString(u.Name) {
				c.log.Info().Msgf("User skipped (reason: regex-exclude-list): %s", u.Name)

				continue
			}
		}

		if stringslice.Contains(c.Config.LDAP.ExcludeUsers, u.Name) {
			c.log.Info().Msgf("User skipped (reason: exclude-list): %s", u.Name)

			continue
		}

		if err := c.Gitea.CreateOrUpdateUser(
			gitea.User{
				UserName:   u.GetAttributeValue(c.Config.LDAP.UserUsernameAttribute),
				FullName:   u.Fullname(c.Config.LDAP),
				Email:      u.GetAttributeValue(c.Config.LDAP.UserEmailAttribute),
				AvatarURL:  u.GetAttributeValue(c.Config.LDAP.UserAvatarAttribute),
				IsAdmin:    *u.Admin,
				Restricted: *u.Restricted,
				Visibility: giteapkg.VisibleTypePrivate,
			},
		); err != nil {
			return err
		}
	}

	c.log.Info().Msg("Syncing users from ldap to gitea finished")

	return nil
}

// syncLDAPGroupsToGitea iterates through the ldapDirectory.
// Creates Gitea Organizations based on the LDAP Groups and creates Gitea Teams based on the LDAP Subgroups.
func (c *Client) syncLDAPGroupsToGitea(ldapDirectory *ldap.Directory) error {
	c.log.Tag("sync-groups-to-gitea")
	c.log.Info().Msg("Syncing groups and subgroups from ldap")

	for _, o := range ldapDirectory.Organizations {
		if err := c.syncOrg(o); err != nil {
			return err
		}
	}

	c.log.Info().Msg("Syncing groups and subgroups from ldap finished")

	return nil
}

func (c *Client) syncOrg(o *ldap.Organization) error {
	c.log.Debug().Msgf("Processing group: %s", o.Name)

	if c.Config.LDAP.ExcludeGroupsRegex != "" {
		r := regexp.MustCompile(c.Config.LDAP.ExcludeGroupsRegex)
		if r.MatchString(o.Name) {
			c.log.Info().Msgf("Group skipped (reason: regex-exclude-list): %s", o.Name)

			return nil
		}
	}

	if stringslice.Contains(c.Config.LDAP.ExcludeGroups, o.Name) {
		c.log.Info().Msgf("Group skipped (reason: exclude-list): %s", o.Name)

		return nil
	}

	c.log.Debug().Msgf("Syncing ldap group to gitea as an organization: %s", o.Name)

	if err := c.Gitea.CreateOrganization(
		gitea.Organization{
			UserName:    o.Name,
			FullName:    o.GetAttributeValue(c.Config.LDAP.GroupFullNameAttribute),
			Description: o.GetAttributeValue(c.Config.LDAP.GroupDescriptionAttribute),
			Visibility:  c.Config.SyncConfig.Defaults.Organization.Visibility,
		},
	); err != nil {
		return err
	}

	if err := c.syncTeams(o); err != nil {
		return err
	}

	c.log.Info().Msgf("Group processed: %s", o.Name)

	return nil
}

func (c *Client) syncTeams(o *ldap.Organization) error {
	for _, t := range o.Teams {
		if err := c.syncTeam(o, t); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) syncTeam(o *ldap.Organization, t *ldap.Team) error {
	c.log.Debug().Msgf("Processing subgroup %s", t.Name)

	if len(c.Config.LDAP.ExcludeSubgroupsRegex) > 0 {
		r := regexp.MustCompile(c.Config.LDAP.ExcludeSubgroupsRegex)
		if r.MatchString(t.Name) {
			c.log.Info().Msgf("Subroup skipped (reason: regex-exclude-list): %s", t.Name)

			return nil
		}
	}

	if stringslice.Contains(c.Config.LDAP.ExcludeSubgroups, o.Name) {
		c.log.Info().Msgf("Subgroup skipped (reason: exclude-list): %s", t.Name)

		return nil
	}

	if err := c.Gitea.CreateTeam(
		o.Name,
		gitea.Team{
			Name:        t.Name,
			Description: t.GetAttributeValue(c.Config.LDAP.SubgroupDescriptionAttribute),
		},
		gitea.CreateTeamOpts{
			Permission:              c.Config.SyncConfig.Defaults.Team.Permission,
			CanCreateOrgRepo:        c.Config.SyncConfig.Defaults.Team.CanCreateOrgRepo,
			IncludesAllRepositories: c.Config.SyncConfig.Defaults.Team.IncludesAllRepositories,
			Units:                   c.Config.SyncConfig.Defaults.Team.Units,
			// UnitsMap:                c.SyncConfig.Defaults.Team.UnitsMap,
		},
	); err != nil {
		return err
	}

	c.log.Info().Msgf("Subgroup processed: %s", t.Name)

	return nil
}

func (c *Client) removeGiteaUsersNotInLDAP(ldapDirectory *ldap.Directory) error {
	c.log.Tag("remove-users-from-gitea")
	c.log.Info().Msg("Syncing Users in Gitea")

	giteaUsers, err := c.Gitea.ListUsers()
	if err != nil {
		return err
	}

	c.log.Info().Msgf("%d Users were found in the LDAP server.", len(ldapDirectory.Users))
	c.log.Info().Msgf("%d Users were found in Gitea.", len(giteaUsers))

	for _, giteaUser := range giteaUsers {
		if giteaUser.UserName == "root" {
			c.log.Info().Msgf("User skipped (reason: root): %s", giteaUser.UserName)

			continue
		}

		if err := c.removeUserIfNotExistsInLDAP(ldapDirectory, giteaUser); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) removeUserIfNotExistsInLDAP(ldapDirectory *ldap.Directory, giteaUser *gitea.User) error {
	c.log.Info().Msgf("Processing gitea user: %s", giteaUser.UserName)

	if len(c.Config.LDAP.ExcludeUsersRegex) > 0 {
		r := regexp.MustCompile(c.Config.LDAP.ExcludeUsersRegex)
		if r.MatchString(giteaUser.UserName) {
			c.log.Info().Msgf("User skipped (reason: regex-exclude-list): %s", giteaUser.UserName)

			return nil
		}
	}

	if stringslice.Contains(c.Config.LDAP.ExcludeUsers, giteaUser.UserName) {
		c.log.Info().Msgf("User skipped (reason: exclude-list): %s", giteaUser.UserName)
		return nil
	}

	_, ok := ldapDirectory.Users[giteaUser.UserName]
	if !ok {
		if !c.Config.SyncConfig.FullSync {
			c.log.Debug().Msgf("User does not exist in LDAP, full sync disabled, skipping: %s", giteaUser.UserName)

			return nil
		}

		c.log.Info().Msgf("User does not exist in LDAP, deleting from gitea: %s", giteaUser.UserName)

		if err := c.Gitea.DeleteUser(giteaUser.UserName); err != nil {
			return err
		}

		return nil
	}

	c.log.Debug().Msgf("User exist in ldap: %s", giteaUser.UserName)

	return nil
}

// syncGiteaGroupHierarchyWithLDAP iterates through all Gitea Organizations and all Teams inside the organizations.
// It is going to attach the Gitea Users to Gitea Teams (if the LDAP users are members of the LDAP Subgroups and if
// those LDAP Subgroups are members of LDAP Groups).
func (c *Client) syncGiteaGroupHierarchyWithLDAP(ldapDirectory *ldap.Directory) error {
	c.log.Tag("remove-groups-from-gitea")

	c.log.Info().Msg("Syncing users to teams in gitea")
	c.log.Info().Msgf("Number of organization groups in ldap: %d", len(ldapDirectory.Organizations))
	c.log.Debug().Msgf("Organization groups in ldap: %s", ldapDirectory.Organizations.String())

	// Check organizations and teams in Gitea, add users to them.
	giteaOrgs, err := c.Gitea.ListOrganizations()
	if err != nil {
		return err
	}

	c.log.Info().Msgf("Number of organizations in gitea: %d", len(giteaOrgs))
	c.log.Debug().Msgf("Organizations in gitea: %s", giteaOrgs)

	for _, giteaOrg := range giteaOrgs {
		if err := c.syncGiteaOrganizationWithLDAP(ldapDirectory, giteaOrg); err != nil {
			return err
		}
	}

	c.log.Info().Msg("Syncing users to teams in gitea finished")

	return nil
}

func (c *Client) syncGiteaOrganizationWithLDAP(ldapDirectory *ldap.Directory, giteaOrg *gitea.Organization) error {
	c.log.Info().Msgf("Processing organization: %s (id: %d)", giteaOrg.UserName, giteaOrg.ID)

	giteaTeams, err := c.Gitea.ListTeams(giteaOrg.UserName)
	if err != nil {
		return err
	}

	c.log.Debug().Msgf("Number of teams in %s organization: %d", giteaOrg.UserName, len(giteaTeams))

	org, ok := ldapDirectory.Organizations[giteaOrg.UserName]
	if !ok {
		if !c.Config.SyncConfig.FullSync {
			c.log.Debug().Msgf(
				"Organization does not exist in LDAP, full sync is disabled, skipping: %s", giteaOrg.UserName,
			)

			return nil
		}

		c.log.Info().Msgf("Organization does not exist in LDAP, deleting from gitea: %s", giteaOrg.UserName)

		if err = c.Gitea.DeleteOrganization(giteaOrg.UserName); err != nil {
			return err
		}

		return nil
	}

	for _, giteaTeam := range giteaTeams {
		if err := c.syncGiteaTeamWithLDAP(org, giteaTeam); err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) syncGiteaTeamWithLDAP(org *ldap.Organization, giteaTeam *giteapkg.Team) error {
	c.log.Info().Msgf("Processing team: %s", giteaTeam.Name)

	if giteaTeam.Name == "Owners" {
		c.log.Info().Msgf("Team skipped (reason: owner): %s", giteaTeam.Name)

		return nil
	}

	ldapTeam, ok := org.Teams[giteaTeam.Name]
	if !ok {
		if !c.Config.SyncConfig.FullSync {
			c.log.Debug().Msgf("Team does not exist in ldap, full sync is disabled, skipping: %s", giteaTeam.Name)

			return nil
		}

		c.log.Info().Msgf("Team does not exist in ldap, full sync is enabled, deleting from gitea: %s", giteaTeam.Name)

		if err := c.Gitea.DeleteTeam(giteaTeam.ID); err != nil {
			return err
		}

		return nil
	}

	giteaUsers, err := c.Gitea.ListTeamUsers(giteaTeam.ID)
	if err != nil {
		return err
	}

	c.log.Debug().Msgf("Gitea team %s (id: %d) has %d users", giteaTeam.Name, giteaTeam.ID, len(giteaUsers))

	if err := c.syncGiteaTeamMembers(ldapTeam, giteaTeam, giteaUsers); err != nil {
		return err
	}

	return nil
}

func (c *Client) syncGiteaTeamMembers(
	ldapTeam *ldap.Team, giteaTeam *giteapkg.Team, giteaAccounts map[string]gitea.Account,
) error {
	c.log.Info().Msgf("Checking team in LDAP: %s.", ldapTeam.Name)
	if err := c.addGiteaUsersToTeams(ldapTeam, giteaTeam, giteaAccounts); err != nil {
		return err
	}

	if err := c.removeGiteaUsersFromTeams(ldapTeam, giteaTeam, giteaAccounts); err != nil {
		return err
	}

	return nil
}

func (c *Client) removeGiteaUsersFromTeams(
	ldapTeam *ldap.Team, giteaTeam *giteapkg.Team, giteaAccounts map[string]gitea.Account,
) error {
	var removeUserCandidates gitea.Accounts

	for _, u := range giteaAccounts {
		c.log.Debug().Msgf("Processing gitea user: %s", u.String())

		exists, err := existInSlice(u.Login, ldapTeam.Users)
		if err != nil {
			return err
		}

		if !exists {
			removeUserCandidates = append(removeUserCandidates, u)
		}
	}

	if len(removeUserCandidates) == 0 {
		c.log.Debug().Msgf("No users to remove from team: %s", ldapTeam.Name)

		return nil
	}

	c.log.Info().Msgf("Users will be removed from gitea team: %s (team: %s)", removeUserCandidates, ldapTeam.Name)

	if err := c.Gitea.DelUsersFromTeam(removeUserCandidates, giteaTeam.ID); err != nil {
		return err
	}

	return nil
}

func (c *Client) addGiteaUsersToTeams(
	ldapTeam *ldap.Team, giteaTeam *giteapkg.Team, giteaAccounts map[string]gitea.Account,
) error {
	var addUserCandidates gitea.Accounts

	for _, u := range ldapTeam.Users {
		c.log.Debug().Msgf("Processing gitea team user: %s", u.Name)

		if giteaAccounts[u.Name].Login != u.Entry.GetAttributeValue(c.Config.LDAP.UserUsernameAttribute) {
			acc := gitea.Account{
				Login:    u.Entry.GetAttributeValue(c.Config.LDAP.UserUsernameAttribute),
				FullName: u.Entry.GetAttributeValue(c.Config.LDAP.UserFullNameAttribute),
			}

			addUserCandidates = append(addUserCandidates, acc)
		}
	}

	if len(addUserCandidates) == 0 {
		c.log.Debug().Msgf("No users to add to team: %s", ldapTeam.Name)

		return nil
	}

	c.log.Info().Msgf("Users will be added to team: %s (team: %s)", addUserCandidates, ldapTeam.Name)

	if err := c.Gitea.AddUsersToTeam(addUserCandidates, giteaTeam.ID); err != nil {
		return err
	}

	return nil
}

func existInSlice(s string, slice interface{}) (bool, error) { //nolint:gocognit
	switch t := slice.(type) {
	case []gitea.Organization:
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
	case []gitea.Team:
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
	case map[string]*ldap.User:
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

	return false, errors.New("unknown type in slice")
}

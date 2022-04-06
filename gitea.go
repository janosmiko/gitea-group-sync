package main

import (
	urlpkg "net/url"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func NewGiteaClient() (*GiteaClient, error) {
	var gtc *gitea.Client
	var err error

	u, err := urlpkg.Parse(viper.GetString("GITEA_BASE_URL"))
	if err != nil {
		return nil, err
	}

	user := urlpkg.UserPassword("", viper.GetString("GITEA_TOKEN"))
	u.User = user

	gtc, err = gitea.NewClient(u.String())
	if err != nil {
		return nil, err
	}

	return &GiteaClient{
		Client:  gtc,
		Token:   []string{viper.GetString("GITEA_TOKEN")},
		BaseURL: u.String(),
	}, nil
}

func (c *GiteaClient) AddUsersToTeam(users []GiteaAccount, team int64) error {
	zap.L().Debug("")

	for i, thisUser := range users {
		sr := gitea.SearchUsersOption{
			KeyWord: thisUser.FullName,
		}
		foundUsers, _, err := c.SearchUsers(sr)
		if err != nil {
			return err
		}

		for _, foundUser := range foundUsers {
			if strings.EqualFold(thisUser.Login, foundUser.UserName) {
				_, err = c.Client.AddTeamMember(team, users[i].Login)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (c *GiteaClient) DelUsersFromTeam(users []GiteaAccount, team int64) error {
	for _, thisUser := range users {
		_, err := c.RemoveTeamMember(team, thisUser.FullName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *GiteaClient) CreateOrganization(o GiteaOrganization) error {
	zap.L().Debug("")

	orgs, err := c.ListOrganizations()
	if err != nil {
		return err
	}

	exist := false
	for _, org := range orgs {
		if org.UserName == o.UserName {
			exist = true
		}
	}

	if !exist {
		zap.S().Infof(`Creating organization: "%v".`, o.UserName)
		opts := gitea.CreateOrgOption{
			Name:                      o.UserName,
			FullName:                  o.FullName,
			Description:               o.Description,
			Website:                   o.Website,
			Location:                  o.Location,
			Visibility:                gitea.VisibleType(o.Visibility),
			RepoAdminChangeTeamAccess: o.RepoAdminChangeTeamAccess,
		}
		_, _, err := c.CreateOrg(opts)
		return err
	}
	zap.S().Infof(`Organization "%v" already exist.`, o.UserName)

	return nil
}

func (c *GiteaClient) DeleteOrganization(orgname string) error {
	zap.S().Infof("Deleting organization: %v", orgname)

	opts := gitea.ListOrgReposOptions{}
	repos, _, err := c.Client.ListOrgRepos(orgname, opts)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		_, err = c.Client.DeleteRepo(orgname, repo.Name)
		if err != nil {
			return err
		}
	}

	_, err = c.Client.DeleteOrg(orgname)
	return err
}

func (c *GiteaClient) CreateTeam(orgname string, t GiteaTeam, o GiteaCreateTeamOpts) error {
	ltopts := gitea.ListTeamsOptions{}
	teams, _, err := c.Client.ListOrgTeams(orgname, ltopts)
	if err != nil {
		return err
	}

	exist := false
	for _, team := range teams {
		if team.Name == t.Name {
			exist = true
		}
	}

	if !exist {
		zap.S().Infof(`Creating team: "%v", in organization: "%v"`, t.Name, orgname)
		opts := gitea.CreateTeamOption{
			Name:                    t.Name,
			Description:             t.Description,
			Permission:              o.Permission,
			CanCreateOrgRepo:        o.CanCreateOrgRepo,
			IncludesAllRepositories: o.IncludesAllRepositories,
			Units:                   o.Units,
		}
		_, _, err := c.Client.CreateTeam(orgname, opts)
		return err
	}

	zap.S().Infof(`Team: "%v", already exist in organization: "%v"`, t.Name, orgname)

	return nil
}

func (c *GiteaClient) DeleteTeam(teamID int64) error {
	zap.S().Infof("Deleting team with ID: %v", teamID)

	_, err := c.Client.DeleteTeam(teamID)
	return err
}

func (c *GiteaClient) CreateUser(u GiteaUser) error {
	opts := gitea.AdminListUsersOptions{}
	users, _, err := c.Client.AdminListUsers(opts)
	if err != nil {
		return err
	}

	exist := false
	for _, user := range users {
		if user.UserName == u.UserName {
			exist = true
		}
	}

	if !exist {
		zap.S().Infof(`Creating user: "%v"`, u.UserName)
		opts := gitea.CreateUserOption{
			LoginName:          u.UserName,
			Username:           u.UserName,
			FullName:           u.FullName,
			Email:              u.Email,
			MustChangePassword: gitea.OptionalBool(false),
			Visibility:         OptionalVisibility(u.Visibility),
		}

		_, _, err = c.Client.AdminCreateUser(opts)
		if err != nil {
			return err
		}
	}

	eOpts := gitea.EditUserOption{
		LoginName:               u.UserName,
		Email:                   gitea.OptionalString(u.Email),
		FullName:                gitea.OptionalString(u.FullName),
		MaxRepoCreation:         OptionalInt(viper.GetInt("sync_config.defaults.max_repo_creation")),
		AllowCreateOrganization: gitea.OptionalBool(viper.GetBool("sync_config.defaults.allow_create_organization")),
		Visibility:              OptionalVisibility(gitea.VisibleType(viper.GetString("sync_config.defaults.visibility"))),
		Admin:                   OptionalBool(u.IsAdmin),
		Restricted:              OptionalBool(u.Restricted),
	}

	_, err = c.Client.AdminEditUser(u.UserName, eOpts)
	if err != nil {
		return err
	}

	zap.S().Infof(`User: "%v", already exist.`, u.UserName)

	return nil
}

func (c *GiteaClient) DeleteUser(username string) error {
	zap.S().Infof("Deleting user: %v", username)

	_, err := c.Client.AdminDeleteUser(username)

	return err
}

func (c *GiteaClient) ListTeamUsers(teamID int64) (map[string]GiteaAccount, error) {
	var accounts = make(map[string]GiteaAccount)

	opts := gitea.ListTeamMembersOptions{}
	foundUsers, _, err := c.Client.ListTeamMembers(teamID, opts)
	if err != nil {
		return nil, err
	}

	for _, user := range foundUsers {
		accounts[user.UserName] = GiteaAccount{
			Email:    user.Email,
			ID:       int(user.ID),
			FullName: user.FullName,
			Login:    user.UserName,
		}
	}

	return accounts, nil
}

func (c *GiteaClient) ListOrganizations() ([]*gitea.Organization, error) {
	opts := gitea.AdminListOrgsOptions{}
	orgs, _, err := c.Client.AdminListOrgs(opts)
	if err != nil {
		return nil, err
	}

	return orgs, nil
}

func (c *GiteaClient) ListTeams(orgname string) ([]*gitea.Team, error) {
	opts := gitea.ListTeamsOptions{}
	teams, _, err := c.ListOrgTeams(orgname, opts)
	if err != nil {
		return nil, err
	}

	return teams, nil
}

func (c *GiteaClient) AdminListUsers() ([]*gitea.User, error) {
	opts := gitea.AdminListUsersOptions{}
	users, _, err := c.Client.AdminListUsers(opts)
	if err != nil {
		return nil, err
	}

	return users, nil
}

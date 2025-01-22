package gitea

import (
	urlpkg "net/url"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/janosmiko/gitea-ldap-sync/internal/config"
	"github.com/janosmiko/gitea-ldap-sync/internal/ptr"
)

type (
	Organization = gitea.Organization
	Team         = gitea.Team
	User         = gitea.User
)

type Organizations []*Organization

func (c Organizations) String() string {
	orgs := make([]string, 0, len(c))

	for _, org := range c {
		orgs = append(orgs, org.UserName)
	}

	return strings.Join(orgs, ",")
}

type Client struct {
	client *gitea.Client
	config *config.Config
	log    zerolog.Logger
}

type Account struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Login    string `json:"login"`
	Email    string `json:"email"`
}

func (c *Account) String() string {
	return c.Login
}

type Accounts []Account

func (c Accounts) String() string {
	users := make([]string, 0, len(c))

	for _, user := range c {
		users = append(users, user.String())
	}

	return strings.Join(users, ",")
}

type CreateTeamOpts struct {
	Permission              gitea.AccessMode
	CanCreateOrgRepo        bool
	IncludesAllRepositories bool
	Units                   []gitea.RepoUnitType
}

func New(conf *config.Config) (*Client, error) {
	u, err := urlpkg.Parse(conf.Gitea.BaseURL)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing gitea base url: %s", conf.Gitea.BaseURL)
	}

	user := urlpkg.UserPassword(conf.Gitea.User, conf.Gitea.Token)
	u.User = user

	client, err := gitea.NewClient(u.String())
	if err != nil {
		return nil, errors.Wrapf(err, "creating gitea client for user: %s", u.User.Username())
	}

	return &Client{
		client: client,
		config: conf,
		log:    log.Logger.With().Str("tag", "[gitea]").Logger(),
	}, nil
}

func (c *Client) AddUsersToTeam(users []Account, team int64) error {
	c.log.Debug().Msgf("Adding users to team: %d", team)

	for i, user := range users {
		c.log.Debug().Msgf("Processing user: %s", user.FullName)

		foundUsers, _, err := c.client.SearchUsers(gitea.SearchUsersOption{
			KeyWord: user.FullName,
		})
		if err != nil {
			return errors.Wrapf(err, "searching users: %s", user.FullName)
		}

		for _, foundUser := range foundUsers {
			if !strings.EqualFold(user.Login, foundUser.UserName) {
				continue
			}

			if _, err = c.client.AddTeamMember(team, users[i].Login); err != nil {
				return errors.Wrapf(err, "adding user to team: %s (team-id: %d)", user.Login, team)
			}

			c.log.Info().Msgf("User: %s added to team: %d", user.FullName, team)
		}
	}

	c.log.Debug().Msgf("Users added to team: %d", team)

	return nil
}

func (c *Client) DelUsersFromTeam(users []Account, team int64) error {
	c.log.Debug().Msgf("Removing users from team with id: %d", team)

	for _, user := range users {
		c.log.Debug().Msgf("Processing user: %s", user.FullName)

		if _, err := c.client.RemoveTeamMember(team, user.FullName); err != nil {
			return errors.Wrapf(err, "removing user from team: %s (team-id: %d)", user.Login, team)
		}

		c.log.Info().Msgf("User: %s removed from team: %d", user.FullName, team)
	}

	c.log.Debug().Msgf("Users removed from team with id: %d", team)

	return nil
}

func (c *Client) CreateOrganization(o Organization) error {
	c.log.Debug().Msgf("Creating organization: %s", o.UserName)

	exist, err := c.OrganizationExists(o)
	if err != nil {
		return err
	}

	if exist {
		c.log.Debug().Msgf("Organization already exist: %s", o.UserName)

		return nil
	}

	if _, _, err := c.client.CreateOrg(gitea.CreateOrgOption{
		Name:                      o.UserName,
		FullName:                  o.FullName,
		Description:               o.Description,
		Website:                   o.Website,
		Location:                  o.Location,
		Visibility:                gitea.VisibleType(o.Visibility),
		RepoAdminChangeTeamAccess: c.config.SyncConfig.Defaults.Organization.RepoAdminChangeTeamAccess,
	}); err != nil {
		return errors.Wrapf(err, "failed to create organization: %s", o.UserName)
	}

	c.log.Info().Msgf("Organization created: %s", o.UserName)

	return nil
}

func (c *Client) OrganizationExists(o Organization) (bool, error) {
	c.log.Debug().Msgf("Checking if organization: %s exists", o.UserName)

	orgs, err := c.ListOrganizations()
	if err != nil {
		return false, err
	}

	for _, org := range orgs {
		if org.UserName == o.UserName {
			return true, nil
		}
	}

	return false, nil
}

func (c *Client) DeleteOrganization(orgname string) error {
	c.log.Debug().Msgf("Deleting organization: %s", orgname)

	repos, _, err := c.client.ListOrgRepos(orgname, gitea.ListOrgReposOptions{})
	if err != nil {
		return errors.Wrapf(err, "listing all repositories")
	}

	for _, repo := range repos {
		if _, err = c.client.DeleteRepo(orgname, repo.Name); err != nil {
			return errors.Wrapf(err, "deleting repository: %s", repo.Name)
		}

		c.log.Info().Msgf("Repository: %s deleted", repo.Name)
	}

	if _, err = c.client.DeleteOrg(orgname); err != nil {
		return errors.Wrapf(err, "deleting organization: %s", orgname)
	}

	c.log.Info().Msgf("Organization: %s deleted", orgname)

	return nil
}

func (c *Client) CreateTeam(orgname string, team Team, opts CreateTeamOpts) error {
	c.log.Debug().Msgf("Creating team in organization: %s (team: %s)", team.Name, orgname)

	exist, err := c.TeamExists(orgname, team)
	if err != nil {
		return err
	}

	if exist {
		c.log.Debug().Msgf("Team already exist in organization: %s (organization: %s)", team.Name, orgname)

		return nil
	}

	c.log.Info().Msgf("Creating team in organization: %s (organization: %s)", team.Name, orgname)

	if _, _, err := c.client.CreateTeam(orgname, gitea.CreateTeamOption{
		Name:                    team.Name,
		Description:             team.Description,
		Permission:              opts.Permission,
		CanCreateOrgRepo:        opts.CanCreateOrgRepo,
		IncludesAllRepositories: opts.IncludesAllRepositories,
		Units:                   opts.Units,
	}); err != nil {
		return errors.Wrapf(err, "creating team: %s", team.Name)
	}

	c.log.Info().Msgf("Team created in organization: %s (organization: %s)", team.Name, orgname)

	return nil
}

func (c *Client) TeamExists(orgname string, t Team) (bool, error) {
	c.log.Debug().Msgf("Checking if team exists in organization: %s (organization: %s)", t.Name, orgname)

	teams, _, err := c.client.ListOrgTeams(orgname, gitea.ListTeamsOptions{})
	if err != nil {
		return false, err
	}

	for _, team := range teams {
		if team.Name == t.Name {
			return true, nil
		}
	}

	return false, nil
}

func (c *Client) DeleteTeam(teamID int64) error {
	c.log.Debug().Msgf("Deleting team with ID: %d", teamID)

	if _, err := c.client.DeleteTeam(teamID); err != nil {
		return errors.Wrapf(err, "deleting team-id: %d", teamID)
	}

	c.log.Info().Msgf("Team with ID: %d deleted", teamID)

	return nil
}

func (c *Client) CreateOrUpdateUser(u User) error {
	c.log.Debug().Msgf("Creating user: %s", u.UserName)

	exist, err := c.userExists(u)
	if err != nil {
		return err
	}

	if !exist {
		if err := c.createUser(u); err != nil {
			return err
		}
	}

	if err := c.updateUser(u); err != nil {
		return err
	}

	return nil
}

func (c *Client) updateUser(user User) error {
	c.log.Debug().Msgf("Updating user: %s", user.UserName)

	if _, err := c.client.AdminEditUser(user.UserName, gitea.EditUserOption{
		LoginName:               user.UserName,
		Email:                   ptr.To(user.Email),
		FullName:                ptr.To(user.FullName),
		MaxRepoCreation:         ptr.To(c.config.SyncConfig.Defaults.User.MaxRepoCreation),
		AllowCreateOrganization: ptr.To(c.config.SyncConfig.Defaults.User.AllowCreateOrganization),
		Visibility: ptr.To(
			gitea.VisibleType(
				c.config.SyncConfig.Defaults.User.Visibility,
			),
		),
		Admin:      ptr.To(user.IsAdmin),
		Restricted: ptr.To(user.Restricted),
	}); err != nil {
		return errors.Wrapf(err, "updating user: %s", user.UserName)
	}

	c.log.Info().Msgf("User updated %s", user.UserName)

	return nil
}

func (c *Client) userExists(u User) (bool, error) {
	c.log.Debug().Msgf("Checking if user exists: %s", u.UserName)

	users, err := c.ListUsers()
	if err != nil {
		return false, err
	}

	exist := false
	for _, user := range users {
		if user.UserName == u.UserName {
			exist = true
		}
	}

	return exist, nil
}

func (c *Client) createUser(user User) error {
	c.log.Debug().Msgf("Creating user: %s", user.UserName)

	if _, _, err := c.client.AdminCreateUser(gitea.CreateUserOption{
		LoginName:          user.UserName,
		Username:           user.UserName,
		FullName:           user.FullName,
		Email:              user.Email,
		MustChangePassword: ptr.To(false),
		Visibility:         ptr.To(user.Visibility),
		SourceID:           c.config.Gitea.AuthSourceID,
	}); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}

		return errors.Wrapf(err, "creating user: %s", user.UserName)
	}

	c.log.Info().Msgf("User created: %s", user.UserName)

	return nil
}

func (c *Client) DeleteUser(username string) error {
	c.log.Debug().Msgf("Deleting user: %s", username)

	if _, err := c.client.AdminDeleteUser(username); err != nil {
		return errors.Wrapf(err, "deleting user: %s", username)
	}

	c.log.Info().Msgf("User: %s deleted", username)

	return nil
}

func (c *Client) ListTeamUsers(teamID int64) (map[string]Account, error) {
	var accounts = make(map[string]Account)

	users, _, err := c.client.ListTeamMembers(teamID, gitea.ListTeamMembersOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "listing all team members")
	}

	for _, user := range users {
		accounts[user.UserName] = Account{
			Email:    user.Email,
			ID:       int(user.ID),
			FullName: user.FullName,
			Login:    user.UserName,
		}
	}

	return accounts, nil
}

func (c *Client) ListOrganizations() (Organizations, error) {
	orgs, _, err := c.client.AdminListOrgs(gitea.AdminListOrgsOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "listing all organizations")
	}

	return orgs, nil
}

func (c *Client) ListTeams(orgname string) ([]*gitea.Team, error) {
	teams, _, err := c.client.ListOrgTeams(orgname, gitea.ListTeamsOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "listing all teams")
	}

	return teams, nil
}

func (c *Client) ListUsers() ([]*User, error) {
	users, _, err := c.client.AdminListUsers(gitea.AdminListUsersOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "listing all users")
	}

	return users, nil
}

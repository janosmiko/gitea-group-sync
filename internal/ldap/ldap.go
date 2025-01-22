package ldap

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/ldap.v3"

	"github.com/janosmiko/gitea-ldap-sync/internal/config"
	"github.com/janosmiko/gitea-ldap-sync/internal/logger"
	"github.com/janosmiko/gitea-ldap-sync/internal/ptr"
	"github.com/janosmiko/gitea-ldap-sync/internal/stringslice"
)

type Client struct {
	ldap.Client
	config *config.Config
	log    logger.Logger
}

func (c *Client) NewUser(entry *ldap.Entry, restricted, admin bool) *User {
	return &User{
		Entry:      entry,
		Name:       entry.GetAttributeValue(c.config.LDAP.UserUsernameAttribute),
		Restricted: ptr.To(restricted),
		Admin:      ptr.To(admin),
		config:     c.config.LDAP,
	}
}

type Directory struct {
	Organizations Organizations
	Users         Users
}

type Organization struct {
	Name string
	*ldap.Entry
	Teams map[string]*Team
}

type Organizations map[string]*Organization

func (c *Organizations) String() string {
	s := make([]string, 0, len(*c))
	for k := range *c {
		s = append(s, k)
	}

	return strings.Join(s, ",")
}

type Team struct {
	Name string
	*ldap.Entry
	Users map[string]*User
}

type User struct {
	*ldap.Entry
	Name       string
	Restricted *bool
	Admin      *bool
	config     *config.LDAPConfig
}

type Users map[string]*User

func (u *User) Fullname(c *config.LDAPConfig) string {
	firstname := u.GetAttributeValue(c.UserFirstNameAttribute)
	surname := u.GetAttributeValue(c.UserSurnameAttribute)
	fullname := ""

	if len(firstname) > 0 && len(surname) > 0 {
		fullname = fmt.Sprintf("%s %s", firstname, surname)
	} else {
		fullname = u.GetAttributeValue(c.UserFullNameAttribute)
	}

	return fullname
}

func (c *Client) Close() {
	c.Client.Close()
}

func New(conf *config.Config) (*Client, error) {
	l, err := NewLDAPConn(conf)
	if err != nil {
		return nil, err
	}

	ldapClient := &Client{
		Client: l,
		config: conf,
		log:    logger.New().Tag("ldap"),
	}

	if err := ldapClient.configureBind(); err != nil {
		return nil, err
	}

	return ldapClient, nil
}

func NewLDAPConn(c *config.Config) (*ldap.Conn, error) {
	// Prepare LDAPConfig Connection
	if c.LDAP.UseTLS {
		//nolint:gosec // allowInsecureTLS should be used with caution.
		l, err := ldap.DialTLS(
			"tcp",
			fmt.Sprintf("%s:%d", c.LDAP.URL, c.LDAP.Port),
			&tls.Config{InsecureSkipVerify: c.LDAP.AllowInsecureTLS},
		)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to dial TLS: %s:%d", c.LDAP.URL, c.LDAP.Port)
		}

		return l, nil
	}

	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", c.LDAP.URL, c.LDAP.Port))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to dial non-TLS: %s:%d", c.LDAP.URL, c.LDAP.Port)
	}

	return l, nil
}

func (c *Client) configureBind() error {
	if len(c.config.LDAP.BindDN) == 0 {
		if err := c.UnauthenticatedBind(""); err != nil {
			return errors.Wrap(err, "failed to set unauthenticated bind")
		}
	}

	if err := c.Bind(c.config.LDAP.BindDN, c.config.LDAP.BindPassword); err != nil {
		return errors.Wrapf(err, "failed to bind with binddn: %s", c.config.LDAP.BindDN)
	}

	return nil
}

func (c *Client) GetDirectory() (*Directory, error) {
	c.log.Debug().Msg("Getting ldap directory")

	ldapGroups, err := c.groups()
	if err != nil {
		return nil, err
	}

	ldapTeams, err := c.teams()
	if err != nil {
		return nil, err
	}

	ldapUsers, err := c.getUsers()
	if err != nil {
		return nil, err
	}

	ldapAdminUsers, err := c.adminUsers()
	if err != nil {
		return nil, err
	}

	ldapRestrictedUsers, err := c.restrictedUsers()
	if err != nil {
		return nil, err
	}

	c.log.Info().Msg("LDAP directory fetched")

	dir := c.buildDirectory(ldapGroups, ldapTeams, ldapUsers, ldapRestrictedUsers, ldapAdminUsers)

	return dir, nil
}

func (c *Client) buildDirectory(
	ldapGroups []*ldap.Entry, ldapTeams []*ldap.Entry,
	ldapUsers []*ldap.Entry, ldapAdminUsers []*ldap.Entry, ldapRestrictedUsers []*ldap.Entry,
) *Directory {
	c.log.Debug().Msg("Building ldap directory")

	o := make(map[string]*Organization)
	u := make(map[string]*User)
	dir := &Directory{
		Organizations: o,
		Users:         u,
	}

	c.buildGroups(dir, ldapGroups, ldapTeams, ldapUsers)
	c.buildUsers(dir, ldapUsers, ldapRestrictedUsers, ldapAdminUsers)

	c.log.Info().Msg("LDAP directory built")

	return dir
}

func (c *Client) buildGroups(
	dir *Directory, ldapGroups []*ldap.Entry, ldapTeams []*ldap.Entry, ldapUsers []*ldap.Entry,
) {
	c.log.Debug().Msg("Building ldap groups")

	ldapOrgs := difference(ldapGroups, ldapTeams)

	for _, o := range ldapOrgs {
		teams := make(map[string]*Team)

		dir.Organizations[o.GetAttributeValue(c.config.LDAP.GroupNameAttribute)] = &Organization{
			Name:  o.GetAttributeValue(c.config.LDAP.GroupNameAttribute),
			Entry: o,
			Teams: teams,
		}

		for _, t := range ldapTeams {
			n := t.GetAttributeValue("memberOf")
			_ = n
			if strings.EqualFold(t.GetAttributeValue("memberOf"), o.DN) {
				users := make(map[string]*User)
				for _, u := range t.GetAttributeValues("member") {
					for _, v := range ldapUsers {
						if strings.EqualFold(u, v.DN) {
							users[v.GetAttributeValue(c.config.LDAP.UserUsernameAttribute)] = c.NewUser(v, false, false)
						}
					}
				}

				name := t.GetAttributeValue(c.config.LDAP.SubgroupNameAttribute)

				if c.config.LDAP.TrimParentName {
					separator := c.config.LDAP.SubgroupSeparator
					name = name[strings.Index(name, separator)+len(separator):]
				}

				dir.Organizations[o.GetAttributeValue(c.config.LDAP.GroupNameAttribute)].Teams[name] = &Team{
					Name:  name,
					Entry: t,
					Users: users,
				}
			}
		}
	}
}

func (c *Client) buildUsers(
	dir *Directory, ldapUsers []*ldap.Entry, ldapRestrictedUsers []*ldap.Entry, ldapAdminUsers []*ldap.Entry,
) {
	c.log.Debug().Msg("Building ldap users")
	for _, u := range ldapUsers {
		restricted := false
		admin := false
		if c.config.LDAP.RestrictedFilter != "" {
			rstUsers := make([]string, len(ldapRestrictedUsers))
			for i, v := range ldapRestrictedUsers {
				rstUsers[i] = v.GetAttributeValue(c.config.LDAP.UserUsernameAttribute)
			}
			if stringslice.Contains(rstUsers, u.GetAttributeValue(c.config.LDAP.UserUsernameAttribute)) {
				restricted = true
			}
		}

		if c.config.LDAP.AdminFilter != "" {
			admUsers := make([]string, len(ldapAdminUsers))
			for i, v := range ldapAdminUsers {
				admUsers[i] = v.GetAttributeValue(c.config.LDAP.UserUsernameAttribute)
			}
			if stringslice.Contains(admUsers, u.GetAttributeValue(c.config.LDAP.UserUsernameAttribute)) {
				admin = true
			}
		}

		dir.Users[u.GetAttributeValue(c.config.LDAP.UserUsernameAttribute)] = c.NewUser(u, restricted, admin)
	}
}

func (c *Client) groups() ([]*ldap.Entry, error) {
	c.log.Debug().Msg("Searching for groups in ldap")

	searchResult, err := c.Search(c.config.LDAP.GroupSearchBase, c.config.LDAP.GroupFilter)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to search for groups in ldap. searchbase: %s, filter: %s",
			c.config.LDAP.GroupSearchBase, c.config.LDAP.GroupFilter,
		)
	}

	c.log.Info().Msgf("Found %d groups in ldap", len(searchResult.Entries))

	return searchResult.Entries, nil
}

func (c *Client) teams() ([]*ldap.Entry, error) {
	c.log.Debug().Msg("Searching for teams in ldap")

	searchResult, err := c.Search(c.config.LDAP.SubgroupSearchBase, c.config.LDAP.SubgroupFilter)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to search for teams in ldap. searchbase: %s, filter: %s",
			c.config.LDAP.SubgroupFilter, c.config.LDAP.SubgroupFilter,
		)
	}

	c.log.Info().Msgf("Found %d teams in ldap", len(searchResult.Entries))

	return searchResult.Entries, nil
}

func (c *Client) restrictedUsers() ([]*ldap.Entry, error) {
	if len(c.config.LDAP.RestrictedFilter) == 0 {
		return nil, nil
	}

	c.log.Debug().Msg("Searching for restricted users in ldap")

	searchResult, err := c.Search(c.config.LDAP.UserSearchBase, c.config.LDAP.RestrictedFilter)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to search for restricted users in ldap. searchbase: %s, filter: %s",
			c.config.LDAP.UserSearchBase, c.config.LDAP.RestrictedFilter,
		)
	}

	c.log.Info().Msgf("Found %d restricted users in ldap", len(searchResult.Entries))

	return searchResult.Entries, nil
}

func (c *Client) adminUsers() ([]*ldap.Entry, error) {
	if len(c.config.LDAP.AdminFilter) == 0 {
		return nil, nil
	}

	c.log.Debug().Msg("Searching for admin users in ldap")

	searchResult, err := c.Search(c.config.LDAP.UserSearchBase, c.config.LDAP.AdminFilter)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to search for admin users in ldap. searchbase: %s, filter: %s",
			c.config.LDAP.UserSearchBase, c.config.LDAP.AdminFilter,
		)
	}

	c.log.Info().Msgf("Found %d admin users in ldap", len(searchResult.Entries))

	return searchResult.Entries, nil
}

func (c *Client) getUsers() ([]*ldap.Entry, error) {
	c.log.Debug().Msg("Searching for users in ldap")

	searchResult, err := c.Search(c.config.LDAP.UserSearchBase, c.config.LDAP.UserFilter)
	if err != nil {
		return nil, errors.Wrapf(
			err, "failed to search for users in ldap. searchbase: %s, filter: %s", c.config.LDAP.UserSearchBase,
			c.config.LDAP.UserFilter,
		)
	}

	c.log.Info().Msgf("Found %d users in ldap", len(searchResult.Entries))

	return searchResult.Entries, nil
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

func (c *Client) Search(baseDN, filter string) (*ldap.SearchResult, error) {
	return c.Client.Search(
		ldap.NewSearchRequest(
			baseDN,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			filter,
			[]string{},
			nil,
		),
	)
}

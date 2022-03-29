package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gopkg.in/ldap.v3"
)

func (c *LDAP) closeLDAP() {
	c.Client.Close()
}

func (c *LDAP) initLDAP() {
	var err error

	// Prepare LDAP Connection
	var l *ldap.Conn
	if c.UseTLS {
		l, err = ldap.DialTLS(
			"tcp",
			fmt.Sprintf("%s:%d", c.URL, c.Port),
			&tls.Config{InsecureSkipVerify: c.AllowInsecureTLS}, // nolint:gosec // allowInsecureTLS should be used with caution.
		)
	} else {
		l, err = ldap.Dial("tcp", fmt.Sprintf("%s:%d", c.URL, c.Port))
	}

	if err != nil {
		fmt.Println(err)
		fmt.Println("Please set the correct values for all specifics.")
		os.Exit(1)
	}

	if len(c.BindDN) == 0 {
		err = l.UnauthenticatedBind("")
	} else {
		err = l.Bind(c.BindDN, c.BindPassword)
	}

	if err != nil {
		zap.L().Fatal(err.Error())
	}

	c.Client = l
}

func (c *LDAP) getLDAPDirectory() *Directory { // nolint: gocognit
	var err error
	var searchRequest *ldap.SearchRequest
	var searchResult *ldap.SearchResult

	// Search for users in LDAP
	searchRequest = ldap.NewSearchRequest(
		c.UserSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		c.UserFilter,
		[]string{},
		nil,
	)

	// make request to ldap server
	searchResult, err = c.Client.Search(searchRequest)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	// All Users
	usersInLDAP := searchResult.Entries

	// Admin users
	var adminUsersInLDAP []*ldap.Entry
	if len(c.AdminFilter) > 0 {
		// Search for users in LDAP
		searchRequest = ldap.NewSearchRequest(
			c.UserSearchBase,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			c.AdminFilter,
			[]string{},
			nil,
		)

		// make request to ldap server
		searchResult, err = c.Client.Search(searchRequest)
		if err != nil {
			zap.L().Fatal(err.Error())
		}
		// All Users
		adminUsersInLDAP = searchResult.Entries
	}

	_ = adminUsersInLDAP

	// Restricted users
	var restrictedUsersInLDAP []*ldap.Entry
	if len(c.RestrictedFilter) > 0 {
		// Search for users in LDAP
		searchRequest = ldap.NewSearchRequest(
			c.UserSearchBase,
			ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
			c.RestrictedFilter,
			[]string{},
			nil,
		)

		// make request to ldap server
		searchResult, err = c.Client.Search(searchRequest)
		if err != nil {
			zap.L().Fatal(err.Error())
		}
		// All Users
		restrictedUsersInLDAP = searchResult.Entries
	}

	// Search for teams (subgroups) in LDAP
	searchRequest = ldap.NewSearchRequest(
		c.SubGroupSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		c.SubGroupFilter,
		[]string{},
		nil,
	)

	// make request to ldap server
	searchResult, err = c.Client.Search(searchRequest)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	teamsInLDAP := searchResult.Entries

	// Search for all groups (including subgroups) in LDAP. In the next step we are going to remove subgroups from
	// the slice.
	searchRequest = ldap.NewSearchRequest(
		c.GroupSearchBase,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		c.GroupFilter,
		[]string{},
		nil,
	)

	searchResult, err = c.Client.Search(searchRequest)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	allGroupsInLDAP := searchResult.Entries

	organizationsInLDAP := difference(allGroupsInLDAP, teamsInLDAP)

	orgs := make(map[string]*LDAPOrganization)
	users := make(map[string]*LDAPUser)
	dir := Directory{
		Organizations: orgs,
		Users:         users,
	}

	for _, o := range organizationsInLDAP {
		teams := make(map[string]*LDAPTeam)

		dir.Organizations[o.GetAttributeValue("cn")] = &LDAPOrganization{
			Name:  o.GetAttributeValue("cn"),
			Entry: o,
			Teams: teams,
		}

		for _, t := range teamsInLDAP {
			if strings.EqualFold(t.GetAttributeValue("memberOf"), o.DN) {
				users := make(map[string]*LDAPUser)
				for _, u := range t.GetAttributeValues("member") {
					for _, v := range usersInLDAP {
						if strings.EqualFold(u, v.DN) {
							user := LDAPUser{
								Name:  v.GetAttributeValue("cn"),
								Entry: v,
							}
							users[v.GetAttributeValue("cn")] = &user
						}
					}
				}

				name := t.GetAttributeValue("cn")

				if viper.GetBool("ldap.trim_parent_name") {
					sep := viper.GetString("ldap.subgroup_separator")
					name = name[strings.Index(name, sep)+len(sep):]
				}

				dir.Organizations[o.GetAttributeValue("cn")].Teams[name] = &LDAPTeam{
					Name:  name,
					Entry: t,
					Users: users,
				}
			}
		}
	}

	for _, u := range usersInLDAP {
		restricted := false
		admin := false
		if len(c.RestrictedFilter) > 0 {
			rstUsers := make([]string, len(restrictedUsersInLDAP))
			for i, v := range restrictedUsersInLDAP {
				rstUsers[i] = v.GetAttributeValue("cn")
			}
			if contains(rstUsers, u.GetAttributeValue("cn")) {
				restricted = true
			}
		}

		if len(c.AdminFilter) > 0 {
			admUsers := make([]string, len(adminUsersInLDAP))
			for i, v := range adminUsersInLDAP {
				admUsers[i] = v.GetAttributeValue("cn")
			}
			if contains(admUsers, u.GetAttributeValue("cn")) {
				admin = true
			}
		}

		dir.Users[u.GetAttributeValue("cn")] = &LDAPUser{
			Name:       u.GetAttributeValue("cn"),
			Entry:      u,
			Restricted: OptionalBool(restricted),
			Admin:      OptionalBool(admin),
		}
	}

	return &dir
}

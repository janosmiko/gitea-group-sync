package main

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

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

func (c *LDAP) getLDAPDirectory() *Directory {
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
	usersInLDAP := searchResult.Entries

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
	dir := Directory{
		Organizations: orgs,
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

				dir.Organizations[o.GetAttributeValue("cn")].Teams[t.GetAttributeValue("cn")] = &LDAPTeam{
					Name:  t.GetAttributeValue("cn"),
					Entry: t,
					Users: users,
				}
			}
		}
	}

	return &dir
}

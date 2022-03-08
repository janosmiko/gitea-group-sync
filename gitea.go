package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	urlpkg "net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

func (c *GiteaClient) AddUsersToTeam(users []GiteaAccount, team int) bool {
	ctx := context.Background()
	for i := 0; i < len(users); i++ {
		fullUsername := urlpkg.PathEscape(users[i].FullName)
		c.Command = "/api/v1/users/search?q=" + fullUsername + "&access_token="
		foundUsers := c.RequestSearchResults()

		for j := 0; j < len(foundUsers.Data); j++ {
			if strings.EqualFold(users[i].Login, foundUsers.Data[j].Login) {
				c.Command = "/api/v1/teams/" + fmt.Sprintf(
					"%d", team,
				) + "/members/" + foundUsers.Data[j].Login + "?access_token="
				errs := c.RequestPut(ctx)
				if len(errs) > 0 {
					zap.S().Info(
						"Error (GiteaTeam does not exist or Not Found GiteaUser) :",
						parseJSON(errs).(map[string]interface{})["message"],
					)
				}
			}
		}
	}
	return true
}

func (c *GiteaClient) DelUsersFromTeam(users []GiteaAccount, team int) bool {
	ctx := context.Background()
	for i := 0; i < len(users); i++ {
		c.Command = "/api/v1/users/search?uid=" + fmt.Sprintf("%d", users[i].ID) + "&access_token="

		foundUser := c.RequestSearchResults()

		c.Command = "/api/v1/teams/" + fmt.Sprintf(
			"%d", team,
		) + "/members/" + foundUser.Data[0].Login + "?access_token="
		c.RequestDel(ctx)
	}
	return true
}

func (c *GiteaClient) CreateOrganization(o GiteaOrganization) bool {
	ctx := context.Background()
	c.Command = "/api/v1/orgs/" + "?access_token="

	data := []byte(fmt.Sprintf(
		`{
	"description": "%[1]v",
	"full_name": "%[2]v",
	"location": "%[3]v",
	"repo_admin_change_team_access": %[4]v,
	"username": "%[5]v",
	"visibility": "%[6]v",
	"website": "%[7]v"
}`, o.Description, o.FullName, o.Location, false, o.Name, o.Visibility, o.Website,
	))

	c.RequestPost(ctx, bytes.NewBuffer(data))

	return true
}

func (c *GiteaClient) DeleteOrganization(orgName string) bool {
	ctx := context.Background()
	c.Command = "/api/v1/orgs/" + orgName + "?access_token="

	zap.S().Infof("Deleting organization: %v", orgName)
	c.RequestDel(ctx)
	return true
}

func (c *GiteaClient) CreateTeam(orgName string, t GiteaTeam, o GiteaCreateTeamOpts) bool {
	ctx := context.Background()
	c.Command = "/api/v1/orgs/" + orgName + "/teams?access_token="

	data := []byte(fmt.Sprintf(
		`{
	"can_create_org_repo": %[1]v,
	"description": "%[2]v",
	"includes_all_repositories": %[3]v,
	"name": "%[4]v",
	"permission": "%[5]v",
	"units": %[6]v,
	"units_map": %[7]v
}`, o.CanCreateOrgRepo, t.Description, o.IncludesAllRepositories, t.Name, o.Permission, o.Units,
		o.UnitsMap,
	))

	c.RequestPost(ctx, bytes.NewBuffer(data))

	return true
}

func (c *GiteaClient) DeleteTeam(teamID int) bool {
	ctx := context.Background()
	c.Command = "/api/v1/teams/" + fmt.Sprintf("%d", teamID) + "?access_token="

	zap.S().Infof("Deleting team with ID: %v", teamID)
	c.RequestDel(ctx)
	return true
}

func CheckStatusCode(res *http.Response) {
	switch {
	case http.StatusMultipleChoices <= res.StatusCode && res.StatusCode < http.StatusBadRequest:
		zap.L().Error("CheckStatusCode gitea apiKeys connection error: Redirect message")

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		zap.L().Error(buf.String())
	case http.StatusUnauthorized == res.StatusCode:
		zap.L().Error("CheckStatusCode gitea apiKeys connection Error: Unauthorized")

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		zap.L().Error(buf.String())
	case http.StatusBadRequest <= res.StatusCode && res.StatusCode < 500:
		zap.L().Error("CheckStatusCode gitea apiKeys connection error: Client error")

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		zap.L().Error(buf.String())
	case http.StatusInternalServerError <= res.StatusCode && res.StatusCode < 600:
		zap.L().Error("CheckStatusCode gitea apiKeys connection error Server error")

		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(res.Body)
		zap.L().Error(buf.String())
	}
}

func (c *GiteaClient) RequestGet(ctxParent context.Context) []byte {
	ctx, cancel := context.WithTimeout(ctxParent, time.Duration(c.ClientTimeout)*time.Second)
	defer cancel()

	cc := &http.Client{}
	url := c.BaseURL + c.Command + c.Token[c.BruteforceToken]

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	res, err := cc.Do(request)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	CheckStatusCode(res)
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	_ = res.Body.Close()

	return b
}

func (c *GiteaClient) RequestPut(ctxParent context.Context) []byte {
	ctx, cancel := context.WithTimeout(ctxParent, time.Duration(c.ClientTimeout)*time.Second)
	defer cancel()

	cc := &http.Client{Timeout: time.Second * time.Duration(c.ClientTimeout)}
	url := c.BaseURL + c.Command + c.Token[c.BruteforceToken]
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	res, err := cc.Do(request)

	CheckStatusCode(res)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	_ = res.Body.Close()

	return b
}

func (c *GiteaClient) RequestDel(ctxParent context.Context) {
	ctx, cancel := context.WithTimeout(ctxParent, time.Duration(c.ClientTimeout)*time.Second)
	defer cancel()

	cc := &http.Client{Timeout: time.Second * time.Duration(c.ClientTimeout)}
	url := c.BaseURL + c.Command + c.Token[c.BruteforceToken]
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	res, err := cc.Do(request)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	CheckStatusCode(res)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	_ = res.Body.Close()
}

func (c *GiteaClient) RequestPost(ctxParent context.Context, body io.Reader) {
	ctx, cancel := context.WithTimeout(ctxParent, time.Duration(c.ClientTimeout)*time.Second)
	defer cancel()

	cc := &http.Client{}
	url := c.BaseURL + c.Command + c.Token[c.BruteforceToken]
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	dump, _ := httputil.DumpRequest(request, true)
	zap.L().Debug(string(dump))

	res, err := cc.Do(request)
	if err != nil {
		zap.L().Fatal(err.Error())
	}

	CheckStatusCode(res)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	_, err = ioutil.ReadAll(res.Body)
	if err != nil {
		zap.L().Fatal(err.Error())
	}
	_ = res.Body.Close()
}

func (c *GiteaClient) RequestSearchResults() SearchResults {
	ctx := context.Background()
	b := c.RequestGet(ctx)

	var f SearchResults
	if err := json.Unmarshal(b, &f); err != nil {
		zap.L().Fatal(err.Error())
	}

	return f
}

func (c *GiteaClient) RequestUsersList(teamID int) (map[string]GiteaAccount, int) {
	ctx := context.Background()
	c.Command = "/api/v1/teams/" + fmt.Sprintf(
		"%d", teamID,
	) + "/members?access_token="

	b := c.RequestGet(ctx)

	var AccountU = make(map[string]GiteaAccount)

	var f []GiteaAccount
	err := json.Unmarshal(b, &f)
	if err != nil {
		zap.L().Info(err.Error())
		if c.BruteforceToken == len(c.Token)-1 {
			zap.L().Info("Token key is unsuitable, call to system administrator ")
		} else {
			zap.L().Info("Can't get UsersList try another token key")
		}
		if c.BruteforceToken < len(c.Token)-1 {
			c.BruteforceToken++
			zap.S().Infof("BruteforceToken=%d", c.BruteforceToken)
			AccountU, c.BruteforceToken = c.RequestUsersList(teamID)
		}
	}

	for i := 0; i < len(f); i++ {
		AccountU[f[i].Login] = GiteaAccount{
			//			Email:     f[i].Email,
			ID:       f[i].ID,
			FullName: f[i].FullName,
			Login:    f[i].Login,
		}
	}
	return AccountU, c.BruteforceToken
}

func (c *GiteaClient) RequestOrganizationList() []GiteaOrganization {
	c.BruteforceToken = 0
	c.Command = "/api/v1/admin/orgs?limit=1000&access_token="

	ctx := context.Background()
	b := c.RequestGet(ctx)

	var f []GiteaOrganization
	if err := json.Unmarshal(b, &f); err != nil {
		zap.S().Infof("Please check setting GITEA_TOKEN, GITEA_BASE_URL ")
		zap.L().Fatal(err.Error())
	}
	return f
}

func (c *GiteaClient) RequestTeamList(organizationName string) []GiteaTeam {
	c.Command = "/api/v1/orgs/" + organizationName + "/teams?access_token="

	ctx := context.Background()
	b := c.RequestGet(ctx)

	var f []GiteaTeam
	if err := json.Unmarshal(b, &f); err != nil {
		zap.L().Fatal(err.Error())
	}
	return f
}

func parseJSON(b []byte) interface{} {
	var f interface{}

	if err := json.Unmarshal(b, &f); err != nil {
		zap.L().Fatal(err.Error())
	}

	return f
}

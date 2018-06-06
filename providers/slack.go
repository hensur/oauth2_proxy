package providers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
)

type SlackProvider struct {
	*ProviderData
	TeamID  string
	GroupID string
}

// Slack API Response for https://api.slack.com/methods/users.identity
type SlackUserIdentityResponse struct {
	OK   bool
	User SlackUserItem
	Team SlackUserItem
}

type SlackUserItem struct {
	ID    string
	Name  string
	Email string
}

type SlackGroupListResponse struct {
	OK     bool
	Groups []SlackGroupItem
}

// SlackGroupItem contains info about a group
// This doesn't hold everything returned by the API, only the ID is needed
type SlackGroupItem struct {
	ID   string
	Name string
}

func NewSlackProvider(p *ProviderData) *SlackProvider {
	p.ProviderName = "slack"
	if p.LoginURL == nil || p.LoginURL.String() == "" {
		p.LoginURL = &url.URL{
			Scheme: "https",
			Host:   "slack.com",
			Path:   "/oauth/authorize",
		}
	}
	if p.RedeemURL == nil || p.RedeemURL.String() == "" {
		p.RedeemURL = &url.URL{
			Scheme: "https",
			Host:   "slack.com",
			Path:   "/api/oauth.access",
		}
	}
	if p.ValidateURL == nil || p.ValidateURL.String() == "" {
		p.ValidateURL = &url.URL{
			Scheme: "https",
			Host:   "slack.com",
			Path:   "/api",
		}
	}
	if p.Scope == "" {
		p.Scope = "identity.basic identity.email"
	}
	return &SlackProvider{ProviderData: p}
}

// SetTeamID to check if the member is in the right team
func (p *SlackProvider) SetTeamID(team string) {
	if team != "" {
		p.TeamID = team
		// If a team id is set we can restrict login to this team directly at login
		params, _ := url.ParseQuery(p.LoginURL.RawQuery)
		params.Set("team", team)
		p.LoginURL.RawQuery = params.Encode()
	}
}

// SetGroupID to check if the member is in a given group
func (p *SlackProvider) SetGroupID(group string) {
	if group != "" {
		p.GroupID = group
	}
}

func (p *SlackProvider) getEndpoint(endpointName string, accessToken string, params url.Values, responseItem interface{}) error {
	if params != nil {
		params.Add("token", accessToken)
	} else {
		params = url.Values{
			"token": {accessToken},
		}
	}
	endpoint := &url.URL{
		Scheme:   p.ValidateURL.Scheme,
		Host:     p.ValidateURL.Host,
		Path:     path.Join(p.ValidateURL.Path, "/"+endpointName),
		RawQuery: params.Encode(),
	}
	req, _ := http.NewRequest("GET", endpoint.String(), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf(
			"got %d from %q %s", resp.StatusCode, endpoint.String(), body)
	}

	if err := json.Unmarshal(body, responseItem); err != nil {
		return err
	}
	return nil
}

func (p *SlackProvider) getIdentity(accessToken string) (*SlackUserIdentityResponse, error) {
	var userIdentity SlackUserIdentityResponse
	if err := p.getEndpoint("users.identity", accessToken, nil, &userIdentity); err != nil {
		return nil, err
	}

	if userIdentity.OK == true {
		return &userIdentity, nil
	}
	return nil, fmt.Errorf("slack response is not ok: %v", userIdentity)
}

func (p *SlackProvider) getGroups(accessToken string) (*SlackGroupListResponse, error) {
	var groupList SlackGroupListResponse
	if err := p.getEndpoint("groups.list", accessToken, url.Values{
		"exclude_archived": {"true"},
		"exclude_members":  {"true"},
	}, &groupList); err != nil {
		return nil, err
	}

	if groupList.OK == true {
		return &groupList, nil
	}
	return nil, fmt.Errorf("slack response is not ok: %v", groupList)
}

func (p *SlackProvider) hasTeamID(resp *SlackUserIdentityResponse) bool {
	return resp.Team.ID == p.TeamID
}

func (p *SlackProvider) hasGroupID(resp *SlackGroupListResponse) bool {
	for _, group := range resp.Groups {
		if group.ID == p.GroupID {
			return true
		}
	}
	return false
}

func (p *SlackProvider) GetEmailAddress(s *SessionState) (string, error) {
	userIdentity, err := p.getIdentity(s.AccessToken)
	if err != nil {
		return "", nil
	}

	// if we require a TeamID, check that first
	if p.TeamID != "" {
		if ok := p.hasTeamID(userIdentity); !ok {
			log.Printf("teamid: %s does not match with %s", userIdentity.Team.ID, p.TeamID)
			return "", fmt.Errorf("team id doesn't match")
		}
	}
	// same for GroupID
	if p.GroupID != "" {
		groupList, err := p.getGroups(s.AccessToken)
		if err != nil {
			return "", nil
		}
		if ok := p.hasGroupID(groupList); !ok {
			log.Printf("groupid: %v does not match with %s", groupList.Groups, p.GroupID)
			return "", fmt.Errorf("group id doesn't match")
		}
	}

	if email := userIdentity.User.Email; email != "" {
		return email, nil
	}

	return "", nil
}

func (p *SlackProvider) SecondAttempt() bool {
	fmt.Println(p.GroupID)
	if strings.Contains(p.Scope, "groups:read") || p.GroupID == "" {
		return false
	}
	p.Scope = "groups:read"
	return true
}

package providers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
)

type SpacesProvider struct {
	*ProviderData
	APIUser  string
	APISpace string
	SpaceID  string
}

// SpacesUserIdentityResponse has basic user information
type SpacesUserIdentityResponse struct {
	ID    string
	EMail string
}

// SpacesSpaceResponse has space information (if user has access)
type SpacesSpaceResponse struct {
	ID string // only thing that matters
}

func NewSpacesProvider(p *ProviderData) *SpacesProvider {
	p.ProviderName = "spaces"
	if p.LoginURL == nil || p.LoginURL.String() == "" {
		p.LoginURL = &url.URL{
			Scheme: "https",
			Host:   "signup.spaces.de",
			Path:   "/o/oauth2/auth",
		}
	}
	if p.RedeemURL == nil || p.RedeemURL.String() == "" {
		p.RedeemURL = &url.URL{
			Scheme: "https",
			Host:   "signup.spaces.de",
			Path:   "/o/oauth2/token",
		}
	}
	if p.ValidateURL == nil || p.ValidateURL.String() == "" {
		p.ValidateURL = &url.URL{
			Scheme: "https",
			Host:   "api.spaces.de",
			Path:   "/v1",
		}
	}
	if p.Scope == "" {
		p.Scope = "profile:read spaces:read"
	}
	return &SpacesProvider{
		ProviderData: p,
		APIUser:      "users/me/profile",
		APISpace:     "spaces/%s", // space id placeholder
	}
}

// SetSpaceID to check if the member is in the right team
func (p *SpacesProvider) SetSpaceID(space string) {
	p.SpaceID = space
}

func (p *SpacesProvider) getEndpoint(endpointName string, accessToken string, params url.Values, responseItem interface{}) (*http.Response, error) {
	if params == nil {
		params = url.Values{}
	}

	endpoint := &url.URL{
		Scheme:   p.ValidateURL.Scheme,
		Host:     p.ValidateURL.Host,
		Path:     path.Join(p.ValidateURL.Path, endpointName),
		RawQuery: params.Encode(),
	}
	fmt.Println(accessToken)
	req, _ := http.NewRequest("GET", endpoint.String(), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	fmt.Println(resp)

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(
			"got %d from %q %s", resp.StatusCode, endpoint.String(), body)
	}

	if err := json.Unmarshal(body, responseItem); err != nil {
		return nil, err
	}
	return resp, nil
}

func (p *SpacesProvider) GetEmailAddress(s *SessionState) (string, error) {
	var userIdentity SpacesUserIdentityResponse
	if _, err := p.getEndpoint(p.APIUser, s.AccessToken, nil, &userIdentity); err != nil {
		return "", err
	}

	// Check for the right space ID
	if p.SpaceID != "" {
		var spaceInformation SpacesSpaceResponse
		if _, err := p.getEndpoint(fmt.Sprintf(p.APISpace, p.SpaceID), s.AccessToken, nil, &spaceInformation); err != nil {
			return "", err
		}
	}
	return userIdentity.EMail, nil
}

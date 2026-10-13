package authentication

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"k8s.io/apimachinery/pkg/runtime"
)

type keycloakClient struct {
	realm    string
	client   *http.Client
	adminURL *url.URL

	accessToken string
	idToken     string
}

func keycloakClientFor(keycloakURL string) (*keycloakClient, error) {
	baseURL, err := url.Parse(keycloakURL)
	if err != nil {
		return nil, fmt.Errorf("parsing url: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Proxy: http.ProxyFromEnvironment,
	}

	return &keycloakClient{
		realm: "master",
		client: &http.Client{
			Transport: transport,
		},
		adminURL: baseURL.JoinPath("admin", "realms", "master"),
	}, nil
}

type group struct {
	Name string `json:"name"`
}

func (kc *keycloakClient) CreateGroup(name string) error {
	groupURL := kc.adminURL.JoinPath("groups")

	group := group{
		Name: name,
	}

	groupBytes, err := json.Marshal(group)
	if err != nil {
		return fmt.Errorf("marshalling group configuration %v", group)
	}

	resp, err := kc.DoRequest(http.MethodPost, groupURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(groupBytes))
	if err != nil {
		return fmt.Errorf("sending POST request to %q to create group %s", groupURL.String(), name)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating group %q: %s - %s", name, resp.Status, respBytes)
	}

	return nil
}

type user struct {
	Username      string       `json:"username"`
	Email         string       `json:"email"`
	Enabled       bool         `json:"enabled"`
	EmailVerified bool         `json:"emailVerified"`
	Groups        []string     `json:"groups"`
	Credentials   []credential `json:"credentials"`
}

type credential struct {
	Temporary bool           `json:"temporary"`
	Type      credentialType `json:"type"`
	Value     string         `json:"value"`
}

type credentialType string

const (
	credentialTypePassword credentialType = "password"
)

func (kc *keycloakClient) CreateUser(username, password string, groups ...string) error {
	userURL := kc.adminURL.JoinPath("users")

	user := user{
		Username:      username,
		Email:         fmt.Sprintf("%s@payload.openshift.io", username),
		Enabled:       true,
		EmailVerified: true,
		Groups:        groups,
		Credentials: []credential{
			{
				Temporary: false,
				Type:      credentialTypePassword,
				Value:     password,
			},
		},
	}

	userBytes, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("marshalling user configuration %v", user)
	}

	resp, err := kc.DoRequest(http.MethodPost, userURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(userBytes))
	if err != nil {
		return fmt.Errorf("sending POST request to %q to create user %v", userURL.String(), user)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating user %v: %s - %s", user, resp.Status, respBytes)
	}

	return nil
}

type authenticationResponse struct {
	AccessToken      string `json:"access_token"`
	IDToken          string `json:"id_token"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (kc *keycloakClient) Authenticate(clientID, username, password string) error {
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	data.Set("grant_type", "password")
	data.Set("client_id", clientID)
	data.Set("scope", "openid")

	tokenURL := *kc.adminURL
	tokenURL.Path = fmt.Sprintf("/realms/%s/protocol/openid-connect/token", kc.realm)

	resp, err := kc.DoRequest(http.MethodPost, tokenURL.String(), "application/x-www-form-urlencoded", false, bytes.NewBuffer([]byte(data.Encode())))
	if err != nil {
		return fmt.Errorf("authenticating as user %q: %w", username, err)
	}
	defer resp.Body.Close()

	respBody := &authenticationResponse{}

	err = json.NewDecoder(resp.Body).Decode(respBody)
	if err != nil {
		return fmt.Errorf("unmarshalling response data: %w", err)
	}

	if respBody.Error != "" {
		return fmt.Errorf("%s: %s", respBody.Error, respBody.ErrorDescription)
	}

	kc.accessToken = respBody.AccessToken
	kc.idToken = respBody.IDToken

	return nil
}

func (kc *keycloakClient) DoRequest(method, url, contentType string, authenticated bool, body io.Reader) (*http.Response, error) {
	if len(kc.accessToken) == 0 && authenticated {
		panic("must authenticate before calling keycloakClient.DoRequest")
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", kc.accessToken))
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", runtime.ContentTypeJSON)

	return kc.client.Do(req)
}

func (kc *keycloakClient) AccessToken() string {
	return kc.accessToken
}

func (kc *keycloakClient) IdToken() string {
	return kc.idToken
}

func (kc *keycloakClient) ConfigureClient(clientId string) error {
	client, err := kc.GetClientByClientID(clientId)
	if err != nil {
		return fmt.Errorf("getting client %q: %w", clientId, err)
	}

	if err := kc.CreateClientGroupMapper(client.ID, "test-groups-mapper", "groups"); err != nil {
		return fmt.Errorf("creating group mapper for client %q: %w", clientId, err)
	}

	if err := kc.CreateClientAudienceMapper(client.ID, "test-aud-mapper"); err != nil {
		return fmt.Errorf("creating audience mapper for client %q: %w", clientId, err)
	}

	return nil
}

type groupMapper struct {
	Name           string            `json:"name"`
	Protocol       protocol          `json:"protocol"`
	ProtocolMapper protocolMapper    `json:"protocolMapper"`
	Config         groupMapperConfig `json:"config"`
}

type protocol string

const (
	protocolOpenIDConnect protocol = "openid-connect"
)

type protocolMapper string

const (
	protocolMapperOpenIDConnectGroupMembership protocolMapper = "oidc-group-membership-mapper"
	protocolMapperOpenIDConnectAudience        protocolMapper = "oidc-audience-mapper"
)

type groupMapperConfig struct {
	FullPath           booleanString `json:"full.path"`
	IDTokenClaim       booleanString `json:"id.token.claim"`
	AccessTokenClaim   booleanString `json:"access.token.claim"`
	UserInfoTokenClaim booleanString `json:"userinfo.token.claim"`
	ClaimName          string        `json:"claim.name"`
}

type booleanString string

const (
	booleanStringTrue  booleanString = "true"
	booleanStringFalse booleanString = "false"
)

func (kc *keycloakClient) CreateClientGroupMapper(clientId, name, claim string) error {
	mappersURL := *kc.adminURL
	mappersURL.Path += fmt.Sprintf("/clients/%s/protocol-mappers/models", clientId)

	mapper := &groupMapper{
		Name:           name,
		Protocol:       protocolOpenIDConnect,
		ProtocolMapper: protocolMapperOpenIDConnectGroupMembership,
		Config: groupMapperConfig{
			FullPath:           booleanStringFalse,
			IDTokenClaim:       booleanStringTrue,
			AccessTokenClaim:   booleanStringTrue,
			UserInfoTokenClaim: booleanStringTrue,
			ClaimName:          claim,
		},
	}

	mapperBytes, err := json.Marshal(mapper)
	if err != nil {
		return err
	}

	// Keycloak does not return the object on successful create so there's no need to attempt to retrieve it from the response
	resp, err := kc.DoRequest(http.MethodPost, mappersURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(mapperBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating mapper %q: %s %s", name, resp.Status, respBytes)
	}

	return nil
}

type audienceMapper struct {
	Name           string               `json:"name"`
	Protocol       protocol             `json:"protocol"`
	ProtocolMapper protocolMapper       `json:"protocolMapper"`
	Config         audienceMapperConfig `json:"config"`
}

type audienceMapperConfig struct {
	IDTokenClaim            booleanString `json:"id.token.claim"`
	AccessTokenClaim        booleanString `json:"access.token.claim"`
	IntrospectionTokenClaim booleanString `json:"introspection.token.claim"`
	IncludedClientAudience  string        `json:"included.client.audience"`
	IncludedCustomAudience  string        `json:"included.custom.audience"`
	LightweightClaim        booleanString `json:"lightweight.claim"`
}

func (kc *keycloakClient) CreateClientAudienceMapper(clientId, name string) error {
	mappersURL := *kc.adminURL
	mappersURL.Path += fmt.Sprintf("/clients/%s/protocol-mappers/models", clientId)

	mapper := &audienceMapper{
		Name:           name,
		Protocol:       protocolOpenIDConnect,
		ProtocolMapper: protocolMapperOpenIDConnectAudience,
		Config: audienceMapperConfig{
			IDTokenClaim:            booleanStringFalse,
			AccessTokenClaim:        booleanStringTrue,
			IntrospectionTokenClaim: booleanStringTrue,
			IncludedClientAudience:  "admin-cli",
			LightweightClaim:        booleanStringFalse,
		},
	}

	mapperBytes, err := json.Marshal(mapper)
	if err != nil {
		return err
	}

	// Keycloak does not return the object on successful create so there's no need to attempt to retrieve it from the response
	resp, err := kc.DoRequest(http.MethodPost, mappersURL.String(), runtime.ContentTypeJSON, true, bytes.NewBuffer(mapperBytes))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed creating mapper %q: %s %s", name, resp.Status, respBytes)
	}

	return nil
}

type client struct {
	ClientID string `json:"clientID"`
	ID       string `json:"id"`
}

// ListClients retrieves all clients
func (kc *keycloakClient) ListClients() ([]client, error) {
	clientsURL := *kc.adminURL
	clientsURL.Path += "/clients"

	resp, err := kc.DoRequest(http.MethodGet, clientsURL.String(), runtime.ContentTypeJSON, true, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing clients failed: %s", resp.Status)
	}

	clients := []client{}
	err = json.NewDecoder(resp.Body).Decode(&clients)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling response data: %w", err)
	}

	return clients, err
}

func (kc *keycloakClient) GetClientByClientID(clientID string) (*client, error) {
	clients, err := kc.ListClients()
	if err != nil {
		return nil, err
	}

	for _, c := range clients {
		if c.ClientID == clientID {
			return &c, nil
		}
	}

	return nil, fmt.Errorf("client with clientID %q not found", clientID)
}

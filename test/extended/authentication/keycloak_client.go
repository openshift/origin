package authentication

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	}

	return &keycloakClient{
		realm: "master",
		client: &http.Client{
			Transport: transport,
		},
		adminURL: baseURL.JoinPath("admin", "realms", "master"),
	}, nil
}

func (kc *keycloakClient) CreateGroup(name string) error {
	groupURL := kc.adminURL.JoinPath("groups")

	group := map[string]interface{}{
		"name": name,
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

func (kc *keycloakClient) CreateUser(username, password string, groups ...string) error {
	userURL := kc.adminURL.JoinPath("users")

	user := map[string]interface{}{
		"username":      username,
		"email":         fmt.Sprintf("%s@payload.openshift.io", username),
		"enabled":       true,
		"emailVerified": true,
		"groups":        groups,
		"credentials": []map[string]interface{}{
			{
				"temporary": false,
				"type":      "password",
				"value":     password,
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

func (kc *keycloakClient) Authenticate(clientID, username, password string) error {
	data := url.Values{
		"username":   []string{username},
		"password":   []string{password},
		"grant_type": []string{"password"},
		"client_id":  []string{clientID},
		"scope":      []string{"openid"},
	}

	tokenURL := *kc.adminURL
	tokenURL.Path = fmt.Sprintf("/realms/%s/protocol/openid-connect/token", kc.realm)

	resp, err := kc.DoRequest(http.MethodPost, tokenURL.String(), "application/x-www-form-urlencoded", false, bytes.NewBuffer([]byte(data.Encode())))
	if err != nil {
		return fmt.Errorf("authenticating as user %q: %w", username, err)
	}
	defer resp.Body.Close()

	respBody := map[string]interface{}{}
	respBodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response data: %w", err)
	}

	err = json.Unmarshal(respBodyData, &respBody)
	if err != nil {
		return fmt.Errorf("unmarshalling response body %s: %w", respBodyData, err)
	}

	accessTokenData, ok := respBody["access_token"]
	if !ok {
		return errors.New("unable to extract access token from the response body: access_token field is missing")
	}

	accessToken, ok := accessTokenData.(string)
	if !ok {
		return fmt.Errorf("expected accessToken to be of type string but was %T", accessTokenData)
	}
	kc.accessToken = accessToken

	idTokenData, ok := respBody["id_token"]
	if !ok {
		return errors.New("unable to extract id token from the response body: id_token field is missing")
	}

	idToken, ok := idTokenData.(string)
	if !ok {
		return fmt.Errorf("expected idToken to be of type string but was %T", idTokenData)
	}
	kc.idToken = idToken

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

	id, ok := client["id"]
	if !ok {
		return fmt.Errorf("client %q doesn't have 'id'", clientId)
	}

	idStr, ok := id.(string)
	if !ok {
		return fmt.Errorf("client %q 'id' is not of type string: %T", clientId, id)
	}

	if err := kc.CreateClientGroupMapper(idStr, "test-groups-mapper", "groups"); err != nil {
		return fmt.Errorf("creating group mapper for client %q: %w", clientId, err)
	}

	if err := kc.CreateClientAudienceMapper(idStr, "test-aud-mapper"); err != nil {
		return fmt.Errorf("creating audience mapper for client %q: %w", clientId, err)
	}

	return nil
}

func (kc *keycloakClient) CreateClientGroupMapper(clientId, name, claim string) error {
	mappersURL := *kc.adminURL
	mappersURL.Path += fmt.Sprintf("/clients/%s/protocol-mappers/models", clientId)

	mapper := map[string]interface{}{
		"name":           name,
		"protocol":       "openid-connect",
		"protocolMapper": "oidc-group-membership-mapper", // protocol-mapper type provided by Keycloak
		"config": map[string]string{
			"full.path":            "false",
			"id.token.claim":       "true",
			"access.token.claim":   "true",
			"userinfo.token.claim": "true",
			"claim.name":           claim,
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

func (kc *keycloakClient) CreateClientAudienceMapper(clientId, name string) error {
	mappersURL := *kc.adminURL
	mappersURL.Path += fmt.Sprintf("/clients/%s/protocol-mappers/models", clientId)

	mapper := map[string]interface{}{
		"name":           name,
		"protocol":       "openid-connect",
		"protocolMapper": "oidc-audience-mapper", // protocol-mapper type provided by Keycloak
		"config": map[string]string{
			"id.token.claim":            "false",
			"access.token.claim":        "true",
			"introspection.token.claim": "true",
			"included.client.audience":  "admin-cli",
			"included.custom.audience":  "",
			"lightweight.claim":         "false",
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

// ListClients retrieves all clients
func (kc *keycloakClient) ListClients() ([]map[string]interface{}, error) {
	clientsURL := *kc.adminURL
	clientsURL.Path += "/clients"

	resp, err := kc.DoRequest(http.MethodGet, clientsURL.String(), runtime.ContentTypeJSON, true, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing clients failed: %s: %s", resp.Status, respBytes)
	}

	clients := []map[string]interface{}{}
	err = json.Unmarshal(respBytes, &clients)

	return clients, err
}

func (kc *keycloakClient) GetClientByClientID(clientID string) (map[string]interface{}, error) {
	clients, err := kc.ListClients()
	if err != nil {
		return nil, err
	}

	for _, c := range clients {
		if c["clientId"].(string) == clientID {
			return c, nil
		}
	}

	return nil, fmt.Errorf("client with clientID %q not found", clientID)
}

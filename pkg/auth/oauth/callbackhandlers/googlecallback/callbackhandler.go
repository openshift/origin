package googlecallback

import (
	// "bytes"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oauth/registry/accesstoken"
)

type OauthCallbackHandler struct {
	TokenRegistry accesstoken.Registry
	ClientId      string
	ClientSecret  string
}

func (o *OauthCallbackHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	if len(code) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// the code we get back seems to have multiple parts, we need everything before the period
	code = code[0:strings.Index(code, ".")]
	glog.Infof("Got a callback from google for: %v", code)

	//
	// with this code, I thee post.  We need the token
	resp, err := http.PostForm("https://accounts.google.com/o/oauth2/token", url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {o.ClientId},
		"client_secret": {o.ClientSecret},
		"redirect_uri":  {"http://localhost:8080/oauth2callback/google"},
	})
	if err != nil {
		glog.Errorf("Error making post: %v", err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		glog.Errorf("Error making post: %v", err)
	}
	var jsonResponse interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		glog.Errorf("Error making post: %v", err)
	}
	tokenResponseMap := jsonResponse.(map[string]interface{})
	glog.V(1).Infof("Got a response: %v", tokenResponseMap)

	googleAccessToken := tokenResponseMap["access_token"].(string)
	glog.Infof("Got a google access token of: %v", googleAccessToken)
	googleIdToken := tokenResponseMap["id_token"].(string)
	glog.Infof("Got a google id token of: %v", googleIdToken)

	// pull the ID from the id token.  We don't need to validate since we got the info directly from
	// our https request to google
	encodedIdTokenParts := strings.Split(googleIdToken, ".")
	if len(encodedIdTokenParts) >= 2 {
		encodedId := encodedIdTokenParts[1]
		decodedIdBytes := make([]byte, base64.StdEncoding.DecodedLen(len(encodedId)))
		base64.StdEncoding.Decode(decodedIdBytes, []byte(encodedId))

		// TODO complete hack. figure out why the decode fails
		decodedIdBytes = append(decodedIdBytes, byte('}'))

		glog.V(1).Infof("decodedIdBytes %v", string(decodedIdBytes))

		var decodedIdJson interface{}
		err = json.Unmarshal(decodedIdBytes, &decodedIdJson)
		if err != nil {
			glog.Errorf("Error interpretting token: %v", err)
		}

		idMap := decodedIdJson.(map[string]interface{})
		emailAddress := idMap["email"]
		glog.Infof("User email: %v", emailAddress)

	}
}

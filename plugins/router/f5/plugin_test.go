package f5

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/openshift/origin/pkg/cmd/util"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

type (
	// mockF5State stores the state necessary to mock the functionality of an F5
	// BIG-IP host that the F5 router uses.
	mockF5State struct {
		// policies is the set of policies that exist in the mock F5 host.
		policies map[string]map[string]policyRule

		// vserverPolicies represents the associations between vservers and policies
		// in the mock F5 host.
		vserverPolicies map[string]map[string]bool

		// certs represents the set of certificates that have been installed into
		// the mock F5 host.
		certs map[string]bool

		// keys represents the set of certificates that have been installed into
		// the mock F5 host.
		keys map[string]bool

		// serverSslProfiles represents the set of server-ssl profiles that exist in
		// the mock F5 host.
		serverSslProfiles map[string]bool

		// clientSslProfiles represents the set of client-ssl profiles that exist in
		// the mock F5 host.
		clientSslProfiles map[string]bool

		// vserverProfiles represents the associations between vservers and
		// client-ssl and server-ssl profiles in the mock F5 host.
		//
		// Note that although the F5 management console displays client and server
		// profiles separately, the F5 iControl REST interface puts these
		// associations under a single REST endpoint.
		vserverProfiles map[string]map[string]bool

		// datagroups represents the iRules data-groups in the F5 host.  For our
		// purposes, we assume that every data-group maps strings to strings.
		datagroups map[string]datagroup

		// iRules represents the iRules that exist in the F5 host.
		iRules map[string]iRule

		// vserverIRules represents the associations between vservers and iRules in
		// the mock F5 host.
		vserverIRules map[string][]string

		// pools represents the pools that exist on the mock F5 host.
		pools map[string]pool
	}

	mockF5 struct {
		// state is the internal state of the mock F5 BIG-IP host.
		state mockF5State

		// server is the mock F5 BIG-IP host, which accepts HTTPS connections and
		// behaves as an actual F5 BIG-IP host for testing purposes.
		server *httptest.Server
	}

	// A policyCondition describes a single condition for a policy rule to match.
	policyCondition struct {
		HttpHost    bool     `json:"httpHost,omitempty"`
		HttpUri     bool     `json:"httpUri,omitempty"`
		PathSegment bool     `json:"pathSegment,omitempty"`
		Index       int      `json:"index"`
		Host        bool     `json:"host,omitempty"`
		Values      []string `json:"values"`
	}

	// A policyRule has a name and comprises a list of conditions and a list of
	// actions.
	policyRule struct {
		conditions []policyCondition
	}

	// A datagroup is an associative array.  For our purposes, a datagroup maps
	// strings to strings.
	datagroup map[string]string

	// An iRule comprises a string of TCL code.
	iRule string

	// A pool comprises a set of strings of the form addr:port.
	pool map[string]bool
)

const (
	httpVserverName               = "ose-vserver"
	httpsVserverName              = "https-ose-vserver"
	insecureRoutesPolicyName      = "openshift_insecure_routes"
	secureRoutesPolicyName        = "openshift_secure_routes"
	passthroughIRuleName          = "openshift_passthrough_irule"
	passthroughIRuleDatagroupName = "ssl_passthrough_servername_dg"
)

func mockExecCommand(command string, args ...string) *exec.Cmd {
	// TODO: Parse ssh and scp commands in order to keep track of what files are
	// being uploaded so that we can perform more validations in HTTP handlers in
	// the mock F5 host that use files uploaded via SSH.
	return exec.Command("true")
}

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc func(mockF5State) http.HandlerFunc
}

var f5Routes = []Route{
	{"getPolicy", "GET", "/mgmt/tm/ltm/policy/{policyName}", getPolicyHandler},
	{"postPolicy", "POST", "/mgmt/tm/ltm/policy", postPolicyHandler},
	{"postRule", "POST", "/mgmt/tm/ltm/policy/{policyName}/rules", postRuleHandler},
	{"getPolicies", "GET", "/mgmt/tm/ltm/virtual/{vserverName}/policies", getPoliciesHandler},
	{"associatePolicyWithVserver", "POST", "/mgmt/tm/ltm/virtual/{vserverName}/policies", associatePolicyWithVserverHandler},
	{"getDatagroup", "GET", "/mgmt/tm/ltm/data-group/internal/{datagroupName}", getDatagroupHandler},
	{"patchDatagroup", "PATCH", "/mgmt/tm/ltm/data-group/internal/{datagroupName}", patchDatagroupHandler},
	{"postDatagroup", "POST", "/mgmt/tm/ltm/data-group/internal", postDatagroupHandler},
	{"getIRule", "GET", "/mgmt/tm/ltm/rule/{iRuleName}", getIRuleHandler},
	{"postIRule", "POST", "/mgmt/tm/ltm/rule", postIRuleHandler},
	{"getVserver", "GET", "/mgmt/tm/ltm/virtual/{vserverName}", getVserverHandler},
	{"patchVserver", "PATCH", "/mgmt/tm/ltm/virtual/{vserverName}", patchVserverHandler},
	{"postPool", "POST", "/mgmt/tm/ltm/pool", postPoolHandler},
	{"deletePool", "DELETE", "/mgmt/tm/ltm/pool/{poolName}", deletePoolHandler},
	{"getPoolMembers", "GET", "/mgmt/tm/ltm/pool/{poolName}/members", getPoolMembersHandler},
	{"postPoolMember", "POST", "/mgmt/tm/ltm/pool/{poolName}/members", postPoolMemberHandler},
	{"deletePoolMember", "DELETE", "/mgmt/tm/ltm/pool/{poolName}/members/{memberName}", deletePoolMemberHandler},
	{"getRules", "GET", "/mgmt/tm/ltm/policy/{policyName}/rules", getRulesHandler},
	{"postCondition", "POST", "/mgmt/tm/ltm/policy/{policyName}/rules/{ruleName}/conditions", postConditionHandler},
	{"postAction", "POST", "/mgmt/tm/ltm/policy/{policyName}/rules/{ruleName}/actions", postActionHandler},
	{"deleteRule", "DELETE", "/mgmt/tm/ltm/policy/{policyName}/rules/{ruleName}", deleteRuleHandler},
	{"postSslCert", "POST", "/mgmt/tm/sys/crypto/cert", postSslCertHandler},
	{"postSslKey", "POST", "/mgmt/tm/sys/crypto/key", postSslKeyHandler},
	{"postClientSslProfile", "POST", "/mgmt/tm/ltm/profile/client-ssl", postClientSslProfileHandler},
	{"deleteClientSslProfile", "DELETE", "/mgmt/tm/ltm/profile/client-ssl/{profileName}", deleteClientSslProfileHandler},
	{"postServerSslProfile", "POST", "/mgmt/tm/ltm/profile/server-ssl", postServerSslProfileHandler},
	{"deleteServerSslProfile", "DELETE", "/mgmt/tm/ltm/profile/server-ssl/{profileName}", deleteServerSslProfileHandler},
	{"associateProfileWithVserver", "POST", "/mgmt/tm/ltm/virtual/{vserverName}/profiles", associateProfileWithVserver},
	{"deleteSslVserverProfile", "DELETE", "/mgmt/tm/ltm/virtual/{vserverName}/profiles/{profileName}", deleteSslVserverProfileHandler},
	{"deleteSslKey", "DELETE", "/mgmt/tm/sys/file/ssl-key/{keyName}", deleteSslKeyHandler},
	{"deleteSslCert", "DELETE", "/mgmt/tm/sys/file/ssl-cert/{certName}", deleteSslCertHandler},
}

func newF5Routes(mockF5State mockF5State) *mux.Router {
	mockF5 := mux.NewRouter().StrictSlash(true)
	for _, route := range f5Routes {
		mockF5.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(route.HandlerFunc(mockF5State))
	}
	return mockF5
}

func newTestRouterWithState(state mockF5State) (*F5Plugin, *mockF5, error) {
	routerLogLevel := util.Env("TEST_ROUTER_LOGLEVEL", "")
	if routerLogLevel != "" {
		flag.Set("v", routerLogLevel)
	}

	execCommand = mockExecCommand

	server := httptest.NewUnstartedServer(newF5Routes(state))
	// Work around performance issues with Golang's ECDHE implementation.
	// See <https://github.com/openshift/origin/issues/4407>.
	server.Config.TLSConfig = new(tls.Config)
	server.Config.TLSConfig.CipherSuites = []uint16{
		tls.TLS_RSA_WITH_RC4_128_SHA,
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}
	server.TLS = server.Config.TLSConfig
	server.StartTLS()

	url, err := url.Parse(server.URL)
	if err != nil {
		return nil, nil,
			fmt.Errorf("Failed to parse URL of mock F5 host; URL: %s, error: %v",
				url, err)
	}

	f5PluginTestCfg := F5PluginConfig{
		Host:         url.Host,
		Username:     "admin",
		Password:     "password",
		HttpVserver:  httpVserverName,
		HttpsVserver: httpsVserverName,
		PrivateKey:   "/dev/null",
		Insecure:     true,
	}
	router, err := NewF5Plugin(f5PluginTestCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create new F5 router: %v", err)
	}

	mockF5 := &mockF5{state: state, server: server}

	return router, mockF5, nil
}

func newTestRouter() (*F5Plugin, *mockF5, error) {
	state := mockF5State{
		policies: map[string]map[string]policyRule{},
		vserverPolicies: map[string]map[string]bool{
			httpVserverName:  {},
			httpsVserverName: {},
		},
		certs:             map[string]bool{},
		keys:              map[string]bool{},
		serverSslProfiles: map[string]bool{},
		clientSslProfiles: map[string]bool{},
		vserverProfiles: map[string]map[string]bool{
			httpsVserverName: {},
		},
		datagroups:    map[string]datagroup{},
		iRules:        map[string]iRule{},
		vserverIRules: map[string][]string{},
		pools:         map[string]pool{},
	}

	return newTestRouterWithState(state)
}

func (f5 *mockF5) close() {
	f5.server.Close()
}

func validatePolicyName(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, policyName string) bool {
	_, ok := f5state.policies[policyName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested Policy (/Common/%s) was not found."}`,
			policyName)
		return false
	}

	return true
}

func getPolicyHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policyName := vars["policyName"]

		if !validatePolicyName(response, request, f5state, policyName) {
			return
		}

		fmt.Fprintf(response,
			`{"controls":["forwarding"],"fullPath":"%s","generation":1,"kind":"tm:ltm:policy:policystate","name":"%s","requires":["http"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~%s/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/%s?ver=11.6.0","strategy":"/Common/best-match"}`,
			policyName, policyName, policyName, policyName)
	}
}

func OK(response http.ResponseWriter) {
	fmt.Fprint(response, `{"code":200,"message":"OK"}`)
}

func postPolicyHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		policyName := payload.Name

		f5state.policies[policyName] = map[string]policyRule{}

		OK(response)
	}
}

func postRuleHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policyName := vars["policyName"]

		if !validatePolicyName(response, request, f5state, policyName) {
			return
		}

		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		ruleName := payload.Name

		newRule := policyRule{[]policyCondition{}}
		f5state.policies[policyName][ruleName] = newRule

		OK(response)
	}
}

func validateVserverName(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, vserverName string) bool {
	if !recogniseVserver(vserverName) {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested Virtual Server (/Common/%s) was not found."}`,
			vserverName)
		return false
	}

	return true
}

func getPoliciesHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserverName := vars["vserverName"]

		if !validateVserverName(response, request, f5state, vserverName) {
			return
		}

		fmt.Fprint(response, `{"items":[{"controls":["classification"],"fullPath":"/Common/_sys_CEC_SSL_client_policy","generation":1,"hints":["no-write","no-delete","no-exclusion"],"kind":"tm:ltm:policy:policystate","name":"_sys_CEC_SSL_client_policy","partition":"Common","requires":["ssl-persistence"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_client_policy/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_client_policy?ver=11.6.0","strategy":"/Common/first-match"},{"controls":["classification"],"fullPath":"/Common/_sys_CEC_SSL_server_policy","generation":1,"hints":["no-write","no-delete","no-exclusion"],"kind":"tm:ltm:policy:policystate","name":"_sys_CEC_SSL_server_policy","partition":"Common","requires":["ssl-persistence"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_server_policy/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_server_policy?ver=11.6.0","strategy":"/Common/first-match"},{"controls":["classification"],"fullPath":"/Common/_sys_CEC_video_policy","generation":1,"hints":["no-write","no-delete","no-exclusion"],"kind":"tm:ltm:policy:policystate","name":"_sys_CEC_video_policy","partition":"Common","requires":["http"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_video_policy/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_video_policy?ver=11.6.0","strategy":"/Common/first-match"}`)
		for policyName := range f5state.vserverPolicies[vserverName] {
			fmt.Fprintf(response,
				`,{"controls":["forwarding"],"fullPath":"/Common/%s","generation":1,"kind":"tm:ltm:policy:policystate","name":"%s","partition":"Common","requires":["http"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~%s/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~%s?ver=11.6.0","strategy":"/Common/best-match"}`,
				policyName, policyName, policyName, policyName)
		}

		fmt.Fprintf(response, `],"kind":"tm:ltm:policy:policycollectionstate","selfLink":"https://localhost/mgmt/tm/ltm/policy?ver=11.6.0"}`)
	}
}

func recogniseVserver(vserverName string) bool {
	return vserverName == httpVserverName || vserverName == httpsVserverName
}

func associatePolicyWithVserverHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserverName := vars["vserverName"]

		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		policyName := payload.Name

		validVserver := recogniseVserver(vserverName)
		_, validPolicy := f5state.policies[policyName]

		if !validVserver || !validPolicy {
			response.WriteHeader(http.StatusNotFound)
		}

		if !validVserver && !validPolicy {
			fmt.Fprintf(response,
				`{"code":400,"errorStack":[],"message":"01070712:3: Values (%s) specified for virtual server policy (/Common/%s %s): foreign key index (policy_FK) do not point at an item that exists in the database."}`,
				policyName, vserverName, policyName)
			return
		}

		if !validVserver {
			fmt.Fprintf(response,
				`{"code":400,"errorStack":[],"message":"01070712:3: Values (/Common/%s) specified for virtual server policy (/Common/%s /Common/%s): foreign key index (vs_FK) do not point at an item that exists in the database."}`,
				vserverName, vserverName, policyName)
			return
		}

		if !validPolicy {
			fmt.Fprintf(response,
				`{"code":404,"errorStack":[],"message":"01020036:3: The requested policy (%s) was not found."}`,
				policyName)
			return
		}

		f5state.vserverPolicies[vserverName][policyName] = true

		OK(response)
	}
}

func validateDatagroupName(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, datagroupName string) bool {
	_, ok := f5state.datagroups[datagroupName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested value list (/Common/%s) was not found."}`,
			datagroupName)
		return false
	}

	return true
}

func getDatagroupHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		datagroupName := vars["datagroupName"]

		if !validateDatagroupName(response, request, f5state, datagroupName) {
			return
		}

		datagroup := f5state.datagroups[datagroupName]

		fmt.Fprintf(response,
			`{"fullPath":"%s","generation":1556,"kind":"tm:ltm:data-group:internal:internalstate","name":"%s","records":[`,
			datagroupName, datagroupName)

		first := true
		for key, value := range datagroup {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			fmt.Fprintf(response, `{"data":"%s","name":"%s"}`, value, key)
		}
		fmt.Fprintf(response,
			`],"selfLink":"https://localhost/mgmt/tm/ltm/data-group/internal/%s?ver=11.6.0","type":"string"}`,
			datagroupName)
	}
}

func patchDatagroupHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		datagroupName := vars["datagroupName"]

		if !validateDatagroupName(response, request, f5state, datagroupName) {
			return
		}

		type record struct {
			Key   string `json:"name"`
			Value string `json:"data"`
		}
		payload := struct {
			Records []record `json:"records"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		dg := datagroup{}
		for _, record := range payload.Records {
			dg[record.Key] = record.Value
		}

		f5state.datagroups[datagroupName] = dg

		OK(response)
	}
}

func postDatagroupHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		datagroupName := payload.Name

		_, datagroupAlreadyExists := f5state.datagroups[datagroupName]
		if datagroupAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"errorStack":[],"message":"01020066:3: The requested value list (/Common/%s) already exists in partition Common."}`,
				datagroupName)
			return
		}

		f5state.datagroups[datagroupName] = map[string]string{}

		OK(response)
	}
}

func getIRuleHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		iRuleName := vars["iRuleName"]

		iRuleCode, ok := f5state.iRules[iRuleName]
		if !ok {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"errorStack":[],"message":"01020036:3: The requested iRule (/Common/%s) was not found."}`,
				iRuleName)
			return
		}

		fmt.Fprintf(response,
			`{"apiAnonymous":"%s","fullPath":"%s","generation":386,"kind":"tm:ltm:rule:rulestate","name":"%s","selfLink":"https://localhost/mgmt/tm/ltm/rule/%s?ver=11.6.0"}`,
			iRuleCode, iRuleName, iRuleName, iRuleName)
	}
}

func postIRuleHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
			Code string `json:"apiAnonymous"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		iRuleName := payload.Name
		iRuleCode := payload.Code

		_, iRuleAlreadyExists := f5state.iRules[iRuleName]
		if iRuleAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"errorStack":[],"message":"01020066:3: The requested iRule (/Common/%s) already exists in partition Common."}`,
				iRuleName)
			return
		}

		// F5 iControl REST does not know how to parse \u escapes.
		if badCharIdx := strings.Index(string(iRuleCode), `\u`); badCharIdx != -1 {
			response.WriteHeader(http.StatusBadRequest)
			truncateAt := badCharIdx
			if truncateAt > 86 {
				truncateAt = 86
			}
			fmt.Fprintf(response,
				`{"code":400,"message":"can't parse TCL script beginning with\n%.*s\n","errorStack":[]}`,
				truncateAt, iRuleCode)
			return
		}

		f5state.iRules[iRuleName] = iRule(iRuleCode)

		OK(response)
	}
}

func getVserverHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserverName := vars["vserverName"]

		if !validateVserverName(response, request, f5state, vserverName) {
			return
		}

		description := "OpenShift Enterprise Virtual Server for HTTPS connections"
		destination := "10.1.1.1:443"
		if vserverName == httpVserverName {
			description = "OpenShift Enterprise Virtual Server for HTTP connections"
			destination = "10.1.1.2:80"
		}

		fmt.Fprintf(response,
			`{"addressStatus":"yes","autoLasthop":"default","cmpEnabled":"yes","connectionLimit":0,"description":"%s","destination":"/Common/%s","enabled":true,"fullPath":"%s","generation":387,"gtmScore":0,"ipProtocol":"tcp","kind":"tm:ltm:virtual:virtualstate","mask":"255.255.255.255","mirror":"disabled","mobileAppTunnel":"disabled","name":"%s","nat64":"disabled","policiesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/virtual/~Common~%s/policies?ver=11.6.0"},"profilesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/virtual/~Common~%s/profiles?ver=11.6.0"},"rateLimit":"disabled","rateLimitDstMask":0,"rateLimitMode":"object","rateLimitSrcMask":0,"rules":[`,
			description, destination, vserverName,
			vserverName, vserverName, vserverName)

		first := true
		for _, ruleName := range f5state.vserverIRules[vserverName] {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			fmt.Fprintf(response, `"/Common/%s"`, ruleName)
		}

		fmt.Fprintf(response, `],"selfLink":"https://localhost/mgmt/tm/ltm/virtual/%s?ver=11.6.0","source":"0.0.0.0/0","sourceAddressTranslation":{"type":"none"},"sourcePort":"preserve","synCookieStatus":"not-activated","translateAddress":"enabled","translatePort":"enabled","vlansDisabled":true,"vsIndex":11}`, vserverName)
	}
}

func patchVserverHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserverName := vars["vserverName"]

		if !validateVserverName(response, request, f5state, vserverName) {
			return
		}

		payload := struct {
			Rules []string `json:"rules"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		iRules := []string(payload.Rules)

		f5state.vserverIRules[vserverName] = iRules

		OK(response)
	}
}

func postPoolHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		poolName := payload.Name

		_, poolAlreadyExists := f5state.pools[poolName]
		if poolAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"errorStack":[],"message":"01020066:3: The requested Pool (/Common/%s) already exists in partition Common."}`,
				poolName)
			return
		}

		f5state.pools[poolName] = pool{}

		OK(response)
	}
}

func validatePoolName(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, poolName string) bool {
	_, ok := f5state.pools[poolName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3:The requested Pool (/Common/%s) was not found."}`,
			poolName)
		return false
	}

	return true
}

func deletePoolHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolName := vars["poolName"]

		if !validatePoolName(response, request, f5state, poolName) {
			return
		}

		// TODO: Validate that no rule references the pool.

		delete(f5state.pools, poolName)

		OK(response)
	}
}

func getPoolMembersHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolName := vars["poolName"]

		if !validatePoolName(response, request, f5state, poolName) {
			return
		}

		fmt.Fprint(response, `{"items":[`)

		first := true
		for member := range f5state.pools[poolName] {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			addr := strings.Split(member, ":")[0]
			fmt.Fprintf(response,
				`{"address":"%s","connectionLimit":0,"dynamicRatio":1,"ephemeral":"false","fqdn":{"autopopulate":"disabled"},"fullPath":"/Common/%s","generation":1190,"inheritProfile":"enabled","kind":"tm:ltm:pool:members:membersstate","logging":"disabled","monitor":"default","name":"%s","partition":"Common","priorityGroup":0,"rateLimit":"disabled","ratio":1,"selfLink":"https://localhost/mgmt/tm/ltm/pool/%s/members/~Common~%s?ver=11.6.0","session":"monitor-enabled","state":"up"}`,
				addr, member, member, member, member)
		}

		fmt.Fprintf(response,
			`],"kind":"tm:ltm:pool:members:memberscollectionstate","selfLink":"https://localhost/mgmt/tm/ltm/pool/%s/members?ver=11.6.0"}`,
			poolName)
	}
}

func postPoolMemberHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolName := vars["poolName"]

		if !validatePoolName(response, request, f5state, poolName) {
			return
		}

		payload := struct {
			Member string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		memberName := payload.Member

		_, memberAlreadyExists := f5state.pools[poolName][memberName]
		if memberAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"message":"01020066:3: The requested Pool Member (/Common/%s /Common/%s) already exists in partition Common.","errorStack":[]}`,
				poolName, strings.Replace(memberName, ":", " ", 1))
			return
		}

		f5state.pools[poolName][memberName] = true

		OK(response)
	}
}

func deletePoolMemberHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolName := vars["poolName"]
		memberName := vars["memberName"]

		if !validatePoolName(response, request, f5state, poolName) {
			return
		}

		_, foundMember := f5state.pools[poolName][memberName]
		if !foundMember {
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested Pool Member (/Common/%s /Common/%s) was not found.","errorStack":[]}`,
				poolName, strings.Replace(memberName, ":", " ", 1))
		}

		delete(f5state.pools[poolName], memberName)

		OK(response)
	}
}

func getRulesHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policyName := vars["policyName"]

		if !validatePolicyName(response, request, f5state, policyName) {
			return
		}

		fmt.Fprint(response, `{"items": [`)

		first := true
		for ruleName := range f5state.policies[policyName] {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			fmt.Fprintf(response,
				`{"actionsReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/%s/rules/%s/actions?ver=11.6.0"},"conditionsReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/%s/rules/%s/conditions?ver=11.6.0"},"fullPath":"%s","generation":1218,"kind":"tm:ltm:policy:rules:rulesstate","name":"%s","ordinal":0,"selfLink":"https://localhost/mgmt/tm/ltm/policy/%s/rules/%s?ver=11.6.0"}`,
				policyName, ruleName, policyName, ruleName,
				ruleName, ruleName, policyName, ruleName)
		}

		fmt.Fprintf(response,
			`],"kind":"tm:ltm:policy:rules:rulescollectionstate","selfLink":"https://localhost/mgmt/tm/ltm/policy/%s/rules?ver=11.6.0"}`,
			policyName)
	}
}

func validateRuleName(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, policyName, ruleName string) bool {
	for rule := range f5state.policies[policyName] {
		if rule == ruleName {
			return true
		}
	}

	response.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(response,
		`{"code":404,"errorStack":[],"message":"01020036:3: The requested policy rule (/Common/%s %s) was not found."}`,
		policyName, ruleName)

	return false
}

func postConditionHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policyName := vars["policyName"]
		ruleName := vars["ruleName"]

		if !validatePolicyName(response, request, f5state, policyName) {
			return
		}

		foundRule := validateRuleName(response, request, f5state,
			policyName, ruleName)
		if !foundRule {
			return
		}

		payload := policyCondition{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		// TODO: Validate more fields in the payload: equals, request, maybe others.

		conditions := f5state.policies[policyName][ruleName].conditions
		conditions = append(conditions, payload)
		f5state.policies[policyName][ruleName] = policyRule{conditions}

		OK(response)
	}
}

func postActionHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policyName := vars["policyName"]
		ruleName := vars["ruleName"]

		if !validatePolicyName(response, request, f5state, policyName) {
			return
		}

		foundRule := validateRuleName(response, request, f5state,
			policyName, ruleName)
		if !foundRule {
			return
		}

		// TODO: Validate payload.

		OK(response)
	}
}

func deleteRuleHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policyName := vars["policyName"]
		ruleName := vars["ruleName"]

		if !validatePolicyName(response, request, f5state, policyName) {
			return
		}

		foundRule := validateRuleName(response, request, f5state,
			policyName, ruleName)
		if !foundRule {
			return
		}

		delete(f5state.policies[policyName], ruleName)

		OK(response)
	}
}

func postSslCertHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		certName := payload.Name

		// TODO: Validate filename (which would require more elaborate mocking of
		// the ssh and scp commands in mockExecCommand).

		// F5 adds the extension to the filename specified in the payload, and this
		// extension must be included in subsequent REST calls that reference the
		// file.
		f5state.certs[fmt.Sprintf("%s.crt", certName)] = true

		OK(response)
	}
}

func postSslKeyHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		keyName := payload.Name

		// TODO: Validate filename (which would require more elaborate mocking of
		// the ssh and scp commands in mockExecCommand).

		f5state.keys[fmt.Sprintf("%s.key", keyName)] = true

		OK(response)
	}
}

func validateClientKey(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, keyName string) bool {
	_, ok := f5state.keys[keyName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":400,"message":"010717e3:3: Client SSL profile must have RSA certificate/key pair.","errorStack":[]}`)
		return false
	}

	return true
}

func validateCert(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, certName string) bool {
	_, ok := f5state.certs[certName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":400,"message":"0107134a:3: File object by name (%s) is missing.","errorStack":[]}`,
			certName)
		return false
	}

	return true
}

func postClientSslProfileHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			CertificateName string `json:"cert"`
			KeyName         string `json:"key"`
			Name            string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		keyName := payload.KeyName
		certificateName := payload.CertificateName
		clientSslProfileName := payload.Name

		// Complain about name collision first because F5 does.
		_, clientSslProfileAlreadyExists := f5state.clientSslProfiles[clientSslProfileName]
		if clientSslProfileAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"message":"01020066:3: The requested ClientSSL Profile (/Common/%s) already exists in partition Common.","errorStack":[]`,
				clientSslProfileName)
			return
		}

		// Check key before certificate because if both are missing, F5 returns the
		// same error message as if only the key were missing.
		if !validateClientKey(response, request, f5state, keyName) {
			return
		}

		if !validateCert(response, request, f5state, certificateName) {
			return
		}

		// The name for a client-ssl profile cannot collide with the name of any
		// server-ssl profile either, but F5 complains about name collisions with
		// server-ssl profiles only if the above checks pass.
		_, serverSslProfileAlreadyExists := f5state.serverSslProfiles[clientSslProfileName]
		if serverSslProfileAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":400,"message":"01070293:3: The profile name (/Common/%s) is already assigned to another profile.","errorStack":[]}`,
				clientSslProfileName)
			return
		}

		f5state.clientSslProfiles[clientSslProfileName] = true

		OK(response)
	}
}

func deleteClientSslProfileHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		clientSslProfileName := vars["profileName"]

		_, clientSslProfileFound := f5state.clientSslProfiles[clientSslProfileName]
		if !clientSslProfileFound {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested ClientSSL Profile (/Common/%s) was not found.","errorStack":[]}`,
				clientSslProfileName)
			return
		}

		delete(f5state.clientSslProfiles, clientSslProfileName)

		OK(response)
	}
}

func validateServerKey(response http.ResponseWriter, request *http.Request,
	f5state mockF5State, keyName string) bool {
	_, ok := f5state.keys[keyName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":400,"message":"0107134a:3: File object by name (%s) is missing.","errorStack":[]}`,
			keyName)
		return false
	}

	return true
}

func postServerSslProfileHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			CertificateName string `json:"chain"`
			Name            string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		certificateName := payload.CertificateName
		serverSslProfileName := payload.Name

		// Complain about name collision first because F5 does.
		_, serverSslProfileAlreadyExists := f5state.serverSslProfiles[serverSslProfileName]
		if serverSslProfileAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"message":"01020066:3: The requested ServerSSL Profile (/Common/%s) already exists in partition Common.","errorStack":[]}`,
				serverSslProfileName)
			return
		}

		_, clientSslProfileAlreadyExists := f5state.clientSslProfiles[serverSslProfileName]
		if clientSslProfileAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":400,"message":"01070293:3: The profile name (/Common/%s) is already assigned to another profile.","errorStack":[]}`,
				serverSslProfileName)
			return
		}

		if !validateCert(response, request, f5state, certificateName) {
			return
		}

		f5state.serverSslProfiles[serverSslProfileName] = true

		OK(response)
	}
}

func deleteServerSslProfileHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		serverSslProfileName := vars["profileName"]

		_, serverSslProfileFound := f5state.serverSslProfiles[serverSslProfileName]
		if !serverSslProfileFound {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested ServerSSL Profile (/Common/%s) was not found.","errorStack":[]}`,
				serverSslProfileName)
			return
		}

		delete(f5state.serverSslProfiles, serverSslProfileName)

		OK(response)
	}
}

func associateProfileWithVserver(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserverName := vars["vserverName"]

		if !validateVserverName(response, request, f5state, vserverName) {
			return
		}

		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		profileName := payload.Name

		f5state.vserverProfiles[vserverName][profileName] = true

		OK(response)
	}
}

func deleteSslVserverProfileHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserverName := vars["vserverName"]
		profileName := vars["profileName"]

		if !validateVserverName(response, request, f5state, vserverName) {
			return
		}

		delete(f5state.vserverProfiles[vserverName], profileName)

		OK(response)
	}
}

func deleteSslKeyHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		keyName := vars["keyName"]

		_, keyFound := f5state.keys[keyName]
		if !keyFound {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested Certificate Key File (/Common/%s) was not found.","errorStack":[]}`,
				keyName)
			return
		}

		// TODO: Validate that the key is not in use (which will require keeping
		// track of more state).
		//{"code":400,"message":"01071349:3: File object by name (/Common/%s) is in use.","errorStack":[]}

		delete(f5state.keys, keyName)

		OK(response)
	}
}

func deleteSslCertHandler(f5state mockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		certName := vars["certName"]

		_, certFound := f5state.certs[certName]
		if !certFound {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested Certificate File (/Common/%s) was not found.","errorStack":[]}`,
				certName)
			return
		}

		// TODO: Validate that the key is not in use (which will require keeping
		// track of more state).
		//{"code":400,"message":"01071349:3: File object by name (/Common/openshift_route_default_route-reencrypt-https-cert.crt) is in use.","errorStack":[]}

		delete(f5state.certs, certName)

		OK(response)
	}
}

// TestInitializeF5Plugin initializes the F5 plug-in with a mock unconfigured F5
// BIG-IP host and validates the configuration of the F5 BIG-IP host after the
// plug-in has performed its initialization.
func TestInitializeF5Plugin(t *testing.T) {
	router, mockF5, err := newTestRouter()
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}

	// The policy for secure routes and the policy for insecure routes should
	// exist.
	expectedPolicies := []string{insecureRoutesPolicyName, secureRoutesPolicyName}
	for _, policyName := range expectedPolicies {
		_, ok := mockF5.state.policies[policyName]
		if !ok {
			t.Errorf("%s policy was not created; policies map: %v",
				policyName, mockF5.state.policies)
		}
	}

	// The HTTPS vserver should have the policy for secure routes associated.
	foundSecureRoutesPolicy := false
	for policyName := range mockF5.state.vserverPolicies[httpsVserverName] {
		if policyName == secureRoutesPolicyName {
			foundSecureRoutesPolicy = true
		} else {
			t.Errorf("Encountered unexpected policy associated to vserver %s: %s",
				httpsVserverName, policyName)
		}
	}
	if !foundSecureRoutesPolicy {
		t.Errorf("%s policy was not associated with vserver %s.",
			secureRoutesPolicyName, httpsVserverName)
	}

	// The HTTP vserver should have the policy for insecure routes associated.
	foundInsecureRoutesPolicy := false
	for policyName := range mockF5.state.vserverPolicies[httpVserverName] {
		if policyName == insecureRoutesPolicyName {
			foundInsecureRoutesPolicy = true
		} else {
			t.Errorf("Encountered unexpected policy associated to vserver %s: %s",
				httpVserverName, policyName)
		}
	}
	if !foundInsecureRoutesPolicy {
		t.Errorf("%s policy was not associated with vserver %s.",
			insecureRoutesPolicyName, httpVserverName)
	}

	// The datagroup for passthrough routes should exist.
	foundPassthroughIRuleDatagroup := false
	for datagroupName := range mockF5.state.datagroups {
		if datagroupName == passthroughIRuleDatagroupName {
			foundPassthroughIRuleDatagroup = true
		}
	}
	if !foundPassthroughIRuleDatagroup {
		t.Errorf("%s datagroup was not created.", passthroughIRuleDatagroupName)
	}

	// The passthrough iRule should exist and should reference the datagroup for
	// passthrough routes.
	foundPassthroughIRule := false
	for iRuleName, iRuleCode := range mockF5.state.iRules {
		if iRuleName == passthroughIRuleName {
			foundPassthroughIRule = true

			if !strings.Contains(string(iRuleCode), passthroughIRuleDatagroupName) {
				t.Errorf("iRule for passthrough routes exists, but its body does not"+
					" reference the datagroup for passthrough routes.\n"+
					"iRule name: %s\nDatagroup name: %s\niRule code: %s",
					iRuleName, passthroughIRuleDatagroupName, iRuleCode)
			}
		} else {
			t.Errorf("Encountered unexpected iRule: %s", iRuleName)
		}
	}
	if !foundPassthroughIRule {
		t.Errorf("%s iRule was not created.", passthroughIRuleName)
	}

	// The HTTPS vserver should have the passthrough iRule associated.
	foundPassthroughIRuleUnderVserver := false
	for _, iRuleName := range mockF5.state.vserverIRules[httpsVserverName] {
		if iRuleName == passthroughIRuleName {
			foundPassthroughIRuleUnderVserver = true
		} else {
			t.Errorf("Encountered unexpected iRule associated with vserver %s: %s",
				httpsVserverName, iRuleName)
		}
	}
	if !foundPassthroughIRuleUnderVserver {
		t.Errorf("%s iRule was not associated with vserver %s.",
			passthroughIRuleName, httpsVserverName)
	}

	// The HTTP vserver should have no iRules associated.
	if len(mockF5.state.vserverIRules[httpVserverName]) != 0 {
		t.Errorf("Vserver %s has iRules associated: %v",
			httpVserverName, mockF5.state.vserverIRules[httpVserverName])
	}

	// Initialization should be idempotent.
	// Warning: This is off-label use of DeepCopy!
	savedMockF5State, err := kapi.Scheme.DeepCopy(mockF5.state)
	if err != nil {
		t.Errorf("Failed to deepcopy mock F5 state for idempotency check: %v", err)
	}

	router.F5Client.Initialize()

	if !reflect.DeepEqual(savedMockF5State, mockF5.state) {
		t.Errorf("Initialize method should be idempotent but it is not.\n"+
			"State after first initialization: %v\n"+
			"State after second initialization: %v\n",
			savedMockF5State, mockF5.state)
	}
}

// TestHandleEndpoints tests endpoint watch events and validates that the state
// of the F5 client object is as expected after each event.
func TestHandleEndpoints(t *testing.T) {
	router, mockF5, err := newTestRouter()
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}
	defer mockF5.close()

	testCases := []struct {
		// name is a human readable name for the test case.
		name string

		// type is the type to be passed to the HandleEndpoints method.
		eventType watch.EventType

		// endpoints is the set of endpoints to be passed to the HandleEndpoints
		// method.
		endpoints *kapi.Endpoints

		// validate checks the state of the F5Plugin object and returns a Boolean
		// indicating whether the state is as expected.
		validate func() error
	}{
		{
			name:      "Endpoint add",
			eventType: watch.Added,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "1.1.1.1"}},
					Ports:     []kapi.EndpointPort{{Port: 345}},
				}}, // Specifying no port implies port 80.
			},
			validate: func() error {
				poolName := "openshift_foo_test"

				poolExists, err := router.F5Client.PoolExists(poolName)
				if err != nil {
					return fmt.Errorf("PoolExists(%s) failed: %v", poolName, err)
				}

				if poolExists != true {
					return fmt.Errorf("PoolExists(%s) returned %v instead of true",
						poolName, poolExists)
				}

				memberName := "1.1.1.1:345"

				poolHasMember, err := router.F5Client.PoolHasMember(poolName, memberName)
				if err != nil {
					return fmt.Errorf("PoolHasMember(%s, %s) failed: %v",
						poolName, memberName, err)
				}

				if poolHasMember != true {
					return fmt.Errorf("PoolHasMember(%s, %s) returned %v instead of true",
						poolName, memberName, poolHasMember)
				}

				return nil
			},
		},
		{
			name:      "Endpoint modify",
			eventType: watch.Modified,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{{
					Addresses: []kapi.EndpointAddress{{IP: "2.2.2.2"}},
					Ports:     []kapi.EndpointPort{{Port: 8080}},
				}},
			},
			validate: func() error {
				poolName := "openshift_foo_test"

				poolExists, err := router.F5Client.PoolExists(poolName)
				if err != nil {
					return fmt.Errorf("PoolExists(%s) failed: %v", poolName, err)
				}

				if poolExists != true {
					return fmt.Errorf("PoolExists(%s) returned %v instead of true",
						poolName, poolExists)
				}

				memberName := "2.2.2.2:8080"

				poolHasMember, err := router.F5Client.PoolHasMember(poolName, memberName)
				if err != nil {
					return fmt.Errorf("PoolHasMember(%s, %s) failed: %v",
						poolName, memberName, err)
				}

				if poolHasMember != true {
					return fmt.Errorf("PoolHasMember(%s, %s) returned %v instead of true",
						poolName, memberName, poolHasMember)
				}

				return nil
			},
		},
		{
			name:      "Endpoint delete",
			eventType: watch.Modified,
			endpoints: &kapi.Endpoints{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "test",
				},
				Subsets: []kapi.EndpointSubset{},
			},
			validate: func() error {
				poolName := "openshift_foo_test"

				poolExists, err := router.F5Client.PoolExists(poolName)
				if err != nil {
					return fmt.Errorf("PoolExists(%s) failed: %v", poolName, err)
				}

				if poolExists != false {
					return fmt.Errorf("PoolExists(%s) returned %v instead of false",
						poolName, poolExists)
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		router.HandleEndpoints(tc.eventType, tc.endpoints)

		err := tc.validate()
		if err != nil {
			t.Errorf("Test case %s failed: %v", tc.name, err)
		}
	}
}

// TestHandleRoute test route watch events and validates that the state of the
// F5 client object is as expected after each event.
func TestHandleRoute(t *testing.T) {
	router, mockF5, err := newTestRouter()
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}
	defer mockF5.close()

	type testCase struct {
		// name is a human readable name for the test case.
		name string

		// type is the type to be passed to the HandleRoute method.
		eventType watch.EventType

		// route specifies the route object to be passed to the
		// HandleRoute method.
		route *routeapi.Route

		// validate checks the state of the F5Plugin
		// object and returns a Boolean
		// indicating whether the state is as
		// expected.
		validate func(tc testCase) error
	}

	testCases := []testCase{
		{
			name:      "Unsecure route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "unsecuretest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				rule, ok := mockF5.state.policies[insecureRoutesPolicyName][rulename]
				if !ok {
					return fmt.Errorf("Policy %s should have rule %s,"+
						" but no rule was found: %v",
						insecureRoutesPolicyName, rulename,
						mockF5.state.policies[insecureRoutesPolicyName])
				}

				if len(rule.conditions) != 1 {
					return fmt.Errorf("Insecure route should have rule with 1 condition,"+
						" but rule has %d conditions: %v",
						len(rule.conditions), rule.conditions)
				}

				condition := rule.conditions[0]

				if !(condition.HttpHost && condition.Host && !condition.HttpUri &&
					!condition.PathSegment) {
					return fmt.Errorf("Insecure route should have rule that matches on"+
						" hostname but found this instead: %v", rule.conditions)
				}

				if len(condition.Values) != 1 {
					return fmt.Errorf("Insecure route rule condition should have 1 value"+
						" but has %d values: %v", len(condition.Values), condition)
				}

				if condition.Values[0] != tc.route.Spec.Host {
					return fmt.Errorf("Insecure route rule condition should match on"+
						" hostname %s, but it has a different value: %v",
						tc.route.Spec.Host, condition)
				}

				return nil
			},
		},
		{
			name:      "Unsecure route modify",
			eventType: watch.Modified,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "unsecuretest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example2.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					Path: "/foo/bar",
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				rule, ok := mockF5.state.policies[insecureRoutesPolicyName][rulename]
				if !ok {
					return fmt.Errorf("Policy %s should have rule %s,"+
						" but no rule was found: %v",
						insecureRoutesPolicyName, rulename,
						mockF5.state.policies[insecureRoutesPolicyName])
				}
				if len(rule.conditions) != 3 {
					return fmt.Errorf("Insecure route with pathname should have rule"+
						" with 3 conditions, but rule has %d conditions: %v",
						len(rule.conditions), rule.conditions)
				}

				pathSegments := strings.Split(tc.route.Spec.Path, "/")

				for _, condition := range rule.conditions {
					if !(condition.PathSegment && condition.HttpUri) {
						continue
					}

					expectedValue := pathSegments[condition.Index]
					foundValue := condition.Values[0]

					if foundValue != expectedValue {
						return fmt.Errorf("Rule condition with index %d for insecure route"+
							" with pathname %s should have value \"%s\" but has value"+
							" \"%s\": %v",
							condition.Index, tc.route.Spec.Path, expectedValue, foundValue, rule)
					}
				}

				return nil
			},
		},
		{
			name:      "Unsecure route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "unsecuretest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example2.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				_, found := mockF5.state.policies[secureRoutesPolicyName][rulename]
				if found {
					return fmt.Errorf("Rule %s should have been deleted from policy %s"+
						" when the corresponding route was deleted, but it remains yet: %v",
						rulename, secureRoutesPolicyName,
						mockF5.state.policies[secureRoutesPolicyName])
				}

				return nil
			},
		},
		{
			name:      "Edge route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "edgetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationEdge,
						Certificate: "abc",
						Key:         "def",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				_, found := mockF5.state.policies[secureRoutesPolicyName][rulename]
				if !found {
					return fmt.Errorf("Policy %s should have rule %s,"+
						" but no such rule was found: %v",
						secureRoutesPolicyName, rulename,
						mockF5.state.policies[secureRoutesPolicyName])
				}

				certfname := fmt.Sprintf("%s-https-cert.crt", rulename)
				_, found = mockF5.state.certs[certfname]
				if !found {
					return fmt.Errorf("Certificate file %s should have been created but"+
						" does not exist: %v",
						certfname, mockF5.state.certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.keys[keyfname]
				if !found {
					return fmt.Errorf("Key file %s should have been created but"+
						" does not exist: %v",
						keyfname, mockF5.state.keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.clientSslProfiles[clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been created"+
						" but does not exist: %v",
						clientSslProfileName, mockF5.state.clientSslProfiles)
				}

				_, found = mockF5.state.vserverProfiles[httpsVserverName][clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been"+
						" associated with the vserver but was not: %v",
						clientSslProfileName,
						mockF5.state.vserverProfiles[httpsVserverName])
				}

				return nil
			},
		},
		{
			name:      "Edge route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "edgetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationEdge,
						Certificate: "abc",
						Key:         "def",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				_, found := mockF5.state.policies[secureRoutesPolicyName][rulename]
				if found {
					return fmt.Errorf("Rule %s should have been deleted from policy %s"+
						" when the corresponding route was deleted, but it remains yet: %v",
						rulename, secureRoutesPolicyName,
						mockF5.state.policies[secureRoutesPolicyName])
				}

				certfname := fmt.Sprintf("%s-https-cert.crt", rulename)
				_, found = mockF5.state.certs[certfname]
				if found {
					return fmt.Errorf("Certificate file %s should have been deleted with"+
						" the route but remains yet: %v",
						certfname, mockF5.state.certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.keys[keyfname]
				if found {
					return fmt.Errorf("Key file %s should have been deleted with the"+
						" route but remains yet: %v",
						keyfname, mockF5.state.keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.vserverProfiles[httpsVserverName][clientSslProfileName]
				if found {
					return fmt.Errorf("client-ssl profile %s should have been deleted"+
						" from the vserver when the route was deleted but remains yet: %v",
						clientSslProfileName,
						mockF5.state.vserverProfiles[httpsVserverName])
				}

				_, found = mockF5.state.clientSslProfiles[clientSslProfileName]
				if found {
					return fmt.Errorf("client-ssl profile %s should have been deleted"+
						" with the route but remains yet: %v",
						clientSslProfileName, mockF5.state.clientSslProfiles)
				}

				return nil
			},
		},
		{
			name:      "Passthrough route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "passthroughtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationPassthrough,
					},
				},
			},
			validate: func(tc testCase) error {
				_, found := mockF5.state.datagroups[passthroughIRuleDatagroupName][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("Datagroup entry for %s should have been created"+
						" in the %s datagroup for the passthrough route but cannot be"+
						" found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.datagroups[passthroughIRuleDatagroupName])
				}

				return nil
			},
		},
		{
			name:      "Add route with same hostname as passthrough route",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "conflictingroutetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationEdge,
						Certificate: "abc",
						Key:         "def",
					},
				},
			},
			validate: func(tc testCase) error {
				_, found := mockF5.state.datagroups[passthroughIRuleDatagroupName][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("Datagroup entry for %s should still exist"+
						" in the %s datagroup after a secure route with the same hostname"+
						" was created, but the datagroup entry cannot be found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.datagroups[passthroughIRuleDatagroupName])
				}

				return nil
			},
		},
		{
			name:      "Modify route with same hostname as passthrough route",
			eventType: watch.Modified,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "conflictingroutetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				_, found := mockF5.state.datagroups[passthroughIRuleDatagroupName][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("Datagroup entry for %s should still exist"+
						" in the %s datagroup after a secure route with the same hostname"+
						" was updated, but the datagroup entry cannot be found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.datagroups[passthroughIRuleDatagroupName])
				}

				return nil
			},
		},
		{
			name:      "Delete route with same hostname as passthrough route",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "conflictingroutetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				_, found := mockF5.state.datagroups[passthroughIRuleDatagroupName][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("Datagroup entry for %s should still exist"+
						" in the %s datagroup after a secure route with the same hostname"+
						" was deleted, but the datagroup entry cannot be found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.datagroups[passthroughIRuleDatagroupName])
				}

				return nil
			},
		},
		{
			name:      "Passthrough route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "passthroughtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationPassthrough,
					},
				},
			},
			validate: func(tc testCase) error {
				_, found := mockF5.state.datagroups[passthroughIRuleDatagroupName][tc.route.Spec.Host]
				if found {
					return fmt.Errorf("Datagroup entry for %s should have been deleted"+
						" from the %s datagroup for the passthrough route but remains"+
						" yet: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.datagroups[passthroughIRuleDatagroupName])
				}

				return nil
			},
		},
		{
			name:      "Reencrypted route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "reencryptedtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example4.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationReencrypt,
						Certificate:              "abc",
						Key:                      "def",
						CACertificate:            "ghi",
						DestinationCACertificate: "jkl",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				_, found := mockF5.state.policies[secureRoutesPolicyName][rulename]
				if !found {
					return fmt.Errorf("Policy %s should have rule %s for secure route,"+
						" but no rule was found: %v",
						secureRoutesPolicyName, rulename,
						mockF5.state.policies[secureRoutesPolicyName])
				}

				certcafname := fmt.Sprintf("%s-https-chain.crt", rulename)
				_, found = mockF5.state.certs[certcafname]
				if !found {
					return fmt.Errorf("Certificate chain file %s should have been"+
						" created but does not exist: %v",
						certcafname, mockF5.state.certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.keys[keyfname]
				if !found {
					return fmt.Errorf("Key file %s should have been created but"+
						" does not exist: %v",
						keyfname, mockF5.state.keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.clientSslProfiles[clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been created"+
						" but does not exist: %v",
						clientSslProfileName, mockF5.state.clientSslProfiles)
				}

				_, found = mockF5.state.vserverProfiles[httpsVserverName][clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been"+
						" associated with the vserver but was not: %v",
						clientSslProfileName,
						mockF5.state.vserverProfiles[httpsVserverName])
				}

				serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", rulename)
				_, found = mockF5.state.serverSslProfiles[serverSslProfileName]
				if !found {
					return fmt.Errorf("server-ssl profile %s should have been created"+
						" but does not exist: %v",
						serverSslProfileName, mockF5.state.serverSslProfiles)
				}

				_, found = mockF5.state.vserverProfiles[httpsVserverName][serverSslProfileName]
				if !found {
					return fmt.Errorf("server-ssl profile %s should have been"+
						" associated with the vserver but was not: %v",
						serverSslProfileName,
						mockF5.state.vserverProfiles[httpsVserverName])
				}

				return nil
			},
		},
		{
			name:      "Reencrypted route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "reencryptedtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example4.com",
					To: kapi.ObjectReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination:              routeapi.TLSTerminationReencrypt,
						Certificate:              "abc",
						Key:                      "def",
						CACertificate:            "ghi",
						DestinationCACertificate: "jkl",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)

				_, found := mockF5.state.policies[secureRoutesPolicyName][rulename]
				if found {
					return fmt.Errorf("Rule %s should have been deleted from policy %s"+
						" when the corresponding route was deleted, but it remains yet: %v",
						rulename, secureRoutesPolicyName,
						mockF5.state.policies[secureRoutesPolicyName])
				}

				certcafname := fmt.Sprintf("%s-https-chain.crt", rulename)
				_, found = mockF5.state.certs[certcafname]
				if found {
					return fmt.Errorf("Certificate chain file %s should have been"+
						" deleted with the route but remains yet: %v",
						certcafname, mockF5.state.certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.keys[keyfname]
				if found {
					return fmt.Errorf("Key file %s should have been deleted with the"+
						" route but remains yet: %v",
						keyfname, mockF5.state.keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.vserverProfiles[httpsVserverName][clientSslProfileName]
				if found {
					return fmt.Errorf("client-ssl profile %s should have been deleted"+
						" from the vserver when the route was deleted but remains yet: %v",
						clientSslProfileName,
						mockF5.state.vserverProfiles[httpsVserverName])
				}

				serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", rulename)
				_, found = mockF5.state.vserverProfiles[httpsVserverName][clientSslProfileName]
				if found {
					return fmt.Errorf("server-ssl profile %s should have been deleted"+
						" from the vserver when the route was deleted but remains yet: %v",
						serverSslProfileName,
						mockF5.state.vserverProfiles[httpsVserverName])
				}

				_, found = mockF5.state.serverSslProfiles[serverSslProfileName]
				if found {
					return fmt.Errorf("server-ssl profile %s should have been deleted"+
						" with the route but remains yet: %v",
						serverSslProfileName, mockF5.state.serverSslProfiles)
				}

				return nil
			},
		},
	}

	for _, tc := range testCases {
		router.HandleRoute(tc.eventType, tc.route)

		err := tc.validate(tc)
		if err != nil {
			t.Errorf("Test case %s failed: %v", tc.name, err)
		}
	}
}

// TestHandleRouteModifications creates an F5 router instance, creates
// a service and a route, modifies the route in several ways, and verifies that
// the router correctly updates the route.
func TestHandleRouteModifications(t *testing.T) {
	router, mockF5, err := newTestRouter()
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}
	defer mockF5.close()

	testRoute := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "foo",
			Name:      "mutatingroute",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: kapi.ObjectReference{
				Name: "testendpoint",
			},
		},
	}

	err = router.HandleRoute(watch.Added, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on adding test route: %v", err)
	}

	// Verify that modifying the route into a secure route works.
	testRoute.Spec.TLS = &routeapi.TLSConfig{
		Termination:              routeapi.TLSTerminationReencrypt,
		Certificate:              "abc",
		Key:                      "def",
		CACertificate:            "ghi",
		DestinationCACertificate: "jkl",
	}

	err = router.HandleRoute(watch.Modified, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on modifying test route: %v", err)
	}

	// Verify that updating the hostname of the route succeeds.
	testRoute.Spec.Host = "www.example2.com"

	err = router.HandleRoute(watch.Modified, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on modifying test route: %v", err)
	}

	// Verify that modifying the route into a passthrough route works.
	testRoute.Spec.TLS = &routeapi.TLSConfig{
		Termination: routeapi.TLSTerminationPassthrough,
	}

	err = router.HandleRoute(watch.Modified, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on modifying test route: %v", err)
	}

	// Verify that updating the hostname of the passthrough route succeeds.
	testRoute.Spec.Host = "www.example3.com"

	err = router.HandleRoute(watch.Modified, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on modifying test route: %v", err)
	}
}

// TestF5RouterSuccessiveInstances creates an F5 router instance, creates
// a service and a route, creates a new F5 router instance, and verifies that
// the new instance behaves correctly picking up the state from the first
// instance.
func TestF5RouterSuccessiveInstances(t *testing.T) {
	router, mockF5, err := newTestRouter()
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}

	testRoute := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "xyzzy",
			Name:      "testroute",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: kapi.ObjectReference{
				Name: "testendpoint",
			},
			TLS: &routeapi.TLSConfig{
				Termination:              routeapi.TLSTerminationReencrypt,
				Certificate:              "abc",
				Key:                      "def",
				CACertificate:            "ghi",
				DestinationCACertificate: "jkl",
			},
		},
	}

	testPassthroughRoute := &routeapi.Route{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "quux",
			Name:      "testpassthroughroute",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example2.com",
			To: kapi.ObjectReference{
				Name: "testhttpsendpoint",
			},
			TLS: &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationPassthrough,
			},
		},
	}

	testHttpEndpoint := &kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "xyzzy",
			Name:      "testhttpendpoint",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "10.1.1.1"}},
			Ports:     []kapi.EndpointPort{{Port: 8080}},
		}},
	}

	testHttpsEndpoint := &kapi.Endpoints{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "quux",
			Name:      "testhttpsendpoint",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "10.1.2.1"}},
			Ports:     []kapi.EndpointPort{{Port: 8443}},
		}},
	}

	// Add routes.

	err = router.HandleRoute(watch.Added, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on adding test route: %v", err)
	}

	err = router.HandleRoute(watch.Added, testPassthroughRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on adding test passthrough route: %v", err)
	}

	err = router.HandleEndpoints(watch.Added, testHttpEndpoint)
	if err != nil {
		t.Fatalf("HandleEndpoints failed on adding test HTTP endpoint subset: %v",
			err)
	}

	err = router.HandleEndpoints(watch.Added, testHttpsEndpoint)
	if err != nil {
		t.Fatalf("HandleEndpoints failed on adding test HTTPS endpoint subset: %v",
			err)
	}

	// Initialize a new router, but retain the mock F5 host.
	mockF5.close()
	router, mockF5, err = newTestRouterWithState(mockF5.state)
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}
	defer mockF5.close()

	// Have the new router delete the routes that the old router created.
	err = router.HandleRoute(watch.Deleted, testRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on deleting test route: %v", err)
	}

	err = router.HandleRoute(watch.Deleted, testPassthroughRoute)
	if err != nil {
		t.Fatalf("HandleRoute failed on deleting test passthrough route: %v", err)
	}

	err = router.HandleEndpoints(watch.Deleted, testHttpEndpoint)
	if err != nil {
		t.Fatalf("HandleEndpoints failed on deleting test HTTP endpoint subset: %v",
			err)
	}

	err = router.HandleEndpoints(watch.Deleted, testHttpsEndpoint)
	if err != nil {
		t.Fatalf("HandleEndpoints failed on deleting test HTTPS endpoint subset: %v",
			err)
	}
}

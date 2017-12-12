package f5

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/openshift/origin/pkg/cmd/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
	f5testing "github.com/openshift/origin/pkg/router/f5/testing"
)

type (
	mockF5 struct {
		// state is the internal state of the mock F5 BIG-IP host.
		state f5testing.MockF5State

		// server is the mock F5 BIG-IP host, which accepts HTTPS connections and
		// behaves as an actual F5 BIG-IP host for testing purposes.
		server *httptest.Server
	}

	// An internal mock of an F5 iControl REST API resource.
	mockF5iControlResource struct {
		// Type is the type of the f5 iControl resource.
		Type string

		// Name is the name of the f5 iControl resource.
		Name string

		// FullPath is the full path to the f5 iControl resource.
		FullPath string

		// Partition is the path of the "owning" partition for the f5 iControl resource.
		Partition string
	}
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
	HandlerFunc func(f5testing.MockF5State) http.HandlerFunc
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
	{"getPartition", "GET", "/mgmt/tm/sys/folder/{partitionPath}", getPartitionPath},
	{"postPartition", "POST", "/mgmt/tm/sys/folder", postPartitionPathHandler},
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

func newF5Routes(mockF5State f5testing.MockF5State) *mux.Router {
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

// newTestRouterWithState creates a new F5 plugin with a mock F5 BIG-IP server
// initialized from the given mock F5 state and returns pointers to the plugin
// and mock server.  Note that these pointers will be nil if an error is
// returned.
func newTestRouterWithState(state f5testing.MockF5State, partitionPath string) (*F5Plugin, *mockF5, error) {
	routerLogLevel := util.Env("TEST_ROUTER_LOGLEVEL", "")
	if routerLogLevel != "" {
		flag.Set("v", routerLogLevel)
	}

	execCommand = mockExecCommand

	server := httptest.NewTLSServer(newF5Routes(state))

	url, err := url.Parse(server.URL)
	if err != nil {
		return nil, nil,
			fmt.Errorf("Failed to parse URL of mock F5 host; URL: %s, error: %v",
				url, err)
	}

	f5PluginTestCfg := F5PluginConfig{
		Host:          url.Host,
		Username:      "admin",
		Password:      "password",
		HttpVserver:   httpVserverName,
		HttpsVserver:  httpsVserverName,
		PrivateKey:    "/dev/null",
		Insecure:      true,
		PartitionPath: partitionPath,
	}
	router, err := NewF5Plugin(f5PluginTestCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create new F5 router: %v", err)
	}

	mockF5 := &mockF5{state: state, server: server}

	return router, mockF5, nil
}

// newTestRouter creates a new F5 plugin with a mock F5 BIG-IP server and
// returns pointers to the plugin and mock server.  Note that these pointers
// will be nil if an error is returned.
func newTestRouter(partitionPath string) (*F5Plugin, *mockF5, error) {
	pathKey := strings.Replace(partitionPath, "/", "~", -1)
	httpVserverPath := path.Join(partitionPath, httpVserverName)
	httpsVserverPath := path.Join(partitionPath, httpsVserverName)
	state := f5testing.MockF5State{
		Policies: map[string]map[string]f5testing.PolicyRule{},
		VserverPolicies: map[string]map[string]bool{
			httpVserverPath:  {},
			httpsVserverPath: {},
		},
		Certs:             map[string]bool{},
		Keys:              map[string]bool{},
		ServerSslProfiles: map[string]bool{},
		ClientSslProfiles: map[string]bool{},
		VserverProfiles: map[string]map[string]bool{
			httpsVserverPath: {},
		},
		Datagroups:    map[string]f5testing.Datagroup{},
		IRules:        map[string]f5testing.IRule{},
		VserverIRules: map[string][]string{},

		// Add the default /Common partition path.
		PartitionPaths: map[string]string{pathKey: partitionPath},
		Pools:          map[string]f5testing.Pool{},
	}

	return newTestRouterWithState(state, partitionPath)
}

func (f5 *mockF5) close() {
	f5.server.Close()
}

func normalizeiControlUriPath(pathName string) string {
	return strings.Replace(pathName, "~", "/", -1)
}

func normalizeResourcePath(resourcePath string) string {
	unescapedPath := normalizeiControlUriPath(resourcePath)
	if strings.HasPrefix(unescapedPath, "/") {
		return unescapedPath
	}

	return path.Join(F5DefaultPartitionPath, unescapedPath)
}

func (r *mockF5iControlResource) id() string {
	return r.FullPath
}

func (r *mockF5iControlResource) uriPath() string {
	return encodeiControlUriPathComponent(r.FullPath)
}

func newMockF5iControlResource(resourceType, resourceName string) *mockF5iControlResource {
	resourcePath := normalizeResourcePath(resourceName)
	resourcePartitionPath, _ := path.Split(resourcePath)

	return &mockF5iControlResource{
		Type:      resourceType,
		Name:      resourceName,
		FullPath:  resourcePath,
		Partition: resourcePartitionPath,
	}
}

func validatePolicy(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, policy *mockF5iControlResource) bool {
	_, ok := f5state.Policies[policy.id()]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested Policy (%s) was not found."}`,
			policy.FullPath)
		return false
	}

	return true
}

func getPolicyHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policy := newMockF5iControlResource("policy", vars["policyName"])

		if !validatePolicy(response, request, f5state, policy) {
			return
		}

		policyUriPath := policy.uriPath()
		fmt.Fprintf(response,
			`{"controls":["forwarding"],"fullPath":"%s","generation":1,"kind":"tm:ltm:policy:policystate","name":"%s","requires":["http"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/%s/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/%s?ver=11.6.0","strategy":"/Common/best-match"}`,
			policy.FullPath, policy.Name, policyUriPath, policyUriPath)
	}
}

func OK(response http.ResponseWriter) {
	fmt.Fprint(response, `{"code":200,"message":"OK"}`)
}

func postPolicyHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name      string `json:"name"`
			Partition string `json:"partition"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		policy := newMockF5iControlResource("policy", payload.Name)

		f5state.Policies[policy.id()] = map[string]f5testing.PolicyRule{}

		OK(response)
	}
}

func postRuleHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policy := newMockF5iControlResource("policy", vars["policyName"])

		if !validatePolicy(response, request, f5state, policy) {
			return
		}

		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		ruleName := payload.Name

		newRule := f5testing.PolicyRule{[]f5testing.PolicyCondition{}}
		f5state.Policies[policy.id()][ruleName] = newRule

		OK(response)
	}
}

func validateVserver(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, vserver *mockF5iControlResource) bool {
	if !recogniseVserver(vserver) {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested Virtual Server (%s) was not found."}`,
			vserver.FullPath)
		return false
	}

	return true
}

func getPoliciesHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserver := newMockF5iControlResource("vserver", vars["vserverName"])

		if !validateVserver(response, request, f5state, vserver) {
			return
		}

		fmt.Fprint(response, `{"items":[{"controls":["classification"],"fullPath":"/Common/_sys_CEC_SSL_client_policy","generation":1,"hints":["no-write","no-delete","no-exclusion"],"kind":"tm:ltm:policy:policystate","name":"_sys_CEC_SSL_client_policy","partition":"Common","requires":["ssl-persistence"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_client_policy/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_client_policy?ver=11.6.0","strategy":"/Common/first-match"},{"controls":["classification"],"fullPath":"/Common/_sys_CEC_SSL_server_policy","generation":1,"hints":["no-write","no-delete","no-exclusion"],"kind":"tm:ltm:policy:policystate","name":"_sys_CEC_SSL_server_policy","partition":"Common","requires":["ssl-persistence"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_server_policy/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_SSL_server_policy?ver=11.6.0","strategy":"/Common/first-match"},{"controls":["classification"],"fullPath":"/Common/_sys_CEC_video_policy","generation":1,"hints":["no-write","no-delete","no-exclusion"],"kind":"tm:ltm:policy:policystate","name":"_sys_CEC_video_policy","partition":"Common","requires":["http"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_video_policy/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/~Common~_sys_CEC_video_policy?ver=11.6.0","strategy":"/Common/first-match"}`)
		for policyName := range f5state.VserverPolicies[vserver.id()] {
			policy := newMockF5iControlResource("policy", policyName)
			policyUriPath := policy.uriPath()
			fmt.Fprintf(response,
				`,{"controls":["forwarding"],"fullPath":"%s","generation":1,"kind":"tm:ltm:policy:policystate","name":"%s","partition":"%s","requires":["http"],"rulesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/%s/rules?ver=11.6.0"},"selfLink":"https://localhost/mgmt/tm/ltm/policy/%s?ver=11.6.0","strategy":"/Common/best-match"}`,
				policy.FullPath, policy.Name, policy.Partition,
				policyUriPath, policyUriPath)
		}

		fmt.Fprintf(response, `],"kind":"tm:ltm:policy:policycollectionstate","selfLink":"https://localhost/mgmt/tm/ltm/policy?ver=11.6.0"}`)
	}
}

func recogniseVserver(vserver *mockF5iControlResource) bool {
	isHttpVserver := strings.HasSuffix(vserver.FullPath, httpVserverName)
	isHttpsVserver := strings.HasSuffix(vserver.FullPath, httpsVserverName)
	return isHttpVserver || isHttpsVserver
}

func associatePolicyWithVserverHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserver := newMockF5iControlResource("vserver", vars["vserverName"])

		payload := struct {
			Name      string `json:"name"`
			Partition string `json:"partition"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		policy := newMockF5iControlResource("policy", payload.Name)

		validVserver := recogniseVserver(vserver)
		_, validPolicy := f5state.Policies[policy.id()]

		if !validVserver || !validPolicy {
			response.WriteHeader(http.StatusNotFound)
		}

		if !validVserver && !validPolicy {
			fmt.Fprintf(response,
				`{"code":400,"errorStack":[],"message":"01070712:3: Values (%s) specified for virtual server policy (%s %s): foreign key index (policy_FK) do not point at an item that exists in the database."}`,
				policy.FullPath, vserver.FullPath, policy.FullPath)
			return
		}

		if !validVserver {
			fmt.Fprintf(response,
				`{"code":400,"errorStack":[],"message":"01070712:3: Values (%s) specified for virtual server policy (%s %s): foreign key index (vs_FK) do not point at an item that exists in the database."}`,
				vserver.FullPath, vserver.FullPath, policy.FullPath)
			return
		}

		if !validPolicy {
			fmt.Fprintf(response,
				`{"code":404,"errorStack":[],"message":"01020036:3: The requested policy (%s) was not found."}`,
				policy.FullPath)
			return
		}

		if _, found := f5state.VserverPolicies[vserver.id()]; !found {
			fmt.Fprintf(response,
				`{"code":400,"errorStack":[],"message":"01070712:3: Values (%s) specified for virtual server policy (%s %s): foreign key index (vs_FK) do not point at an item that exists in the database."}`,
				vserver.FullPath, vserver.FullPath, policy.FullPath)
			return
		}

		f5state.VserverPolicies[vserver.id()][policy.id()] = true

		OK(response)
	}
}

func validateDatagroup(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, datagroupResource *mockF5iControlResource) bool {
	_, ok := f5state.Datagroups[datagroupResource.id()]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested value list (%s) was not found."}`,
			datagroupResource.FullPath)
		return false
	}

	return true
}

func getDatagroupHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		datagroupResource := newMockF5iControlResource("datagroup", vars["datagroupName"])

		if !validateDatagroup(response, request, f5state, datagroupResource) {
			return
		}

		datagroup := f5state.Datagroups[datagroupResource.id()]

		fmt.Fprintf(response,
			`{"fullPath":"%s","generation":1556,"kind":"tm:ltm:data-group:internal:internalstate","name":"%s","records":[`,
			datagroupResource.FullPath, datagroupResource.Name)

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
			datagroupResource.uriPath())
	}
}

func patchDatagroupHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		datagroupResource := newMockF5iControlResource("datagroup", vars["datagroupName"])

		if !validateDatagroup(response, request, f5state, datagroupResource) {
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

		dg := f5testing.Datagroup{}
		for _, record := range payload.Records {
			dg[record.Key] = record.Value
		}

		f5state.Datagroups[datagroupResource.id()] = dg

		OK(response)
	}
}

func postDatagroupHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		datagroupResource := newMockF5iControlResource("datagroup", payload.Name)

		_, datagroupAlreadyExists := f5state.Datagroups[datagroupResource.id()]
		if datagroupAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"errorStack":[],"message":"01020066:3: The requested value list (%s) already exists in partition Common."}`,
				datagroupResource.FullPath)
			return
		}

		f5state.Datagroups[datagroupResource.id()] = map[string]string{}

		OK(response)
	}
}

func getIRuleHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		rule := newMockF5iControlResource("irule", vars["iRuleName"])

		iRuleCode, ok := f5state.IRules[rule.id()]
		if !ok {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"errorStack":[],"message":"01020036:3: The requested iRule (%s) was not found."}`,
				rule.FullPath)
			return
		}

		fmt.Fprintf(response,
			`{"apiAnonymous":"%s","fullPath":"%s","generation":386,"kind":"tm:ltm:rule:rulestate","name":"%s","selfLink":"https://localhost/mgmt/tm/ltm/rule/%s?ver=11.6.0"}`,
			iRuleCode, rule.FullPath, rule.Name, rule.uriPath())
	}
}

func postIRuleHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
			Code string `json:"apiAnonymous"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		rule := newMockF5iControlResource("irule", payload.Name)
		iRuleCode := payload.Code

		_, iRuleAlreadyExists := f5state.IRules[rule.id()]
		if iRuleAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"errorStack":[],"message":"01020066:3: The requested iRule (%s) already exists in partition %s."}`,
				rule.FullPath, rule.Partition)
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

		f5state.IRules[rule.id()] = f5testing.IRule(iRuleCode)

		OK(response)
	}
}

func getVserverHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserver := newMockF5iControlResource("vserver", vars["vserverName"])

		if !validateVserver(response, request, f5state, vserver) {
			return
		}

		description := "OpenShift Enterprise Virtual Server for HTTPS connections"
		destination := "10.1.1.1:443"

		if strings.HasSuffix(vserver.FullPath, httpVserverName) {
			description = "OpenShift Enterprise Virtual Server for HTTP connections"
			destination = "10.1.1.2:80"
		}

		vserverUriPath := vserver.uriPath()
		fmt.Fprintf(response,
			`{"addressStatus":"yes","autoLasthop":"default","cmpEnabled":"yes","connectionLimit":0,"description":"%s","destination":"%s/%s","enabled":true,"fullPath":"%s","generation":387,"gtmScore":0,"ipProtocol":"tcp","kind":"tm:ltm:virtual:virtualstate","mask":"255.255.255.255","mirror":"disabled","mobileAppTunnel":"disabled","name":"%s","nat64":"disabled","policiesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/virtual/%s/policies?ver=11.6.0"},"profilesReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/virtual/%s/profiles?ver=11.6.0"},"rateLimit":"disabled","rateLimitDstMask":0,"rateLimitMode":"object","rateLimitSrcMask":0,"rules":[`,
			description, vserver.Partition, destination, vserver.FullPath,
			vserver.Name, vserverUriPath, vserverUriPath)

		first := true
		for _, ruleName := range f5state.VserverIRules[vserver.id()] {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			fmt.Fprintf(response, `"%s"`, ruleName)
		}

		fmt.Fprintf(response, `],"selfLink":"https://localhost/mgmt/tm/ltm/virtual/%s?ver=11.6.0","source":"0.0.0.0/0","sourceAddressTranslation":{"type":"none"},"sourcePort":"preserve","synCookieStatus":"not-activated","translateAddress":"enabled","translatePort":"enabled","vlansDisabled":true,"vsIndex":11}`,
			vserverUriPath)
	}
}

func patchVserverHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserver := newMockF5iControlResource("policy", vars["vserverName"])

		if !validateVserver(response, request, f5state, vserver) {
			return
		}

		payload := struct {
			Rules []string `json:"rules"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		iRules := []string(payload.Rules)

		f5state.VserverIRules[vserver.id()] = iRules

		OK(response)
	}
}

func validatePartition(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, partition *mockF5iControlResource) bool {
	_, ok := f5state.PartitionPaths[partition.id()]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3: The requested folder (%s) was not found."}`,
			partition.FullPath)
		return false
	}

	return true
}

func getPartitionPath(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		partition := newMockF5iControlResource("partition", vars["partitionPath"])

		if !validatePartition(response, request, f5state, partition) {
			return
		}

		fullPath := f5state.PartitionPaths[partition.id()]
		fmt.Fprintf(response,
			`{"deviceGroup":"%s/ose-sync-failover","fullPath":"%s","generation":580,"hidden":"false","inheritedDevicegroup":"true","inheritedTrafficGroup":"true","kind":"tm:sys:folder:folderstate","name":"%s","noRefCheck":"false","selfLink":"https://localhost/mgmt/tm/sys/folder/%s?ver=11.6.0","subPath":"/"}`,
			fullPath, fullPath, partition.Name, encodeiControlUriPathComponent(fullPath))
	}
}

func postPartitionPathHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		partition := newMockF5iControlResource("partition", payload.Name)

		f5state.PartitionPaths[partition.id()] = partition.FullPath

		OK(response)
	}
}

func postPoolHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		poolResource := newMockF5iControlResource("pool", payload.Name)

		_, poolAlreadyExists := f5state.Pools[poolResource.id()]
		if poolAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"errorStack":[],"message":"01020066:3: The requested f5testing.Pool (%s) already exists in partition %s."}`,
				poolResource.FullPath, poolResource.Partition)
			return
		}

		f5state.Pools[poolResource.id()] = f5testing.Pool{}

		OK(response)
	}
}

func validatePool(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, poolResource *mockF5iControlResource) bool {
	_, ok := f5state.Pools[poolResource.id()]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":404,"errorStack":[],"message":"01020036:3:The requested f5testing.Pool (%s) was not found."}`,
			poolResource.FullPath)
		return false
	}

	return true
}

func deletePoolHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolResource := newMockF5iControlResource("pool", vars["poolName"])

		if !validatePool(response, request, f5state, poolResource) {
			return
		}

		// TODO: Validate that no rule references the pool.

		delete(f5state.Pools, poolResource.id())

		OK(response)
	}
}

func getPoolMembersHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolResource := newMockF5iControlResource("pool", vars["poolName"])

		if !validatePool(response, request, f5state, poolResource) {
			return
		}

		fmt.Fprint(response, `{"items":[`)

		first := true
		for member := range f5state.Pools[poolResource.id()] {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			addr := strings.Split(member, ":")[0]
			fmt.Fprintf(response,
				`{"address":"%s","connectionLimit":0,"dynamicRatio":1,"ephemeral":"false","fqdn":{"autopopulate":"disabled"},"fullPath":"%s","generation":1190,"inheritProfile":"enabled","kind":"tm:ltm:pool:members:membersstate","logging":"disabled","monitor":"default","name":"%s","partition":"Common","priorityGroup":0,"rateLimit":"disabled","ratio":1,"selfLink":"https://localhost/mgmt/tm/ltm/pool/%s/members/%s?ver=11.6.0","session":"monitor-enabled","state":"up"}`,
				addr, member, member, member, member)
		}

		fmt.Fprintf(response,
			`],"kind":"tm:ltm:pool:members:memberscollectionstate","selfLink":"https://localhost/mgmt/tm/ltm/pool/%s/members?ver=11.6.0"}`,
			poolResource.uriPath())
	}
}

func postPoolMemberHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolResource := newMockF5iControlResource("pool", vars["poolName"])

		if !validatePool(response, request, f5state, poolResource) {
			return
		}

		payload := struct {
			Member string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		memberName := payload.Member

		_, memberAlreadyExists := f5state.Pools[poolResource.id()][memberName]
		if memberAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"message":"01020066:3: The requested f5testing.Pool Member (%s %s) already exists in partition %s.","errorStack":[]}`,
				poolResource.FullPath, strings.Replace(memberName, ":", " ", 1), poolResource.Partition)
			return
		}

		f5state.Pools[poolResource.id()][memberName] = true

		OK(response)
	}
}

func deletePoolMemberHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		poolResource := newMockF5iControlResource("policy", vars["poolName"])
		memberName := vars["memberName"]

		if !validatePool(response, request, f5state, poolResource) {
			return
		}

		_, foundMember := f5state.Pools[poolResource.id()][memberName]
		if !foundMember {
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested f5testing.Pool Member (%s %s) was not found.","errorStack":[]}`,
				poolResource.FullPath, strings.Replace(memberName, ":", " ", 1))
		}

		delete(f5state.Pools[poolResource.id()], memberName)

		OK(response)
	}
}

func getRulesHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policy := newMockF5iControlResource("policy", vars["policyName"])

		if !validatePolicy(response, request, f5state, policy) {
			return
		}

		fmt.Fprint(response, `{"items": [`)

		policyUriPath := policy.uriPath()
		first := true
		for ruleName := range f5state.Policies[policy.id()] {
			if first {
				first = false
			} else {
				fmt.Fprintf(response, ",")
			}

			ruleUriPath := encodeiControlUriPathComponent(ruleName)
			fmt.Fprintf(response,
				`{"actionsReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/%s/rules/%s/actions?ver=11.6.0"},"conditionsReference":{"isSubcollection":true,"link":"https://localhost/mgmt/tm/ltm/policy/%s/rules/%s/conditions?ver=11.6.0"},"fullPath":"%s","generation":1218,"kind":"tm:ltm:policy:rules:rulesstate","name":"%s","ordinal":0,"selfLink":"https://localhost/mgmt/tm/ltm/policy/%s/rules/%s?ver=11.6.0"}`,
				policyUriPath, ruleUriPath,
				policyUriPath, ruleUriPath,
				ruleName, ruleName,
				policyUriPath, ruleUriPath)
		}

		fmt.Fprintf(response,
			`],"kind":"tm:ltm:policy:rules:rulescollectionstate","selfLink":"https://localhost/mgmt/tm/ltm/policy/%s/rules?ver=11.6.0"}`,
			policyUriPath)
	}
}

func validateRuleName(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, policy *mockF5iControlResource, ruleName string) bool {
	for rule := range f5state.Policies[policy.id()] {
		if rule == ruleName {
			return true
		}
	}

	response.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(response,
		`{"code":404,"errorStack":[],"message":"01020036:3: The requested policy rule (%s %s) was not found."}`,
		policy.FullPath, ruleName)
	return false
}

func postConditionHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policy := newMockF5iControlResource("policy", vars["policyName"])
		ruleName := vars["ruleName"]

		if !validatePolicy(response, request, f5state, policy) {
			return
		}

		foundRule := validateRuleName(response, request, f5state,
			policy, ruleName)
		if !foundRule {
			return
		}

		payload := f5testing.PolicyCondition{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		// TODO: Validate more fields in the payload: equals, request, maybe others.

		conditions := f5state.Policies[policy.id()][ruleName].Conditions
		conditions = append(conditions, payload)
		f5state.Policies[policy.id()][ruleName] = f5testing.PolicyRule{conditions}

		OK(response)
	}
}

func postActionHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policy := newMockF5iControlResource("policy", vars["policyName"])
		ruleName := vars["ruleName"]

		if !validatePolicy(response, request, f5state, policy) {
			return
		}

		foundRule := validateRuleName(response, request, f5state,
			policy, ruleName)
		if !foundRule {
			return
		}

		// TODO: Validate payload.

		OK(response)
	}
}

func deleteRuleHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		policy := newMockF5iControlResource("policy", vars["policyName"])
		ruleName := vars["ruleName"]

		if !validatePolicy(response, request, f5state, policy) {
			return
		}

		foundRule := validateRuleName(response, request, f5state,
			policy, ruleName)
		if !foundRule {
			return
		}

		delete(f5state.Policies[policy.id()], ruleName)

		OK(response)
	}
}

func postSslCertHandler(f5state f5testing.MockF5State) http.HandlerFunc {
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
		f5state.Certs[fmt.Sprintf("%s.crt", certName)] = true

		OK(response)
	}
}

func postSslKeyHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		keyName := payload.Name

		// TODO: Validate filename (which would require more elaborate mocking of
		// the ssh and scp commands in mockExecCommand).

		f5state.Keys[fmt.Sprintf("%s.key", keyName)] = true

		OK(response)
	}
}

func validateClientKey(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, keyName string) bool {
	_, ok := f5state.Keys[keyName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":400,"message":"010717e3:3: Client SSL profile must have RSA certificate/key pair.","errorStack":[]}`)
		return false
	}

	return true
}

func validateCert(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, certName string) bool {
	_, ok := f5state.Certs[certName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":400,"message":"0107134a:3: File object by name (%s) is missing.","errorStack":[]}`,
			certName)
		return false
	}

	return true
}

func postClientSslProfileHandler(f5state f5testing.MockF5State) http.HandlerFunc {
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
		_, clientSslProfileAlreadyExists := f5state.ClientSslProfiles[clientSslProfileName]
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
		_, serverSslProfileAlreadyExists := f5state.ServerSslProfiles[clientSslProfileName]
		if serverSslProfileAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":400,"message":"01070293:3: The profile name (/Common/%s) is already assigned to another profile.","errorStack":[]}`,
				clientSslProfileName)
			return
		}

		f5state.ClientSslProfiles[clientSslProfileName] = true

		OK(response)
	}
}

func deleteClientSslProfileHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		clientSslProfileName := vars["profileName"]

		_, clientSslProfileFound := f5state.ClientSslProfiles[clientSslProfileName]
		if !clientSslProfileFound {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested ClientSSL Profile (/Common/%s) was not found.","errorStack":[]}`,
				clientSslProfileName)
			return
		}

		delete(f5state.ClientSslProfiles, clientSslProfileName)

		OK(response)
	}
}

func validateServerKey(response http.ResponseWriter, request *http.Request,
	f5state f5testing.MockF5State, keyName string) bool {
	_, ok := f5state.Keys[keyName]
	if !ok {
		response.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(response,
			`{"code":400,"message":"0107134a:3: File object by name (%s) is missing.","errorStack":[]}`,
			keyName)
		return false
	}

	return true
}

func postServerSslProfileHandler(f5state f5testing.MockF5State) http.HandlerFunc {
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
		_, serverSslProfileAlreadyExists := f5state.ServerSslProfiles[serverSslProfileName]
		if serverSslProfileAlreadyExists {
			response.WriteHeader(http.StatusConflict)
			fmt.Fprintf(response,
				`{"code":409,"message":"01020066:3: The requested ServerSSL Profile (/Common/%s) already exists in partition Common.","errorStack":[]}`,
				serverSslProfileName)
			return
		}

		_, clientSslProfileAlreadyExists := f5state.ClientSslProfiles[serverSslProfileName]
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

		f5state.ServerSslProfiles[serverSslProfileName] = true

		OK(response)
	}
}

func deleteServerSslProfileHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		serverSslProfileName := vars["profileName"]

		_, serverSslProfileFound := f5state.ServerSslProfiles[serverSslProfileName]
		if !serverSslProfileFound {
			response.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(response,
				`{"code":404,"message":"01020036:3: The requested ServerSSL Profile (/Common/%s) was not found.","errorStack":[]}`,
				serverSslProfileName)
			return
		}

		delete(f5state.ServerSslProfiles, serverSslProfileName)

		OK(response)
	}
}

func associateProfileWithVserver(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserver := newMockF5iControlResource("vserver", vars["vserverName"])

		if !validateVserver(response, request, f5state, vserver) {
			return
		}

		payload := struct {
			Name string `json:"name"`
		}{}
		decoder := json.NewDecoder(request.Body)
		decoder.Decode(&payload)

		profileName := payload.Name

		f5state.VserverProfiles[vserver.id()][profileName] = true

		OK(response)
	}
}

func deleteSslVserverProfileHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		vserver := newMockF5iControlResource("vserver", vars["vserverName"])
		profileName := vars["profileName"]

		if !validateVserver(response, request, f5state, vserver) {
			return
		}

		delete(f5state.VserverProfiles[vserver.id()], profileName)

		OK(response)
	}
}

func deleteSslKeyHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		keyName := vars["keyName"]

		_, keyFound := f5state.Keys[keyName]
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

		delete(f5state.Keys, keyName)

		OK(response)
	}
}

func deleteSslCertHandler(f5state f5testing.MockF5State) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		vars := mux.Vars(request)
		certName := vars["certName"]

		_, certFound := f5state.Certs[certName]
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

		delete(f5state.Certs, certName)

		OK(response)
	}
}

// TestInitializeF5Plugin initializes the F5 plug-in with a mock unconfigured F5
// BIG-IP host and validates the configuration of the F5 BIG-IP host after the
// plug-in has performed its initialization.
func TestInitializeF5Plugin(t *testing.T) {
	router, mockF5, err := newTestRouter(F5DefaultPartitionPath)
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}
	defer mockF5.close()

	// The policy for secure routes and the policy for insecure routes should
	// exist.
	expectedPolicies := []string{insecureRoutesPolicyName, secureRoutesPolicyName}
	for _, policyName := range expectedPolicies {
		policy := newMockF5iControlResource("policy", path.Join(F5DefaultPartitionPath, policyName))
		_, ok := mockF5.state.Policies[policy.id()]
		if !ok {
			t.Errorf("%s policy was not created; policies map: %v",
				policyName, mockF5.state.Policies)
		}
	}

	// The HTTPS vserver should have the policy for secure routes associated.
	foundSecureRoutesPolicy := false
	httpsVserver := newMockF5iControlResource("vserver", path.Join(F5DefaultPartitionPath, httpsVserverName))
	for policyName := range mockF5.state.VserverPolicies[httpsVserver.id()] {
		if strings.HasSuffix(policyName, secureRoutesPolicyName) {
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
	httpVserver := newMockF5iControlResource("vserver", path.Join(F5DefaultPartitionPath, httpVserverName))
	for policyName := range mockF5.state.VserverPolicies[httpVserver.id()] {
		if strings.HasSuffix(policyName, insecureRoutesPolicyName) {
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

	resource := newMockF5iControlResource("datagroup", passthroughIRuleDatagroupName)

	// The datagroup for passthrough routes should exist.
	foundPassthroughIRuleDatagroup := false
	for datagroupName := range mockF5.state.Datagroups {
		if datagroupName == resource.FullPath {
			foundPassthroughIRuleDatagroup = true
		}
	}
	if !foundPassthroughIRuleDatagroup {
		t.Errorf("%s datagroup was not created.", passthroughIRuleDatagroupName)
	}

	// The passthrough iRule should exist and should reference the datagroup for
	// passthrough routes.
	foundPassthroughIRule := false
	for iRuleName, iRuleCode := range mockF5.state.IRules {
		if strings.HasSuffix(iRuleName, passthroughIRuleName) {
			foundPassthroughIRule = true

			if !strings.Contains(string(iRuleCode), passthroughIRuleDatagroupName) {
				t.Errorf("iRule for passthrough routes exists, but its body does not"+
					" reference the datagroup for passthrough routes.\n"+
					"iRule name: %s\nf5testing.Datagroup name: %s\niRule code: %s",
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
	for _, iRuleName := range mockF5.state.VserverIRules[httpsVserver.id()] {
		if strings.HasSuffix(iRuleName, passthroughIRuleName) {
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
	if len(mockF5.state.VserverIRules[httpVserver.id()]) != 0 {
		t.Errorf("Vserver %s has iRules associated: %v",
			httpVserverName, mockF5.state.VserverIRules[httpVserver.id()])
	}

	// Initialization should be idempotent.
	// Warning: This is off-label use of DeepCopy!
	savedMockF5State := *mockF5.state.DeepCopy()

	router.F5Client.Initialize()

	if !reflect.DeepEqual(savedMockF5State, mockF5.state) {
		t.Errorf("Initialize method should be idempotent but it is not.\n"+
			"State after first initialization: %v\n"+
			"State after second initialization: %v\n",
			savedMockF5State, mockF5.state)
	}
}

// TestF5PartitionPath creates an F5 router instance with a specific partition.
func TestF5RouterPartition(t *testing.T) {
	testCases := []struct {
		// name of the test
		name string

		// partition path.
		partition string
	}{
		{
			name:      "Default/Common partition",
			partition: "/Common",
		},
		{
			name:      "Custom partition",
			partition: "/OSPartA",
		},
		{
			name:      "Sub partition",
			partition: "/OSPartA/ShardOne",
		},
		{
			name:      "Layered sub partition",
			partition: "/OSPartA/region1/zone4/Shard-7",
		},
	}

	for _, tc := range testCases {
		_, mockF5, err := newTestRouter(tc.partition)
		if err != nil {
			t.Fatalf("Test case %q failed to initialize test router: %v", tc.name, err)
		}

		defer mockF5.close()
		_, ok := mockF5.state.PartitionPaths[tc.partition]
		if !ok {
			t.Fatalf("Test case %q missing partition key %s", tc.name, tc.partition)
		}
	}
}

// TestHandleEndpoints tests endpoint watch events and validates that the state
// of the F5 client object is as expected after each event.
func TestHandleEndpoints(t *testing.T) {
	router, mockF5, err := newTestRouter(F5DefaultPartitionPath)
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
	router, mockF5, err := newTestRouter(F5DefaultPartitionPath)
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "unsecuretest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)
				policy := newMockF5iControlResource("policy", insecureRoutesPolicyName)

				rule, ok := mockF5.state.Policies[policy.id()][rulename]
				if !ok {
					return fmt.Errorf("Policy %s should have rule %s,"+
						" but no rule was found: %v",
						insecureRoutesPolicyName, rulename,
						mockF5.state.Policies[policy.id()])
				}

				if len(rule.Conditions) != 1 {
					return fmt.Errorf("Insecure route should have rule with 1 condition,"+
						" but rule has %d conditions: %v",
						len(rule.Conditions), rule.Conditions)
				}

				condition := rule.Conditions[0]

				if !(condition.HttpHost && condition.Host && !condition.HttpUri &&
					!condition.PathSegment) {
					return fmt.Errorf("Insecure route should have rule that matches on"+
						" hostname but found this instead: %v", rule.Conditions)
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "unsecuretest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example2.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
					Path: "/foo/bar",
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)
				policy := newMockF5iControlResource("policy", insecureRoutesPolicyName)

				rule, ok := mockF5.state.Policies[policy.id()][rulename]
				if !ok {
					return fmt.Errorf("Policy %s should have rule %s,"+
						" but no rule was found: %v",
						insecureRoutesPolicyName, rulename,
						mockF5.state.Policies[policy.id()])
				}
				if len(rule.Conditions) != 3 {
					return fmt.Errorf("Insecure route with pathname should have rule"+
						" with 3 conditions, but rule has %d conditions: %v",
						len(rule.Conditions), rule.Conditions)
				}

				pathSegments := strings.Split(tc.route.Spec.Path, "/")

				for _, condition := range rule.Conditions {
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
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "unsecuretest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example2.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				rulename := routeName(*tc.route)
				policy := newMockF5iControlResource("policy", secureRoutesPolicyName)

				_, found := mockF5.state.Policies[policy.id()][rulename]
				if found {
					return fmt.Errorf("Rule %s should have been deleted from policy %s"+
						" when the corresponding route was deleted, but it remains yet: %v",
						rulename, secureRoutesPolicyName,
						mockF5.state.Policies[policy.id()])
				}

				return nil
			},
		},
		{
			name:      "Edge route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "edgetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: routeapi.RouteTargetReference{
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
				policy := newMockF5iControlResource("policy", secureRoutesPolicyName)

				_, found := mockF5.state.Policies[policy.id()][rulename]
				if !found {
					return fmt.Errorf("Policy %s should have rule %s,"+
						" but no such rule was found: %v",
						secureRoutesPolicyName, rulename,
						mockF5.state.Policies[policy.id()])
				}

				certfname := fmt.Sprintf("%s-https-cert.crt", rulename)
				_, found = mockF5.state.Certs[certfname]
				if !found {
					return fmt.Errorf("Certificate file %s should have been created but"+
						" does not exist: %v",
						certfname, mockF5.state.Certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.Keys[keyfname]
				if !found {
					return fmt.Errorf("Key file %s should have been created but"+
						" does not exist: %v",
						keyfname, mockF5.state.Keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.ClientSslProfiles[clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been created"+
						" but does not exist: %v",
						clientSslProfileName, mockF5.state.ClientSslProfiles)
				}

				httpsVserverPath := normalizeResourcePath(httpsVserverName)
				_, found = mockF5.state.VserverProfiles[httpsVserverPath][clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been"+
						" associated with the vserver but was not: %v",
						clientSslProfileName,
						mockF5.state.VserverProfiles[httpsVserverPath])
				}

				return nil
			},
		},
		{
			name:      "Edge route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "edgetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example.com",
					To: routeapi.RouteTargetReference{
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
				policy := newMockF5iControlResource("policy", secureRoutesPolicyName)

				_, found := mockF5.state.Policies[policy.id()][rulename]
				if found {
					return fmt.Errorf("Rule %s should have been deleted from policy %s"+
						" when the corresponding route was deleted, but it remains yet: %v",
						rulename, secureRoutesPolicyName,
						mockF5.state.Policies[policy.id()])
				}

				certfname := fmt.Sprintf("%s-https-cert.crt", rulename)
				_, found = mockF5.state.Certs[certfname]
				if found {
					return fmt.Errorf("Certificate file %s should have been deleted with"+
						" the route but remains yet: %v",
						certfname, mockF5.state.Certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.Keys[keyfname]
				if found {
					return fmt.Errorf("Key file %s should have been deleted with the"+
						" route but remains yet: %v",
						keyfname, mockF5.state.Keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				httpsVserverPath := normalizeResourcePath(httpsVserverName)
				_, found = mockF5.state.VserverProfiles[httpsVserverPath][clientSslProfileName]
				if found {
					return fmt.Errorf("client-ssl profile %s should have been deleted"+
						" from the vserver when the route was deleted but remains yet: %v",
						clientSslProfileName,
						mockF5.state.VserverProfiles[httpsVserverPath])
				}

				_, found = mockF5.state.ClientSslProfiles[clientSslProfileName]
				if found {
					return fmt.Errorf("client-ssl profile %s should have been deleted"+
						" with the route but remains yet: %v",
						clientSslProfileName, mockF5.state.ClientSslProfiles)
				}

				return nil
			},
		},
		{
			name:      "Passthrough route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "passthroughtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationPassthrough,
					},
				},
			},
			validate: func(tc testCase) error {
				resource := newMockF5iControlResource("datagroup", passthroughIRuleDatagroupName)
				_, found := mockF5.state.Datagroups[resource.id()][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("f5testing.Datagroup entry for %s should have been created"+
						" in the %s datagroup for the passthrough route but cannot be"+
						" found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.Datagroups[resource.id()])
				}

				return nil
			},
		},
		{
			name:      "Add route with same hostname as passthrough route",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "conflictingroutetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: routeapi.RouteTargetReference{
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
				resource := newMockF5iControlResource("datagroup", passthroughIRuleDatagroupName)
				_, found := mockF5.state.Datagroups[resource.id()][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("f5testing.Datagroup entry for %s should still exist"+
						" in the %s datagroup after a secure route with the same hostname"+
						" was created, but the datagroup entry cannot be found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.Datagroups[resource.id()])
				}

				return nil
			},
		},
		{
			name:      "Modify route with same hostname as passthrough route",
			eventType: watch.Modified,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "conflictingroutetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				resource := newMockF5iControlResource("datagroup", passthroughIRuleDatagroupName)
				_, found := mockF5.state.Datagroups[resource.id()][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("f5testing.Datagroup entry for %s should still exist"+
						" in the %s datagroup after a secure route with the same hostname"+
						" was updated, but the datagroup entry cannot be found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.Datagroups[resource.id()])
				}

				return nil
			},
		},
		{
			name:      "Delete route with same hostname as passthrough route",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "conflictingroutetest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
				},
			},
			validate: func(tc testCase) error {
				resource := newMockF5iControlResource("datagroup", passthroughIRuleDatagroupName)
				_, found := mockF5.state.Datagroups[resource.id()][tc.route.Spec.Host]
				if !found {
					return fmt.Errorf("f5testing.Datagroup entry for %s should still exist"+
						" in the %s datagroup after a secure route with the same hostname"+
						" was deleted, but the datagroup entry cannot be found: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.Datagroups[resource.id()])
				}

				return nil
			},
		},
		{
			name:      "Passthrough route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "passthroughtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example3.com",
					To: routeapi.RouteTargetReference{
						Name: "TestService",
					},
					TLS: &routeapi.TLSConfig{
						Termination: routeapi.TLSTerminationPassthrough,
					},
				},
			},
			validate: func(tc testCase) error {
				resource := newMockF5iControlResource("datagroup", passthroughIRuleDatagroupName)
				_, found := mockF5.state.Datagroups[resource.id()][tc.route.Spec.Host]
				if found {
					return fmt.Errorf("f5testing.Datagroup entry for %s should have been deleted"+
						" from the %s datagroup for the passthrough route but remains"+
						" yet: %v",
						tc.route.Spec.Host, passthroughIRuleDatagroupName,
						mockF5.state.Datagroups[resource.id()])
				}

				return nil
			},
		},
		{
			name:      "Reencrypted route add",
			eventType: watch.Added,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "reencryptedtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example4.com",
					To: routeapi.RouteTargetReference{
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
				policy := newMockF5iControlResource("policy", secureRoutesPolicyName)

				_, found := mockF5.state.Policies[policy.id()][rulename]
				if !found {
					return fmt.Errorf("Policy %s should have rule %s for secure route,"+
						" but no rule was found: %v",
						secureRoutesPolicyName, rulename,
						mockF5.state.Policies[policy.id()])
				}

				certcafname := fmt.Sprintf("%s-https-chain.crt", rulename)
				_, found = mockF5.state.Certs[certcafname]
				if !found {
					return fmt.Errorf("Certificate chain file %s should have been"+
						" created but does not exist: %v",
						certcafname, mockF5.state.Certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.Keys[keyfname]
				if !found {
					return fmt.Errorf("Key file %s should have been created but"+
						" does not exist: %v",
						keyfname, mockF5.state.Keys)
				}

				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.ClientSslProfiles[clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been created"+
						" but does not exist: %v",
						clientSslProfileName, mockF5.state.ClientSslProfiles)
				}

				httpsVserver := newMockF5iControlResource("vserver", httpsVserverName)
				_, found = mockF5.state.VserverProfiles[httpsVserver.id()][clientSslProfileName]
				if !found {
					return fmt.Errorf("client-ssl profile %s should have been"+
						" associated with the vserver but was not: %v",
						clientSslProfileName,
						mockF5.state.VserverProfiles[httpsVserver.id()])
				}

				serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", rulename)
				_, found = mockF5.state.ServerSslProfiles[serverSslProfileName]
				if !found {
					return fmt.Errorf("server-ssl profile %s should have been created"+
						" but does not exist: %v",
						serverSslProfileName, mockF5.state.ServerSslProfiles)
				}

				_, found = mockF5.state.VserverProfiles[httpsVserver.id()][serverSslProfileName]
				if !found {
					return fmt.Errorf("server-ssl profile %s should have been"+
						" associated with the vserver but was not: %v",
						serverSslProfileName,
						mockF5.state.VserverProfiles[httpsVserver.id()])
				}

				return nil
			},
		},
		{
			name:      "Reencrypted route delete",
			eventType: watch.Deleted,
			route: &routeapi.Route{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "reencryptedtest",
				},
				Spec: routeapi.RouteSpec{
					Host: "www.example4.com",
					To: routeapi.RouteTargetReference{
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
				policy := newMockF5iControlResource("policy", secureRoutesPolicyName)

				_, found := mockF5.state.Policies[policy.id()][rulename]
				if found {
					return fmt.Errorf("Rule %s should have been deleted from policy %s"+
						" when the corresponding route was deleted, but it remains yet: %v",
						rulename, secureRoutesPolicyName,
						mockF5.state.Policies[policy.id()])
				}

				certcafname := fmt.Sprintf("%s-https-chain.crt", rulename)
				_, found = mockF5.state.Certs[certcafname]
				if found {
					return fmt.Errorf("Certificate chain file %s should have been"+
						" deleted with the route but remains yet: %v",
						certcafname, mockF5.state.Certs)
				}

				keyfname := fmt.Sprintf("%s-https-key.key", rulename)
				_, found = mockF5.state.Keys[keyfname]
				if found {
					return fmt.Errorf("Key file %s should have been deleted with the"+
						" route but remains yet: %v",
						keyfname, mockF5.state.Keys)
				}

				httpsVserver := newMockF5iControlResource("vserver", httpsVserverName)
				clientSslProfileName := fmt.Sprintf("%s-client-ssl-profile", rulename)
				_, found = mockF5.state.VserverProfiles[httpsVserver.id()][clientSslProfileName]
				if found {
					return fmt.Errorf("client-ssl profile %s should have been deleted"+
						" from the vserver when the route was deleted but remains yet: %v",
						clientSslProfileName,
						mockF5.state.VserverProfiles[httpsVserver.id()])
				}

				serverSslProfileName := fmt.Sprintf("%s-server-ssl-profile", rulename)
				_, found = mockF5.state.VserverProfiles[httpsVserver.id()][clientSslProfileName]
				if found {
					return fmt.Errorf("server-ssl profile %s should have been deleted"+
						" from the vserver when the route was deleted but remains yet: %v",
						serverSslProfileName,
						mockF5.state.VserverProfiles[httpsVserver.id()])
				}

				_, found = mockF5.state.ServerSslProfiles[serverSslProfileName]
				if found {
					return fmt.Errorf("server-ssl profile %s should have been deleted"+
						" with the route but remains yet: %v",
						serverSslProfileName, mockF5.state.ServerSslProfiles)
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
	router, mockF5, err := newTestRouter(F5DefaultPartitionPath)
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}
	defer mockF5.close()

	testRoute := &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "mutatingroute",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: routeapi.RouteTargetReference{
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
	router, mockF5, err := newTestRouter(F5DefaultPartitionPath)
	if err != nil {
		t.Fatalf("Failed to initialize test router: %v", err)
	}

	testRoute := &routeapi.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "xyzzy",
			Name:      "testroute",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example.com",
			To: routeapi.RouteTargetReference{
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
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "quux",
			Name:      "testpassthroughroute",
		},
		Spec: routeapi.RouteSpec{
			Host: "www.example2.com",
			To: routeapi.RouteTargetReference{
				Name: "testhttpsendpoint",
			},
			TLS: &routeapi.TLSConfig{
				Termination: routeapi.TLSTerminationPassthrough,
			},
		},
	}

	testHttpEndpoint := &kapi.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "xyzzy",
			Name:      "testhttpendpoint",
		},
		Subsets: []kapi.EndpointSubset{{
			Addresses: []kapi.EndpointAddress{{IP: "10.1.1.1"}},
			Ports:     []kapi.EndpointPort{{Port: 8080}},
		}},
	}

	testHttpsEndpoint := &kapi.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
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
	router, mockF5, err = newTestRouterWithState(mockF5.state, F5DefaultPartitionPath)
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

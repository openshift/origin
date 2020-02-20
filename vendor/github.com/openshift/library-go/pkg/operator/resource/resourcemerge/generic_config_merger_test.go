package resourcemerge

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"

	controlplanev1 "github.com/openshift/api/kubecontrolplane/v1"
)

func TestMergeConfig(t *testing.T) {
	tests := []struct {
		name         string
		curr         map[string]interface{}
		additional   map[string]interface{}
		specialCases map[string]MergeFunc

		expected    map[string]interface{}
		expectedErr string
	}{
		{
			name: "add non-conflicting",
			curr: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple": "one",
				},
			},
			additional: map[string]interface{}{
				"bravo": map[string]interface{}{
					"banana": "two",
					"cake": map[string]interface{}{
						"armadillo": "uno",
					},
				},
				"charlie": "third",
			},

			expected: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple":  "one",
					"banana": "two",
					"cake": map[string]interface{}{
						"armadillo": "uno",
					},
				},
				"charlie": "third",
			},
		},
		{
			name: "add conflicting, replace type",
			curr: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple": "one",
				},
			},
			additional: map[string]interface{}{
				"bravo": map[string]interface{}{
					"apple": map[string]interface{}{
						"armadillo": "uno",
					},
				},
			},

			expected: map[string]interface{}{
				"alpha": "first",
				"bravo": map[string]interface{}{
					"apple": map[string]interface{}{
						"armadillo": "uno",
					},
				},
			},
		},
		{
			name: "nil out",
			curr: map[string]interface{}{
				"alpha": "first",
			},
			additional: map[string]interface{}{
				"alpha": nil,
			},

			expected: map[string]interface{}{
				"alpha": nil,
			},
		},
		{
			name: "force empty",
			curr: map[string]interface{}{
				"alpha": "first",
			},
			additional: map[string]interface{}{
				"alpha": "",
			},

			expected: map[string]interface{}{
				"alpha": "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := mergeConfig(test.curr, test.additional, "", test.specialCases)
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing %q", test.expectedErr)
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err != nil && len(test.expectedErr) != 0 && !strings.Contains(err.Error(), test.expectedErr):
				t.Fatalf("expected %q, got %q", test.expectedErr, err)
			}

			if !reflect.DeepEqual(test.expected, test.curr) {
				t.Error(diff.ObjectDiff(test.expected, test.curr))
			}
		})
	}
}

func TestMergeProcessConfig(t *testing.T) {
	tests := []struct {
		name         string
		curr         string
		additional   string
		specialCases map[string]MergeFunc

		expected    string
		expectedErr string
	}{
		{
			name: "no conflict on missing typemeta",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
bravo: two
`,
			expected: `{"alpha":"first","apiVersion":"foo","bravo":"two","kind":"the-kind"}
`,
		},
		{
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			name: "no conflict on same typemeta",
			additional: `
apiVersion: foo
kind: the-kind
bravo: two
`,
			expected: `{"alpha":"first","apiVersion":"foo","bravo":"two","kind":"the-kind"}
`,
		},
		{
			name: "conflict on different typemeta 01",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
kind: the-other-kind
bravo: two
`,
			expectedErr: `/the-other-kind does not equal foo/the-kind`,
		},
		{
			name: "conflict on different typemeta 03",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
apiVersion: bar
bravo: two
`,
			expectedErr: `bar/ does not equal foo/the-kind`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := MergeProcessConfig(test.specialCases, []byte(test.curr), []byte(test.additional))
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing %q", test.expectedErr)
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err != nil && len(test.expectedErr) != 0 && !strings.Contains(err.Error(), test.expectedErr):
				t.Fatalf("expected %q, got %q", test.expectedErr, err)
			}
			if err != nil {
				return
			}

			if test.expected != string(actual) {
				t.Error(diff.StringDiff(test.expected, string(actual)))
			}
		})
	}
}

func TestMergePrunedConfig(t *testing.T) {
	tests := []struct {
		name         string
		curr         string
		additional   string
		specialCases map[string]MergeFunc

		expected    string
		expectedErr string
	}{
		{
			name: "prune unknown values",
			curr: `
apiVersion: foo
kind: the-kind
alpha: first
`,
			additional: `
consolePublicURL: http://foo/bar
`,
			expected: `{"apiVersion":"foo","consolePublicURL":"http://foo/bar","kind":"the-kind"}`,
		},
		{
			name: "prune unknown values with array",
			curr: `
apiVersion: foo
kind: the-kind
corsAllowedOrigins:
- (?i)//openshift(:|\z)
`,
			additional: `
consolePublicURL: http://foo/bar
`,
			expected: `{"apiVersion":"foo","consolePublicURL":"http://foo/bar","corsAllowedOrigins":["(?i)//openshift(:|\\z)"],"kind":"the-kind"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := MergePrunedProcessConfig(&controlplanev1.KubeAPIServerConfig{}, test.specialCases, []byte(test.curr), []byte(test.additional))
			switch {
			case err == nil && len(test.expectedErr) == 0:
			case err == nil && len(test.expectedErr) != 0:
				t.Fatalf("missing %q", test.expectedErr)
			case err != nil && len(test.expectedErr) == 0:
				t.Fatal(err)
			case err != nil && len(test.expectedErr) != 0 && !strings.Contains(err.Error(), test.expectedErr):
				t.Fatalf("expected %q, got %q", test.expectedErr, err)
			}
			if err != nil {
				return
			}

			if test.expected != string(actual) {
				t.Error(diff.StringDiff(test.expected, string(actual)))
			}
		})
	}
}

func TestIsRequiredConfigPresent(t *testing.T) {
	tests := []struct {
		name          string
		config        string
		expectedError string
	}{
		{
			name: "unparseable",
			config: `{
		 "servingInfo": {
		}
		`,
			expectedError: "error parsing config",
		},
		{
			name:          "empty",
			config:        ``,
			expectedError: "no observedConfig",
		},
		{
			name: "nil-storage-urls",
			config: `{
		 "servingInfo": {
		   "namedCertificates": [
		     {
		       "certFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.crt",
		       "keyFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.key"
		     }
		   ]
		 },
		 "admission": {"pluginConfig": { "network.openshift.io/RestrictedEndpointsAdmission": {}}},
		 "storageConfig": {
		   "urls": null
		 }
		}
		`,
			expectedError: "storageConfig.urls null in config",
		},
		{
			name: "missing-storage-urls",
			config: `{
		 "servingInfo": {
		   "namedCertificates": [
		     {
		       "certFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.crt",
		       "keyFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.key"
		     }
		   ]
		 },
        "admission": {"pluginConfig": { "network.openshift.io/RestrictedEndpointsAdmission": {}}},
		 "storageConfig": {
		   "urls": []
		 }
		}
		`,
			expectedError: "storageConfig.urls empty in config",
		},
		{
			name: "empty-string-storage-urls",
			config: `{
  "servingInfo": {
    "namedCertificates": [
      {
        "certFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.crt",
        "keyFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.key"
      }
    ]
  },
  "admission": {"pluginConfig": { "network.openshift.io/RestrictedEndpointsAdmission": {}}},
  "storageConfig": {
    "urls": ""
  }
}
`,
			expectedError: "storageConfig.urls empty in config",
		},
		{
			name: "good",
			config: `{
		 "servingInfo": {
		   "namedCertificates": [
		     {
		       "certFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.crt",
		       "keyFile": "/etc/kubernetes/static-pod-certs/secrets/localhost-serving-cert-certkey/tls.key"
		     }
		   ]
		 },
         "admission": {"pluginConfig": { "network.openshift.io/RestrictedEndpointsAdmission": {}}},
		 "storageConfig": {
		   "urls": [ "val" ]
		 }
		}
		`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := IsRequiredConfigPresent([]byte(test.config), [][]string{
				{"servingInfo", "namedCertificates"},
				{"storageConfig", "urls"},
				{"admission", "pluginConfig", "network.openshift.io/RestrictedEndpointsAdmission"},
			})
			switch {
			case actual == nil && len(test.expectedError) == 0:
			case actual == nil && len(test.expectedError) != 0:
				t.Fatal(actual)
			case actual != nil && len(test.expectedError) == 0:
				t.Fatal(actual)
			case actual != nil && len(test.expectedError) != 0 && !strings.Contains(actual.Error(), test.expectedError):
				t.Fatal(actual)
			}
		})
	}
}

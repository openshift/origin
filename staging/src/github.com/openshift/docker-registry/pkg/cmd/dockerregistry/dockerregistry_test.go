package dockerregistry

import (
	"reflect"
	"strings"
	"testing"

	"github.com/docker/distribution/configuration"
)

func TestDefaultMiddleware(t *testing.T) {
	checks := []struct {
		title, input, expect string
	}{
		{
			title: "miss all middlewares",
			input: `
version: 0.1
storage:
  inmemory: {}
`,
			expect: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "miss some middlewares",
			input: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
`,
			expect: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "all middlewares are in place",
			input: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
			expect: `
version: 0.1
storage:
  inmemory: {}
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "check v1.0.8 config",
			input: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
auth:
  openshift:
    realm: openshift
middleware:
  repository:
   - name: openshift
`,
			expect: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
  storage:
    - name: openshift
`,
		},
		{
			title: "check v1.2.1 config",
			input: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  repository:
    - name: openshift
      options:
        pullthrough: true
`,
			expect: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    layerinfo: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        pullthrough: true
  storage:
    - name: openshift
`,
		},
		{
			title: "check v1.3.0-alpha.3 config",
			input: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        acceptschema2: false
        pullthrough: true
        enforcequota: false
        projectcachettl: 1m
        blobrepositorycachettl: 10m
  storage:
    - name: openshift
`,
			expect: `
version: 0.1
log:
  level: debug
http:
  addr: :5000
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: /registry
  delete:
    enabled: true
auth:
  openshift:
    realm: openshift
middleware:
  registry:
    - name: openshift
  repository:
    - name: openshift
      options:
        acceptschema2: false
        pullthrough: true
        enforcequota: false
        projectcachettl: 1m
        blobrepositorycachettl: 10m
  storage:
    - name: openshift
`,
		},
	}

	for _, check := range checks {
		currentConfig, err := configuration.Parse(strings.NewReader(check.input))
		if err != nil {
			t.Fatal(err)
		}
		expectConfig, err := configuration.Parse(strings.NewReader(check.expect))
		if err != nil {
			t.Fatal(err)
		}
		setDefaultMiddleware(currentConfig)

		if !reflect.DeepEqual(currentConfig, expectConfig) {
			t.Errorf("%s: expected\n\t%#v\ngot\n\t%#v", check.title, expectConfig, currentConfig)
		}
	}
}

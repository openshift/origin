package describe

import (
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

func mustParseTime(t string) time.Time {
	out, err := time.Parse(time.RFC3339, t)
	if err != nil {
		panic(err)
	}
	return out
}

func TestProjectStatus(t *testing.T) {
	testCases := map[string]struct {
		Path     string
		Extra    []runtime.Object
		ErrFn    func(error) bool
		Contains []string
		Time     time.Time
	}{
		"missing project": {
			ErrFn: func(err error) bool { return errors.IsNotFound(err) },
		},
		"empty project with display name": {
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "example",
						Namespace: "",
						Annotations: map[string]string{
							projectapi.ProjectDisplayName: "Test",
						},
					},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project Test (example) on server https://example.com:8443\n",
				"You have no services, deployment configs, or build configs.",
			},
		},
		"empty service": {
			Path: "../../../../test/fixtures/app-scenarios/k8s-service-with-nothing.json",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/empty-service",
				"<initializing>:5432",
				"To see more, use",
			},
		},
		"service with RC": {
			Path: "../../../../test/fixtures/app-scenarios/k8s-unserviced-rc.json",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/database-rc",
				"rc/database-rc-1 runs mysql",
				"0/1 pods growing to 1",
				"To see more, use",
			},
		},
		"rc with unmountable and missing secrets": {
			Path: "../../../../pkg/api/graph/test/bad_secret_with_just_rc.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"rc/my-rc runs openshift/mysql-55-centos7",
				"0/1 pods growing to 1",
				"rc/my-rc is attempting to mount a secret secret/existing-secret disallowed by sa/default",
				"rc/my-rc is attempting to mount a secret secret/dne disallowed by sa/default",
				"rc/my-rc is attempting to mount a missing secret secret/dne",
			},
		},
		"dueling rcs": {
			Path: "../../../../pkg/api/graph/test/dueling-rcs.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "dueling-rc", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"rc/rc-1 is competing for pod/conflicted-pod with rc/rc-2",
				"rc/rc-2 is competing for pod/conflicted-pod with rc/rc-1",
			},
		},
		"service with pod": {
			Path: "../../../../pkg/api/graph/test/service-with-pod.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/frontend-app",
				"pod/frontend-app-1-bjwh8 runs openshift/ruby-hello-world",
				"To see more, use",
			},
		},
		"standalone rc": {
			Path: "../../../../pkg/api/graph/test/bare-rc.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"  rc/database-1 runs openshift/mysql-55-centos7",
				"rc/frontend-rc-1 runs openshift/ruby-hello-world",
			},
		},
		"unstarted build": {
			Path: "../../../../test/fixtures/app-scenarios/new-project-no-build.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/sinatra-example-2 - 172.30.17.48:8080",
				"builds git://github.com",
				"with docker.io/openshift/ruby-20-centos7:latest",
				"not built yet",
				"#1 deployment waiting on image or update",
				"To see more, use",
			},
		},
		"unpushable build": {
			Path: "../../../../pkg/api/graph/test/unpushable-build.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"bc/ruby-hello-world is pushing to imagestreamtag/ruby-hello-world:latest that is using is/ruby-hello-world, but the administrator has not configured the integrated Docker registry.",
			},
		},
		"cyclical build": {
			Path: "../../../../pkg/api/graph/test/circular.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"Cycle detected in build configurations:",
			},
		},
		"running build": {
			Path: "../../../../test/fixtures/app-scenarios/new-project-one-build.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/sinatra-example-1 - 172.30.17.47:8080",
				"builds git://github.com",
				"with docker.io/openshift/ruby-20-centos7:latest",
				"build 1 running for about a minute",
				"#1 deployment waiting on image or update",
				"To see more, use",
			},
			Time: mustParseTime("2015-04-06T21:20:03Z"),
		},
		"a/b test DeploymentConfig": {
			Path: "../../../../test/fixtures/app-scenarios/new-project-two-deployment-configs.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/sinatra-app-example - 172.30.17.49:8080",
				"sinatra-app-example-a deploys",
				"sinatra-app-example-b deploys",
				"with docker.io/openshift/ruby-20-centos7:latest",
				"build 1 running for about a minute",
				"- 7a4f354: Prepare v1beta3 Template types (Roy Programmer <someguy@outhere.com>)",
				"To see more, use",
			},
			Time: mustParseTime("2015-04-06T21:20:03Z"),
		},
		"with real deployments": {
			Path: "../../../../test/fixtures/app-scenarios/new-project-deployed-app.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"service/database - 172.30.17.240:5434 -> 3306",
				"service/frontend - 172.30.17.154:5432 -> 8080",
				"database deploys",
				"frontend deploys",
				"with docker.io/openshift/ruby-20-centos7:latest",
				"#2 deployment failed less than a second ago: unable to contact server - 0/1 pods",
				"#2 deployment running for 7 seconds - 2/1 pods",
				"#1 deployed 8 seconds ago",
				"#1 deployed less than a second ago",
				"To see more, use",
			},
			Time: mustParseTime("2015-04-07T04:12:25Z"),
		},
	}
	oldTimeFn := timeNowFn
	defer func() { timeNowFn = oldTimeFn }()
	for k, test := range testCases {
		timeNowFn = func() time.Time {
			if !test.Time.IsZero() {
				return test.Time
			}
			return time.Now()
		}
		o := ktestclient.NewObjects(kapi.Scheme, kapi.Scheme)
		if len(test.Path) > 0 {
			if err := ktestclient.AddObjectsFromPath(test.Path, o, kapi.Scheme); err != nil {
				t.Fatal(err)
			}
		}
		for _, obj := range test.Extra {
			o.Add(obj)
		}
		oc, kc := testclient.NewFixtureClients(o)
		d := ProjectStatusDescriber{C: oc, K: kc, Server: "https://example.com:8443"}
		out, err := d.Describe("example", "")
		if !test.ErrFn(err) {
			t.Errorf("%s: unexpected error: %v", k, err)
		}
		if err != nil {
			continue
		}
		for _, s := range test.Contains {
			if !strings.Contains(out, s) {
				t.Errorf("%s: did not have %q:\n%s\n---", k, s, out)
			}
		}
	}
}

package describe

import (
	"strings"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"
	utilerrors "k8s.io/kubernetes/pkg/util/errors"

	oapi "github.com/openshift/origin/pkg/api"
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
		File     string
		Extra    []runtime.Object
		ErrFn    func(error) bool
		Contains []string
		Time     time.Time
	}{
		"missing project": {
			ErrFn: func(err error) bool { return err == nil },
		},
		"empty project with display name": {
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "example",
						Namespace: "",
						Annotations: map[string]string{
							oapi.OpenShiftDisplayName: "Test",
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
			File: "k8s-service-with-nothing.json",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/empty-service",
				"<initializing>:5432",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
		},
		"service with RC": {
			File: "k8s-unserviced-rc.json",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/database-rc",
				"rc/database-rc-1 runs mysql",
				"0/1 pods growing to 1",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
		},
		"rc with unmountable and missing secrets": {
			File: "bad_secret_with_just_rc.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"rc/my-rc runs centos/mysql-56-centos7",
				"0/1 pods growing to 1",
				"rc/my-rc is attempting to mount a missing secret secret/dne",
			},
		},
		"dueling rcs": {
			File: "dueling-rcs.yaml",
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
			File: "service-with-pod.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/frontend-app",
				"pod/frontend-app-1-bjwh8 runs openshift/ruby-hello-world",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
		},
		"build chains": {
			File: "build-chains.json",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"from bc/frontend",
			},
		},
		"scheduled image stream": {
			File: "prereq-image-present-with-sched.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"import scheduled",
			},
		},
		"standalone rc": {
			File: "bare-rc.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"  rc/database-1 runs centos/mysql-56-centos7",
				"rc/frontend-rc-1 runs openshift/ruby-hello-world",
			},
		},
		"unstarted build": {
			File: "new-project-no-build.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/sinatra-example-2 - 172.30.17.48:8080",
				"deploys istag/sinatra-example-2:latest <-",
				"builds git://github.com",
				"on docker.io/centos/ruby-22-centos7:latest",
				"not built yet",
				"deployment #1 waiting on image or update",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
		},
		"unpushable build": {
			File: "unpushable-build.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"bc/ruby-hello-world is pushing to istag/ruby-hello-world:latest, but the administrator has not configured the integrated Docker registry.",
			},
		},
		"bare-bc-can-push": {
			File: "bare-bc-can-push.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				// this makes sure that status knows this can push.  If it fails, there's a "(can't push image)" next to like #8
				" hours\n  build #7",
				"on istag/fedora:23",
				"-> istag/repo-base:latest",
			},
			Time: mustParseTime("2015-12-17T20:36:15Z"),
		},
		"cyclical build": {
			File: "circular.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"Cycle detected in build configurations:",
				"on istag/ruby-22-centos7:latest",
				"-> istag/ruby-hello-world:latest",
			},
		},
		"running build": {
			File: "new-project-one-build.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/sinatra-example-1 - 172.30.17.47:8080",
				"builds git://github.com",
				"on docker.io/centos/ruby-22-centos7:latest",
				"build #1 running for about a minute",
				"deployment #1 waiting on image or update",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
			Time: mustParseTime("2015-04-06T21:20:03Z"),
		},
		"a/b test DeploymentConfig": {
			File: "new-project-two-deployment-configs.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/sinatra-app-example - 172.30.17.49:8080",
				"sinatra-app-example-a deploys",
				"sinatra-app-example-b deploys",
				"on docker.io/centos/ruby-22-centos7:latest",
				"build #1 running for about a minute",
				"- 7a4f354: Prepare v1 Template types (Roy Programmer <someguy@outhere.com>)",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
			Time: mustParseTime("2015-04-06T21:20:03Z"),
		},
		"with real deployments": {
			File: "new-project-deployed-app.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/database - 172.30.17.240:5434 -> 3306",
				"https://www.test.com (redirects) to pod port 8080 (svc/frontend)",
				"http://frontend-example.router.default.svc.cluster.local to pod port 8080 (!)",
				"svc/database-external (all nodes):31000 -> 3306",
				"database test deploys",
				"frontend deploys",
				"istag/origin-ruby-sample:latest <-",
				"on docker.io/centos/ruby-22-centos7:latest",
				"deployment #3 pending on image",
				"deployment #2 failed less than a second ago: unable to contact server - 0/1 pods",
				"deployment #1 deployed less than a second ago",
				"test deployment #2 running for 7 seconds - 2/1 pods",
				"test deployment #1 deployed 8 seconds ago",
				"* bc/ruby-sample-build is pushing to istag/origin-ruby-sample:latest, but the image stream for that tag does not exist.",
				"* The image trigger for dc/frontend will have no effect because is/origin-ruby-sample does not exist",
				"* route/frontend was not accepted by router \"other\":  (HostAlreadyClaimed)",
				"* dc/database has no readiness probe to verify pods are ready to accept traffic or ensure deployment is successful.",
				"View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.",
			},
			Time: mustParseTime("2015-04-07T04:12:25Z"),
		},
		"with stateful sets": {
			File: "statefulset.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"svc/galera (headless):3306",
				"statefulset/mysql manages erkules/galera:basic, created less than a second ago - 3 pods",
				"* pod/mysql-1 has restarted 7 times",
			},
			Time: mustParseTime("2015-04-07T04:12:25Z"),
		},
		"restarting pod": {
			File: "restarting-pod.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				`container "ruby-helloworld" in pod/frontend-app-1-bjwh8 has restarted 8 times`,
				`pod/gitlab-ce-1-lc411 is crash-looping`,
				`oc logs -p gitlab-ce-1-lc411 -c gitlab-ce`, // verifies we print the log command
				`policycommand example default`,             // verifies that we print the help command
			},
		},
		"cross namespace reference": {
			File: "different-project-image-deployment.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				// If there was a warning we wouldn't get the following message. Since we ignore cross-namespace
				// links by default, there should be no warning here.
				`View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.`,
			},
		},
		"monopod": {
			File: "k8s-lonely-pod.json",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"pod/lonely-pod runs openshift/hello-openshift",
				"You have no services, deployment configs, or build configs.",
			},
		},
		"deploys single pod": {
			File: "simple-deployment.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"In project example on server https://example.com:8443\n",
				"dc/simple-deployment deploys docker.io/openshift/deployment-example:v1",
				`View details with 'oc describe <resource>/<name>' or list everything with 'oc get all'.`,
			},
		},
		"deployment with unavailable pods": {
			File: "available-deployment.yaml",
			Extra: []runtime.Object{
				&projectapi.Project{
					ObjectMeta: kapi.ObjectMeta{Name: "example", Namespace: ""},
				},
			},
			ErrFn: func(err error) bool { return err == nil },
			Contains: []string{
				"deployment #2 running for 30 seconds - 0/1 pods\n",
				"deployment #1 deployed about a minute ago - 1/2 pods",
			},
			Time: mustParseTime("2016-04-07T04:12:25Z"),
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
		objs := []runtime.Object{}
		if len(test.File) > 0 {
			// Load data from a folder dedicated to mock data, which is never loaded into the API during tests
			var err error
			objs, err = testclient.ReadObjectsFromPath("../../../../pkg/api/graph/test/"+test.File, "example", kapi.Codecs.UniversalDecoder(), kapi.Scheme)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", k, err)
			}
		}
		for _, o := range test.Extra {
			objs = append(objs, o)
		}
		oc, kc := testclient.NewFixtureClients(objs...)
		d := ProjectStatusDescriber{K: kc, C: oc, Server: "https://example.com:8443", Suggest: true, CommandBaseName: "oc", LogsCommandName: "oc logs -p", SecurityPolicyCommandFormat: "policycommand %s %s"}
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

func TestProjectStatusErrors(t *testing.T) {
	testCases := map[string]struct {
		Err   error
		ErrFn func(error) bool
	}{
		"project error is returned": {
			Err: errors.NewBadRequest("unavailable"),
			ErrFn: func(err error) bool {
				if aggr, ok := err.(utilerrors.Aggregate); ok {
					for _, e := range aggr.Errors() {
						if !errors.IsBadRequest(e) {
							return false
						}
					}
					return true
				}
				return false
			},
		},
	}
	for k, test := range testCases {
		oc, kc := testclient.NewErrorClients(test.Err)
		d := ProjectStatusDescriber{K: kc, C: oc, Server: "https://example.com:8443", Suggest: true, CommandBaseName: "oc", LogsCommandName: "oc logs -p", SecurityPolicyCommandFormat: "policycommand %s %s"}
		_, err := d.Describe("example", "")
		if !test.ErrFn(err) {
			t.Errorf("%s: unexpected error: %v", k, err)
		}
	}
}

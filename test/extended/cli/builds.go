package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	t "github.com/onsi/gomega/types"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const (
	bcOutputTemplate       = "{{with .spec.output.to}}{{.kind}} {{.name}}{{end}}"
	bcOutputToNameTemplate = "{{ .spec.output.to.name }}"
	bcSourceTypeTemplate   = "{{ .spec.source.type }}"
)

func getExpectedBCOutputMatcher(bcName string, isNames ...string) t.GomegaMatcher {
	matcher := o.ContainSubstring("Success")
	if bcName != "" {
		matcher = o.And(matcher, o.ContainSubstring(fmt.Sprintf("buildconfig.build.openshift.io %q created", bcName)))
	} else {
		e2e.Logf("no bcName specified")
	}

	for _, isName := range isNames {
		if isName != "" {
			matcher = o.And(matcher, o.ContainSubstring(fmt.Sprintf("imagestream.image.openshift.io %q created", isName)))
		}
	}
	return matcher
}

func getBCInfoByTemplate(oc *exutil.CLI, bcName, template string) string {
	if bcName == "" {
		e2e.Logf("no bcName specified")
		return ""
	}
	out, err := oc.Run("get").Args("bc/"+bcName, "--template", template).Output()
	if err != nil {
		e2e.Logf("failed to get %v: %v", bcName, err)
		return ""
	}
	return out
}

func getBCSourceType(oc *exutil.CLI, bcName string) string {
	return getBCInfoByTemplate(oc, bcName, bcSourceTypeTemplate)
}

func getBCOutputType(oc *exutil.CLI, bcName string) string {
	return getBCInfoByTemplate(oc, bcName, bcOutputTemplate)
}

var _ = g.Describe("[sig-cli] oc builds", func() {
	defer g.GinkgoRecover()

	var (
		oc                    = exutil.NewCLI("oc-builds").AsAdmin()
		testDockerfileContent = fmt.Sprintf("FROM %s", image.ShellImage())
	)

	g.It("new-build [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		g.By("build from a binary with no inputs requires name")
		out, err := oc.Run("new-build").Args("--binary").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("you must provide a --name"))

		g.By("build from a binary with inputs creates a binary build")
		out, err = oc.Run("new-build").Args("--binary", "--name=binary-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("binary-test", "binary-test"))
		o.Expect(getBCSourceType(oc, "binary-test")).To(o.Equal("Binary"))
		o.Expect(getBCOutputType(oc, "binary-test")).To(o.Equal("ImageStreamTag binary-test:latest"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from git with output to ImageStreamTag")
		out, err = oc.Run("new-build").Args("registry.access.redhat.com/ubi8/ruby-27", "https://github.com/openshift/ruby-hello-world.git").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("ruby-hello-world", "ruby-hello-world"))
		o.Expect(getBCSourceType(oc, "ruby-hello-world")).To(o.Equal("Git"))
		o.Expect(getBCOutputType(oc, "ruby-hello-world")).To(o.Equal("ImageStreamTag ruby-hello-world:latest"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from Dockerfile with output to ImageStreamTag")
		out, err = oc.Run("new-build").Args("--to=tools:custom", "--dockerfile="+testDockerfileContent+"\nRUN yum install -y httpd").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("tools", "tools"))
		o.Expect(getBCSourceType(oc, "tools")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "tools")).To(o.Equal("ImageStreamTag tools:custom"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from stdin Dockerfile  to ImageStreamTag")
		out, err = oc.Run("new-build").Args("-D", "-", "--name=stdintest").InputString(testDockerfileContent).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("stdintest", "stdintest", "tools"))
		o.Expect(getBCSourceType(oc, "stdintest")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "stdintest")).To(o.Equal("ImageStreamTag stdintest:latest"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from Dockerfile with output to DockerImage")
		out, err = oc.Run("new-build").Args("-D", testDockerfileContent, "--to-docker").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("tools", "tools"))
		o.Expect(getBCSourceType(oc, "tools")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "tools")).To(o.Equal("DockerImage tools:latest"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from Dockerfile with given output ImageStreamTag spec")
		out, err = oc.Run("new-build").Args("-D", testDockerfileContent+"\nENV ok=1", "--to", "origin-test:v1.1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("origin-test", "origin-test", "tools"))
		o.Expect(getBCSourceType(oc, "origin-test")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "origin-test")).To(o.Equal("ImageStreamTag origin-test:v1.1"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from Dockerfile with given output DockerImage spec")
		out, err = oc.Run("new-build").Args("-D", testDockerfileContent+"\nENV ok=1", "--to-docker", "--to", "openshift/origin:v1.1-test").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("origin", "tools"))
		o.Expect(getBCSourceType(oc, "origin")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "origin")).To(o.Equal("DockerImage openshift/origin:v1.1-test"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from Dockerfile with custom name and given output ImageStreamTag spec")
		out, err = oc.Run("new-build").Args("-D", testDockerfileContent+"\nENV ok=1", "--to", "origin-name-test", "--name", "origin-test2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("origin-test2", "origin-name-test"))
		o.Expect(getBCSourceType(oc, "origin-test2")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "origin-test2")).To(o.Equal("ImageStreamTag origin-name-test:latest"))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build from Dockerfile with no output")
		out, err = oc.Run("new-build").Args("-D", testDockerfileContent, "--no-output").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(getExpectedBCOutputMatcher("tools", "tools"))
		o.Expect(getBCSourceType(oc, "tools")).To(o.Equal("Dockerfile"))
		o.Expect(getBCOutputType(oc, "tools")).To(o.Equal(""))
		o.Expect(oc.Run("delete").Args("bc,is", "--all").Execute()).NotTo(o.HaveOccurred())

		g.By("build: ensure output is valid JSON")
		out, err = oc.Run("new-build").Args("--to=tools:json", "-D", testDockerfileContent, "-o", "json").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(json.Valid([]byte(out))).To(o.BeTrue())
	})

	g.It("get buildconfig [apigroup:build.openshift.io]", g.Label("Size:M"), func() {
		err := oc.Run("new-build").Args("-D", testDockerfileContent, "--name", "get-test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, name := range []string{"buildConfigs", "buildConfig", "bc"} {
			out, err := oc.Run("get").Args(name).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			lines := strings.Split(out, "\n")
			o.Expect(lines).To(o.HaveLen(2))
			o.Expect(strings.Fields(lines[0])).To(o.Equal([]string{"NAME", "TYPE", "FROM", "LATEST"}))
			o.Expect(strings.Fields(lines[1])[:3]).To(o.Equal([]string{"get-test", "Docker", "Dockerfile"}))
		}

		out, err := oc.Run("get").Args("is").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		lines := strings.Split(out, "\n")
		o.Expect(lines).To(o.HaveLen(3))
		o.Expect(strings.Fields(lines[0])).To(o.Equal([]string{"NAME", "IMAGE", "REPOSITORY", "TAGS", "UPDATED"}))
		o.Expect(strings.Fields(lines[1])[0]).To(o.Equal("get-test"))
		o.Expect(strings.Fields(lines[1])[1]).To(o.MatchRegexp(fmt.Sprintf("%v/get-test$", oc.Namespace())))
	})

	g.It("patch buildconfig [apigroup:build.openshift.io]", g.Label("Size:M"), func() {
		err := oc.Run("new-build").Args("-D", testDockerfileContent, "--name", "patch-test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		realOutputTo, err := oc.Run("get").Args("bc/patch-test", "--template", bcOutputToNameTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.Run("patch").Args("bc/patch-test", "-p", "{\"spec\":{\"output\":{\"to\":{\"name\":\"different:tag1\"}}}}").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		outputTo, err := oc.Run("get").Args("bc/patch-test", "--template", bcOutputToNameTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(outputTo).To(o.Equal("different:tag1"))
		err = oc.Run("patch").
			Args("bc/patch-test", "-p", fmt.Sprintf("{\"spec\":{\"output\":{\"to\":{\"name\":\"%v:tag1\"}}}}", realOutputTo)).
			Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("complex build", func() {
		var (
			appTemplatePath = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "application-template-dockerbuild.json")
			apiServer       = ""
		)

		getTriggerURL := func(secret, name string) string {
			return fmt.Sprintf("%v/apis/build.openshift.io/v1/namespaces/%v/buildconfigs/ruby-sample-build/webhooks/%v/%v",
				apiServer, oc.Namespace(), secret, name)
		}

		g.JustBeforeEach(func() {
			g.By("getting the api server host")
			out, err := oc.WithoutNamespace().Run("--namespace=default", "status").Args().Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("got status value of: %s", out)
			matcher := regexp.MustCompile(`https?://.*?:\d+`)
			apiServer = matcher.FindString(out)
			o.Expect(apiServer).NotTo(o.BeEmpty())

			g.By("install application")
			appObjects, _, err := oc.Run("process").Args("-f", appTemplatePath, "-l", "build=docker").Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("create").Args("-f", "-").InputString(appObjects).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// make sure the imagestream has the latest tag before trying to test it
			err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				err := oc.Run("get").Args("istag", "ruby-27-centos7:latest").Execute()
				return err == nil, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("webhooks CRUD [apigroup:build.openshift.io]", g.Label("Size:M"), func() {
			g.By("check bc webhooks")
			out, err := oc.Run("describe").Args("buildConfigs", "ruby-sample-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.And(
				o.ContainSubstring("Webhook GitHub"),
				o.ContainSubstring(getTriggerURL("<secret>", "github")),
				o.ContainSubstring("Webhook Generic"),
				o.ContainSubstring(getTriggerURL("<secret>", "generic")),
			))

			g.By("set webhook triggers")
			out, err = oc.Run("set").Args("triggers", "bc/ruby-sample-build", "--from-gitlab").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("triggers updated"))
			out, err = oc.Run("set").Args("triggers", "bc/ruby-sample-build", "--from-bitbucket").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("triggers updated"))

			g.By("list webhooks")
			_, err = oc.Run("start-build").Args("--list-webhooks=blah", "ruby-sample-build").Output()
			o.Expect(err).To(o.HaveOccurred())
			out, err = oc.Run("start-build").Args("--list-webhooks=all", "ruby-sample-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.And(
				o.ContainSubstring(getTriggerURL("<secret>", "generic")),
				o.ContainSubstring(getTriggerURL("<secret>", "github")),
				o.ContainSubstring(getTriggerURL("<secret>", "gitlab")),
				o.ContainSubstring(getTriggerURL("<secret>", "bitbucket")),
			))
			out, err = oc.Run("start-build").Args("--list-webhooks=github", "ruby-sample-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.Equal(getTriggerURL("<secret>", "github")))

			g.By("remove webhook triggers")
			out, err = oc.Run("set").Args("triggers", "bc/ruby-sample-build", "--from-github", "--remove").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("triggers updated"))
			out, err = oc.Run("describe").Args("buildConfigs", "ruby-sample-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).NotTo(o.Or(
				o.ContainSubstring("Webhook GitHub"),
				o.ContainSubstring(getTriggerURL("<secret>", "github")),
			))

			g.By("make sure we describe webhooks using secretReferences properly")
			err = oc.Run("patch").Args("bc/ruby-sample-build", "-p",
				"{\"spec\":{\"triggers\":[{\"github\":{\"secretReference\":{\"name\":\"mysecret\"}},\"type\":\"GitHub\"}]}}",
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			out, err = oc.Run("describe").Args("buildConfigs", "ruby-sample-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.And(
				o.ContainSubstring("Webhook GitHub"),
				o.ContainSubstring(getTriggerURL("<secret>", "github")),
			))
		})

		g.It("start-build [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			g.By("valid build")
			out, err := oc.Run("start-build").Args("--from-webhook", getTriggerURL("secret101", "generic")).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.MatchRegexp("build.build.openshift.io/ruby-sample-build-[0-9] started"))
			buildName := regexp.MustCompile("ruby-sample-build-[0-9]+").FindString(out)
			err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				out, err := oc.Run("get").Args("build", buildName).Output()
				return err == nil && strings.Contains(out, buildName), nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("invalid webhook URL")
			out, err = oc.Run("start-build").Args("--from-webhook", getTriggerURL("secret101", "generic")+"/foo").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("error: server rejected our request"))

			g.By("invalid from istag")
			err = oc.Run("patch").Args("bc/ruby-sample-build", "-p",
				"{\"spec\":{\"strategy\":{\"dockerStrategy\":{\"from\":{\"name\":\"asdf:7\"}}}}}",
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			out, err = oc.Run("start-build").Args("--from-webhook", getTriggerURL("secret101", "generic")).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Error resolving ImageStreamTag asdf:7"))
			err = oc.Run("get").Args("builds").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		// TODO move rest from test/extended/testdata/cmd/test/cmd/builds.sh to here
	})
})

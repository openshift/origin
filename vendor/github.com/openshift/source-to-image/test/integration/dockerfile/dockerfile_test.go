// +build integration

package dockerfile

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build/strategies"
	"github.com/openshift/source-to-image/pkg/scm/git"
)

func TestDockerfileBuild(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: tempdir + string(os.PathSeparator) + "MyDockerfile",
	}
	expected := []string{
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"\"io.openshift.s2i.build.commit.date\"",
		"\"io.openshift.s2i.build.commit.id\"",
		"\"io.openshift.s2i.build.commit.ref\"",
		"\"io.openshift.s2i.build.commit.message\"",
		"\"io.openshift.s2i.build.source-location\"",
		"\"io.openshift.s2i.build.image\"=\"docker.io/centos/nodejs-8-centos7\"",
		"\"io.openshift.s2i.build.commit.author\"",
		"(?m)^COPY upload/src /tmp/src",
		"(?m)^RUN chown -R 1001:0.* /tmp/src",
		// Ensure we are using the default image user when running assemble
		"(?m)^USER 1001\n.+\n.+\nRUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "MyDockerfile"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildDefaultDockerfile(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: tempdir + string(os.PathSeparator),
	}
	expected := []string{
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"\"io.openshift.s2i.build.commit.date\"",
		"\"io.openshift.s2i.build.commit.id\"",
		"\"io.openshift.s2i.build.commit.ref\"",
		"\"io.openshift.s2i.build.commit.message\"",
		"\"io.openshift.s2i.build.source-location\"",
		"\"io.openshift.s2i.build.image\"=\"docker.io/centos/nodejs-8-centos7\"",
		"\"io.openshift.s2i.build.commit.author\"",
		"(?m)^COPY upload/src /tmp/src",
		"(?m)^RUN chown -R 1001:0.* /tmp/src",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "Dockerfile"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildEnv(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{
			{
				Name:  "key1",
				Value: "value1",
			},
			{
				Name:  "key2",
				Value: "value2",
			},
		},
		Labels: map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"key1=\"value1\"",
		"key2=\"value2\"",
	}
	runDockerfileTest(t, config, expected, nil, nil, false)
}

func TestDockerfileBuildLabels(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels: map[string]string{"label1": "value1",
			"label2":                               "value2",
			"io.openshift.s2i.build.commit.author": "shadowman"},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"\"io.openshift.s2i.build.commit.date\"",
		"\"io.openshift.s2i.build.commit.id\"",
		"\"io.openshift.s2i.build.commit.ref\"",
		"\"io.openshift.s2i.build.commit.message\"",
		"\"io.openshift.s2i.build.source-location\"",
		"\"io.openshift.s2i.build.image\"=\"docker.io/centos/nodejs-8-centos7\"",
		"\"io.openshift.s2i.build.commit.author\"=\"shadowman\"",
		"\"label1\"=\"value1\"",
		"\"label2\"=\"value2\"",
	}
	runDockerfileTest(t, config, expected, nil, nil, false)
}

func TestDockerfileBuildInjections(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	injection1 := filepath.Join(tempdir, "injection1")
	err = os.Mkdir(injection1, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err = ioutil.TempFile(injection1, "injectfile-")
		if err != nil {
			t.Errorf("Unable to create injection file: %v", err)
		}
	}

	injection2 := filepath.Join(tempdir, "injection2")
	err = os.Mkdir(injection2, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}
	_, err = ioutil.TempFile(injection2, "injectfile-2")
	if err != nil {
		t.Errorf("Unable to create injection file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "/workdir",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Injections: api.VolumeList{
			{
				Source:      injection1,
				Destination: "injection1",
				Keep:        false,
			},
			{
				Source:      injection2,
				Destination: "/destination/injection2",
				Keep:        true,
			},
		},
		Destination: "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	// strip the C: from windows paths because it's not valid in the middle of a path
	// like upload/injections/C:/tempdir/injection1
	trimmedInjection1 := filepath.ToSlash(strings.TrimPrefix(injection1, filepath.VolumeName(injection1)))
	trimmedInjection2 := filepath.ToSlash(strings.TrimPrefix(injection2, filepath.VolumeName(injection2)))

	expected := []string{
		"(?m)^COPY upload/injections" + trimmedInjection1 + " /workdir/injection1",
		"(?m)^RUN chown -R 1001:0.* /workdir/injection1",
		"(?m)^COPY upload/injections" + trimmedInjection2 + " /destination/injection2",
		"(?m)^RUN chown -R 1001:0.* /destination/injection2",
		"(?m)^RUN rm /workdir/injection1/injectfile-",
		"    rm /workdir/injection1/injectfile-",
	}
	notExpected := []string{
		"rm -rf /destination/injection2",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/injections"+trimmedInjection1),
		filepath.Join(tempdir, "upload/injections"+trimmedInjection2),
	}
	runDockerfileTest(t, config, expected, notExpected, expectedFiles, false)
}

func TestDockerfileBuildScriptsURLAssemble(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	assemble := filepath.Join(tempdir, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildScriptsURLRun(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	run := filepath.Join(tempdir, "run")
	_, err = os.OpenFile(run, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create run file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /destination/scripts/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/run"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildScriptsURLNone(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	runDockerfileTest(t, config, nil, nil, nil, true)
}

func TestDockerfileBuildSourceScriptsAssemble(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	assemble := filepath.Join(sourcescripts, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:    true,
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildSourceScriptsRun(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	run := filepath.Join(sourcescripts, "run")
	_, err = os.OpenFile(run, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create run file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:    true,
		ScriptsURL:   "",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /destination/scripts/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/run"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

// TestDockerfileBuildScriptsURLImage tests the behavior if the ScriptsURL
// is set to an image:// URL. In this case we blind trust that the image
// contains all of the s2i scripts at the given directory, regardless
// of what is contained in the source.
func TestDockerfileBuildScriptsURLImage(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	assemble := filepath.Join(sourcescripts, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:    true,
		ScriptsURL:   "image:///usr/custom/s2i",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^RUN /usr/custom/s2i/assemble",
		"(?m)^CMD /usr/custom/s2i/run",
	}
	notExpected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
	}
	runDockerfileTest(t, config, expected, notExpected, nil, false)
}

func TestDockerfileBuildImageScriptsURLAssemble(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	assemble := filepath.Join(tempdir, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ImageScriptsURL: "file://" + filepath.ToSlash(tempdir),
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildImageScriptsURLRun(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	run := filepath.Join(tempdir, "run")
	_, err = os.OpenFile(run, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create run file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ImageScriptsURL: "file://" + filepath.ToSlash(tempdir),
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /destination/scripts/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/run"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildImageScriptsURLImage(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	assemble := filepath.Join(sourcescripts, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ForceCopy:       true,
		ImageScriptsURL: "image:///usr/custom/s2i",
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/custom/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileBuildScriptsAndImageURL(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	assemble := filepath.Join(tempdir, "assemble")
	_, err = os.OpenFile(assemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:      "file://" + filepath.ToSlash(tempdir),
		ImageScriptsURL: "image:///usr/some/dir",
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/some/dir/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/src/server.js"),
		filepath.Join(tempdir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

// TestDockerfileBuildScriptsAndImageURLConflicts tests if both
// the ScriptsURL and ImageScriptsURL point to a non-image directory.
// In this event, the ScriptsURL value should take precedence.
func TestDockerfileBuildScriptsAndImageURLConflicts(t *testing.T) {
	scriptsTempDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(scriptsTempDir)

	imageTempDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(imageTempDir)

	outputDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(outputDir)

	scriptsAssemble := filepath.Join(scriptsTempDir, "assemble")
	assembleData := []byte("#!/bin/bash\necho \"Hello World!\"")
	err = ioutil.WriteFile(scriptsAssemble, assembleData, 0666)
	if err != nil {
		t.Errorf("Unable to create image assemble file: %v", err)
	}

	imageAssemble := filepath.Join(imageTempDir, "assemble")
	_, err = os.OpenFile(imageAssemble, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create assemble file: %v", err)
	}

	config := &api.Config{
		BuilderImage:    "docker.io/centos/nodejs-8-centos7",
		AssembleUser:    "",
		ImageWorkDir:    "",
		Source:          git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:      "file://" + filepath.ToSlash(scriptsTempDir),
		ImageScriptsURL: "file://" + filepath.ToSlash(imageTempDir),
		Injections:      api.VolumeList{},
		Destination:     "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(outputDir, "Dockerfile"),
	}
	expected := []string{
		"(?m)^COPY upload/scripts /destination/scripts",
		"(?m)^RUN chown -R 1001:0.* /destination/scripts",
		"(?m)^RUN /destination/scripts/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(outputDir, "upload/src/server.js"),
		filepath.Join(outputDir, "upload/scripts/assemble"),
	}
	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
	dockerfileAssemble, err := ioutil.ReadFile(filepath.Join(outputDir, "upload/scripts/assemble"))
	if err != nil {
		t.Errorf("Failed to read uploaded assemble script: %v", err)
	}
	if string(dockerfileAssemble) != string(assembleData) {
		t.Errorf("Expected uploaded assemble script:\n\n%s\n\nto be:\n\n%s", dockerfileAssemble, assembleData)
	}
}

func TestDockerfileIncrementalBuild(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "",
		Tag:          "test:tag",
		Injections:   api.VolumeList{},
		Destination:  "",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER 1001",
		"(?m)^RUN if \\[ -s /usr/libexec/s2i/save-artifacts \\]; then /usr/libexec/s2i/save-artifacts > /tmp/artifacts.tar; else touch /tmp/artifacts.tar; fi",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"(?m)^COPY --from=cached /tmp/artifacts.tar /tmp/artifacts.tar",
		"(?m)^RUN chown -R 1001:0.* /tmp/artifacts.tar",
		"if \\[ -s /tmp/artifacts.tar \\]; then mkdir -p /tmp/artifacts; tar -xf /tmp/artifacts.tar -C /tmp/artifacts; fi",
		"rm /tmp/artifacts.tar",
		"(?m)^COPY upload/src /tmp/src",
		"(?m)^RUN chown -R 1001:0.* /tmp/src",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}

	runDockerfileTest(t, config, expected, nil, nil, false)
}

func TestDockerfileIncrementalSourceSave(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	sourcecode := filepath.Join(tempdir, "sourcecode")
	sourcescripts := filepath.Join(sourcecode, ".s2i", "bin")
	err = os.MkdirAll(sourcescripts, 0777)
	if err != nil {
		t.Errorf("Unable to create injection dir: %v", err)
	}

	saveArtifacts := filepath.Join(sourcescripts, "save-artifacts")
	_, err = os.OpenFile(saveArtifacts, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create save-artifacts file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("file:///" + filepath.ToSlash(sourcecode)),
		ScriptsURL:   "",
		Tag:          "test:tag",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER root\n",
		"(?m)^COPY upload/scripts/save-artifacts /destination/scripts/save-artifacts",
		"(?m)^RUN chown .*1001:0 /destination/scripts/save-artifacts",
		"(?m)^USER 1001\nRUN if \\[ -s /destination/scripts/save-artifacts \\]; then /destination/scripts/save-artifacts > /tmp/artifacts.tar;",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"mkdir -p /destination/artifacts",
		"tar -xf /tmp/artifacts.tar -C /destination/artifacts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/save-artifacts"),
	}

	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileIncrementalSaveURL(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	saveArtifacts := filepath.Join(tempdir, "save-artifacts")
	_, err = os.OpenFile(saveArtifacts, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		t.Errorf("Unable to create save-artifacts file: %v", err)
	}

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		ScriptsURL:   "file://" + filepath.ToSlash(tempdir),
		Tag:          "test:tag",
		Injections:   api.VolumeList{},
		Destination:  "/destination",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER root\n",
		"(?m)^COPY upload/scripts/save-artifacts /destination/scripts/save-artifacts",
		"(?m)^RUN chown 1001:0 /destination/scripts/save-artifacts",
		"(?m)^USER 1001\nRUN if \\[ -s /destination/scripts/save-artifacts \\]; then /destination/scripts/save-artifacts > /tmp/artifacts.tar;",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"mkdir -p /destination/artifacts",
		"tar -xf /tmp/artifacts.tar -C /destination/artifacts",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}
	expectedFiles := []string{
		filepath.Join(tempdir, "upload/scripts/save-artifacts"),
	}

	runDockerfileTest(t, config, expected, nil, expectedFiles, false)
}

func TestDockerfileIncrementalTag(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage:       "docker.io/centos/nodejs-8-centos7",
		AssembleUser:       "",
		ImageWorkDir:       "",
		Incremental:        true,
		Source:             git.MustParse("https://github.com/sclorg/nodejs-ex"),
		Tag:                "test:tag",
		IncrementalFromTag: "incremental:tag",

		Environment: api.EnvironmentList{},
		Labels:      map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM incremental:tag as cached",
		"/usr/libexec/s2i/save-artifacts > /tmp/artifacts.tar",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"mkdir -p /tmp/artifacts",
		"tar -xf /tmp/artifacts.tar -C /tmp/artifacts",
		"rm /tmp/artifacts.tar",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}

	runDockerfileTest(t, config, expected, nil, nil, false)
}

func TestDockerfileIncrementalAssembleUser(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tempdir)

	config := &api.Config{
		BuilderImage: "docker.io/centos/nodejs-8-centos7",
		AssembleUser: "2250",
		ImageWorkDir: "",
		Incremental:  true,
		Source:       git.MustParse("https://github.com/sclorg/nodejs-ex"),
		Tag:          "test:tag",
		Environment:  api.EnvironmentList{},
		Labels:       map[string]string{},

		AsDockerfile: filepath.Join(tempdir, "Dockerfile"),
	}

	expected := []string{
		"(?m)^FROM test:tag as cached\n#.+\nUSER 2250",
		"/usr/libexec/s2i/save-artifacts > /tmp/artifacts.tar",
		"(?m)^FROM docker.io/centos/nodejs-8-centos7",
		"(?m)^COPY --from=cached /tmp/artifacts.tar /tmp/artifacts.tar",
		"(?m)^RUN chown -R 2250:0 .*/tmp/artifacts.tar",
		"mkdir -p /tmp/artifacts",
		"tar -xf /tmp/artifacts.tar -C /tmp/artifacts",
		"rm /tmp/artifacts.tar",
		"(?m)^RUN /usr/libexec/s2i/assemble",
		"(?m)^CMD /usr/libexec/s2i/run",
	}

	runDockerfileTest(t, config, expected, nil, nil, false)
}

func TestDockerfileLocalSource(t *testing.T) {
	localTempDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(localTempDir)

	outputDir, err := ioutil.TempDir("", "s2i-dockerfiletest-dir")
	if err != nil {
		t.Errorf("Unable to create temporary directory: %v", err)
	}
	defer os.RemoveAll(outputDir)

	config := &api.Config{
		BuilderImage: "sti_test/sti-fake",
		Source:       git.MustParse("file:///" + filepath.ToSlash(localTempDir)),
		AsDockerfile: filepath.Join(outputDir, "Dockerfile"),
	}

	dirTree := []string{
		"foo/bar",
		"foo/baz/foobar",
	}
	for _, dirName := range dirTree {
		err = os.MkdirAll(filepath.Join(localTempDir, dirName), 0777)
		if err != nil {
			t.Errorf("Unable to create dir: %v", err)
		}
	}

	fileTree := []string{
		"foo/a_file",
		"foo/bar/a_file",
		"foo/bar/another_file",
		"foo/baz/foobar/a_file",
	}
	for _, fileName := range fileTree {
		dummyContent := []byte("Hello World!")
		err = ioutil.WriteFile(filepath.Join(localTempDir, fileName), dummyContent, 0666)
		if err != nil {
			t.Errorf("Unable to create file: %v", err)
		}
	}

	expectedFiles := []string{
		filepath.Join(outputDir, "upload/src/foo/a_file"),
		filepath.Join(outputDir, "upload/src/foo/bar"),
		filepath.Join(outputDir, "upload/src/foo/bar/a_file"),
		filepath.Join(outputDir, "upload/src/foo/bar/another_file"),
		filepath.Join(outputDir, "upload/src/foo/baz/foobar"),
		filepath.Join(outputDir, "upload/src/foo/baz/foobar/a_file"),
	}

	runDockerfileTest(t, config, nil, nil, expectedFiles, false)

	s2iignore := filepath.Join(localTempDir, ".s2iignore")
	s2iignoreDate := []byte("dummy\n#skip_file\nfoo/bar/another_file\nfoo/baz/foobar")
	err = ioutil.WriteFile(s2iignore, s2iignoreDate, 0666)
	if err != nil {
		t.Errorf("Unable to create .s2iignore file: %v", err)
	}

	expectedFiles = []string{
		filepath.Join(outputDir, "upload/src/foo/a_file"),
		filepath.Join(outputDir, "upload/src/foo/bar"),
		filepath.Join(outputDir, "upload/src/foo/bar/a_file"),
	}

	runDockerfileTest(t, config, nil, nil, expectedFiles, false)
}

func runDockerfileTest(t *testing.T, config *api.Config, expected []string, notExpected []string, expectedFiles []string, expectFailure bool) {

	b, _, err := strategies.GetStrategy(nil, config)
	if err != nil {
		t.Fatalf("Cannot create a new builder.")
	}
	resp, err := b.Build(config)
	if expectFailure {
		if err == nil || resp.Success {
			t.Errorf("The build succeded when it should have failed. Success: %t, error: %v", resp.Success, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("An error occurred during the build: %v", err)
	}
	if !resp.Success {
		t.Fatalf("The build failed when it should have succeeded.")
	}

	filebytes, err := ioutil.ReadFile(config.AsDockerfile)
	if err != nil {
		t.Fatalf("An error occurred reading the dockerfile: %v", err)
	}
	dockerfile := string(filebytes)

	buf := bytes.NewBuffer(filebytes)
	_, err = parser.Parse(buf)
	if err != nil {
		t.Fatalf("An error occurred parsing the dockerfile: %v\n%s", err, dockerfile)
	}

	for _, s := range expected {
		reg, err := regexp.Compile(s)
		if err != nil {
			t.Fatalf("failed to compile regex %q: %v", s, err)
		}
		if !reg.MatchString(dockerfile) {
			t.Fatalf("Expected dockerfile to contain %s, it did not: \n%s", s, dockerfile)
		}
	}
	for _, s := range notExpected {
		reg, err := regexp.Compile(s)
		if err != nil {
			t.Fatalf("failed to compile regex %q: %v", s, err)
		}
		if reg.MatchString(dockerfile) {
			t.Fatalf("Expected dockerfile not to contain %s, it did: \n%s", s, dockerfile)
		}
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("Did not find expected file %s, ", f)
		}
	}
}

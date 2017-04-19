// +build conformance

package dockerclient

import (
	"archive/tar"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/builder/dockerfile/command"
	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/fileutils"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/openshift/imagebuilder"
)

var compareLayers = flag.Bool("compare-layers", false, "If true, compare each generated layer for equivalence")

type conformanceTest struct {
	Dockerfile string
	Git        string
	ContextDir string
	Args       map[string]string
	Ignore     []ignoreFunc
	PostClone  func(dir string) error
}

// TestConformance* compares the result of running the direct build against a
// sequential docker build. A dockerfile and git repo is loaded, then each step
// in the file is run sequentially, committing after each step. The generated
// image.Config and the resulting filesystems are compared. The next step reuses
// the previously generated layer and performs an incremental diff. This ensures
// that each step is functionally equivalent.
//
// Deviations:
// * Builds run at different times
//   * Modification timestamps are ignored on files
//   * Some processes (gem install) result in files created in the image that
//     have different content because of that (timestamps in files). We treat
//     a file that is identical except for size within 10 bytes and neither old
//     or new is zero bytes to be identical.
// * Docker container commit with ENV FOO=BAR and a Docker build with line
//   ENV FOO=BAR will generate an image with FOO=BAR in different positions
//   (commit places the variable first, build: last). We try to align the
//   generated environment variable to ensure they are equal.
// * The parent image ID is ignored.
//
// TODO: .dockerignore
// TODO: check context dir
// TODO: ONBUILD
// TODO: ensure that the final built image has the right UIDs
//
func TestConformanceInternal(t *testing.T) {
	testCases := []conformanceTest{
		{
			ContextDir: "testdata/dir",
		},
		// TODO: Fix this test
		// {
		// 	ContextDir: "testdata/ignore",
		// },
		{
			Dockerfile: "testdata/Dockerfile.env",
		},
		{
			Dockerfile: "testdata/Dockerfile.edgecases",
		},
		{
			Dockerfile: "testdata/Dockerfile.exposedefault",
		},
		{
			Dockerfile: "testdata/Dockerfile.add",
		},
		{
			Dockerfile: "testdata/Dockerfile.args",
			Args:       map[string]string{"BAR": "first"},
		},
		/*{ // uncomment when docker allows this
			Dockerfile: "testdata/Dockerfile.args",
			Args:       map[string]string{"BAZ": "first"},
		},*/
		{
			ContextDir: "testdata/wildcard",
		},
		{
			ContextDir: "testdata/volume",
		},
		{
			ContextDir: "testdata/volumerun",
		},
	}

	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	for i, test := range testCases {
		conformanceTester(t, c, test, i, *compareLayers)
	}
}

// TestConformanceExternal applies external repo testing that may be more expensive or
// change more frequently.
func TestConformanceExternal(t *testing.T) {
	testCases := []conformanceTest{
		{
			// Tests user ownership change under COPY
			Git: "https://github.com/openshift/ruby-hello-world.git",
		},
		{
			// Tests Non-default location dockerfile
			Dockerfile: "Dockerfile.build",
			Git:        "https://github.com/docker-library/hello-world.git",
			PostClone: func(dir string) error {
				return os.Remove(filepath.Join(dir, ".dockerignore"))
			},
		},
		{
			// Tests COPY and other complex interactions of ENV
			ContextDir: "9.3",
			Dockerfile: "9.3/Dockerfile",
			Git:        "https://github.com/docker-library/postgres.git",
			Ignore: []ignoreFunc{
				func(a, b *tar.Header) bool {
					switch {
					case (a != nil) == (b != nil):
						return false
					case a != nil:
						return strings.HasPrefix(a.Name, "etc/ssl/certs/")
					case b != nil:
						return strings.HasPrefix(b.Name, "etc/ssl/certs/")
					default:
						return false
					}
				},
			},
		},
	}

	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	for i, test := range testCases {
		conformanceTester(t, c, test, i, *compareLayers)
	}
}

func TestTransientMount(t *testing.T) {
	c, err := docker.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	e := NewClientExecutor(c)
	e.AllowPull = true
	e.Directory = "testdata"
	e.TransientMounts = []Mount{
		{SourcePath: "dir", DestinationPath: "/mountdir"},
		{SourcePath: "Dockerfile.env", DestinationPath: "/mountfile"},
	}
	e.Tag = fmt.Sprintf("conformance%d", rand.Int63())

	defer c.RemoveImage(e.Tag)

	out := &bytes.Buffer{}
	e.Out = out
	b, node, err := imagebuilder.NewBuilderForReader(bytes.NewBufferString("FROM busybox\nRUN ls /mountdir/subdir\nRUN cat /mountfile\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Build(b, node); err != nil {
		t.Fatalf("unable to build image: %v", err)
	}
	if !strings.Contains(out.String(), "ENV name=value\n") {
		t.Errorf("did not find expected output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "file2\n") {
		t.Errorf("did not find expected output:\n%s", out.String())
	}

	result, err := testContainerOutput(c, e.Tag, []string{"/bin/sh", "-c", "ls -al /mountdir"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "subdir") {
		t.Errorf("did not find expected output:\n%s", result)
	}
	result, err = testContainerOutput(c, e.Tag, []string{"/bin/sh", "-c", "cat /mountfile"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result, "ENV name=value\n") {
		t.Errorf("did not find expected output:\n%s", result)
	}
}

func testContainerOutput(c *docker.Client, tag string, command []string) (string, error) {
	container, err := c.CreateContainer(docker.CreateContainerOptions{
		Name: tag + "-test",
		Config: &docker.Config{
			Image:      tag,
			Entrypoint: command,
			Cmd:        nil,
		},
	})
	if err != nil {
		return "", err
	}
	defer c.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID})
	if err := c.StartContainer(container.ID, nil); err != nil {
		return "", err
	}
	code, err := c.WaitContainer(container.ID)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("unrecognized exit code: %d", code)
	}
	out := &bytes.Buffer{}
	if err := c.Logs(docker.LogsOptions{Container: container.ID, Stdout: true, OutputStream: out}); err != nil {
		return "", fmt.Errorf("unable to get logs: %v", err)
	}
	return out.String(), nil
}

func conformanceTester(t *testing.T, c *docker.Client, test conformanceTest, i int, deep bool) {
	dockerfile := test.Dockerfile
	if len(dockerfile) == 0 {
		dockerfile = "Dockerfile"
	}
	tmpDir, err := ioutil.TempDir("", "dockerbuild-conformance-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dir := tmpDir
	contextDir := filepath.Join(dir, test.ContextDir)
	dockerfilePath := filepath.Join(dir, dockerfile)

	// clone repo or copy the Dockerfile
	var input string
	switch {
	case len(test.Git) > 0:
		input = test.Git
		cmd := exec.Command("git", "clone", test.Git, dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("unable to clone %q: %v\n%s", test.Git, err, out)
			return
		}

		if test.PostClone != nil {
			if err := test.PostClone(dir); err != nil {
				t.Errorf("unable to fixup clone: %v", err)
				return
			}
		}

	case len(test.Dockerfile) > 0:
		input = dockerfile
		dockerfilePath = filepath.Join(dir, "Dockerfile")
		if _, err := fileutils.CopyFile(filepath.Join("", dockerfile), dockerfilePath); err != nil {
			t.Fatal(err)
		}
		dockerfile = "Dockerfile"

	default:
		input = filepath.Join(test.ContextDir, dockerfile)
		dockerfilePath = input
		contextDir = test.ContextDir
		dir = test.ContextDir
	}

	// read the dockerfile
	data, err := ioutil.ReadFile(dockerfilePath)
	if err != nil {
		t.Errorf("%d: unable to read Dockerfile %q: %v", i, input, err)
		return
	}
	node, err := imagebuilder.ParseDockerfile(bytes.NewBuffer(data))
	if err != nil {
		t.Errorf("%d: can't parse Dockerfile %q: %v", i, input, err)
		return
	}
	from, err := imagebuilder.NewBuilder().From(node)
	if err != nil {
		t.Errorf("%d: can't get base FROM %q: %v", i, input, err)
		return
	}
	nameFormat := "conformance-dockerbuild-%d-%s-%d"

	var toDelete []string
	steps := node.Children
	lastImage := from

	ignoreSmallFileChange := func(a, b *tar.Header) bool {
		if a == nil || b == nil {
			return false
		}
		diff := a.Size - b.Size
		if differOnlyByFileSize(a, b, 10) {
			t.Logf("WARNING: %s differs only in size by %d bytes, probably a timestamp value change", a.Name, diff)
			return true
		}
		return false
	}

	if deep {
		// execute each step on both Docker build and the direct builder, comparing as we
		// go
		fail := false
		for j := range steps {
			testFile := dockerfileWithFrom(lastImage, steps[j:j+1])

			nameDirect := fmt.Sprintf(nameFormat, i, "direct", j)
			nameDocker := fmt.Sprintf(nameFormat, i, "docker", j)

			// run docker build
			if err := ioutil.WriteFile(dockerfilePath, []byte(testFile), 0600); err != nil {
				t.Errorf("%d: unable to update Dockerfile %q: %v", i, dockerfilePath, err)
				break
			}
			in, err := archive.TarWithOptions(dir, &archive.TarOptions{IncludeFiles: []string{"."}})
			if err != nil {
				t.Errorf("%d: unable to generate build context %q: %v", i, dockerfilePath, err)
				break
			}
			out := &bytes.Buffer{}
			if err := c.BuildImage(docker.BuildImageOptions{
				Name:                nameDocker,
				Dockerfile:          dockerfile,
				RmTmpContainer:      true,
				ForceRmTmpContainer: true,
				InputStream:         in,
				OutputStream:        out,
			}); err != nil {
				in.Close()
				t.Errorf("%d: unable to build Docker image %q: %v\n%s", i, test.Git, err, out)
				break
			}
			toDelete = append(toDelete, nameDocker)

			// run direct build
			e := NewClientExecutor(c)
			out = &bytes.Buffer{}
			e.Out, e.ErrOut = out, out
			e.Directory = contextDir
			e.Tag = nameDirect
			b, node, err := imagebuilder.NewBuilderForReader(bytes.NewBufferString(testFile), test.Args)
			if err != nil {
				t.Fatalf("%d: %v", i, err)
			}
			if err := e.Build(b, node); err != nil {
				t.Errorf("%d: failed to build step %d in dockerfile %q: %s\n%s", i, j, dockerfilePath, steps[j].Original, out)
				break
			}
			toDelete = append(toDelete, nameDirect)

			// only compare filesystem on layers that change the filesystem
			mutation := steps[j].Value == command.Add || steps[j].Value == command.Copy || steps[j].Value == command.Run
			// metadata must be strictly equal
			if !equivalentImages(
				t, c, nameDocker, nameDirect, mutation,
				metadataEqual,
				append(ignoreFuncs{ignoreSmallFileChange}, test.Ignore...)...,
			) {
				t.Errorf("%d: layered Docker build was not equivalent to direct layer image metadata %s", i, input)
				fail = true
			}

			lastImage = nameDocker
		}

		if fail {
			t.Fatalf("%d: Conformance test failed for %s", i, input)
		}

	} else {
		exclude, _ := imagebuilder.ParseDockerignore(dir)
		//exclude = append(filtered, ".dockerignore")
		in, err := archive.TarWithOptions(dir, &archive.TarOptions{IncludeFiles: []string{"."}, ExcludePatterns: exclude})
		if err != nil {
			t.Errorf("%d: unable to generate build context %q: %v", i, dockerfilePath, err)
			return
		}
		out := &bytes.Buffer{}
		nameDocker := fmt.Sprintf(nameFormat, i, "docker", 0)
		var args []docker.BuildArg
		for k, v := range test.Args {
			args = append(args, docker.BuildArg{Name: k, Value: v})
		}
		if err := c.BuildImage(docker.BuildImageOptions{
			Name:                nameDocker,
			Dockerfile:          dockerfile,
			RmTmpContainer:      true,
			ForceRmTmpContainer: true,
			InputStream:         in,
			OutputStream:        out,
			BuildArgs:           args,
		}); err != nil {
			in.Close()
			t.Errorf("%d: unable to build Docker image %q: %v\n%s", i, test.Git, err, out)
			return
		}
		lastImage = nameDocker
		toDelete = append(toDelete, nameDocker)
	}

	// if we ran more than one step, compare the squashed output with the docker build output
	if len(steps) > 1 || !deep {
		nameDirect := fmt.Sprintf(nameFormat, i, "direct", len(steps)-1)
		e := NewClientExecutor(c)
		out := &bytes.Buffer{}
		e.Out, e.ErrOut = out, out
		e.Directory = contextDir
		e.Tag = nameDirect
		b, node, err := imagebuilder.NewBuilderForReader(bytes.NewBuffer(data), test.Args)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		if err := e.Build(b, node); err != nil {
			t.Errorf("%d: failed to build complete image in %q: %v\n%s", i, input, err, out)
		} else {
			if !equivalentImages(
				t, c, lastImage, nameDirect, true,
				// metadata should be loosely equivalent, but because we squash and because of limitations
				// in docker commit, there are some differences
				metadataLayerEquivalent,
				append(ignoreFuncs{
					ignoreSmallFileChange,
					// the direct dockerfile contains all steps, the layered image is synthetic from our previous
					// test and so only contains the last layer
					ignoreDockerfileSize(dockerfile),
				}, test.Ignore...)...,
			) {
				t.Errorf("%d: full Docker build was not equivalent to squashed image metadata %s", i, input)
			}
		}
	}

	for _, s := range toDelete {
		c.RemoveImageExtended(s, docker.RemoveImageOptions{Force: true})
	}
}

// ignoreFunc returns true if the difference between the two can be ignored
type ignoreFunc func(a, b *tar.Header) bool

type ignoreFuncs []ignoreFunc

func (fns ignoreFuncs) Ignore(a, b *tar.Header) bool {
	for _, fn := range fns {
		if fn(a, b) {
			return true
		}
	}
	return false
}

// metadataFunc returns true if the metadata is equivalent
type metadataFunc func(a, b *docker.Config) bool

func normalizeOutputMetadata(a, b *docker.Config) {
	// old docker servers can report no args escaped
	if !a.ArgsEscaped && b.ArgsEscaped {
		b.ArgsEscaped = false
	}
	if a.Entrypoint == nil && len(b.Entrypoint) == 0 {
		// we are forced to set Entrypoint [] to reset the entrypoint
		b.Entrypoint = nil
	}
	// Serialization of OnBuild is omitempty, which means it may be nil or empty depending on
	// docker version
	if len(a.OnBuild) == len(b.OnBuild) && len(a.OnBuild) == 0 {
		b.OnBuild = a.OnBuild
	}
}

// metadataEqual checks that the metadata of two images is directly equivalent.
func metadataEqual(a, b *docker.Config) bool {
	// compare output metadata
	a.Image, b.Image = "", ""
	a.Hostname, b.Hostname = "", ""
	e1, e2 := envMap(a.Env), envMap(b.Env)
	if !reflect.DeepEqual(e1, e2) {
		return false
	}
	normalizeOutputMetadata(a, b)
	a.Env, b.Env = nil, nil
	if !reflect.DeepEqual(a, b) {
		return false
	}
	return true
}

// metadataLayerEquivalent returns true if the last layer of a is equivalent to b, assuming
// that b is squashed over multiple layers, and a is not. b, for instance, will have an empty
// slice entrypoint, while a would have a nil entrypoint.
func metadataLayerEquivalent(a, b *docker.Config) bool {
	normalizeOutputMetadata(a, b)
	if len(a.OnBuild) == 1 && len(b.OnBuild) > 0 && a.OnBuild[0] == b.OnBuild[len(b.OnBuild)-1] {
		// a layered file will only contain the last OnBuild statement
		b.OnBuild = a.OnBuild
	}
	return metadataEqual(a, b)
}

// equivalentImages executes the provided checks against two docker images, returning true
// if the images are equivalent, and recording a test suite error in any other condition.
func equivalentImages(t *testing.T, c *docker.Client, a, b string, testFilesystem bool, metadataFn metadataFunc, ignoreFns ...ignoreFunc) bool {
	imageA, err := c.InspectImage(a)
	if err != nil {
		t.Errorf("can't get image %q: %v", a, err)
		return false
	}
	imageB, err := c.InspectImage(b)
	if err != nil {
		t.Errorf("can't get image %q: %v", b, err)
		return false
	}

	if !metadataFn(imageA.Config, imageB.Config) {
		t.Errorf("generated image metadata did not match:\n%#v\n%#v", imageA.Config, imageB.Config)
		return false
	}

	// for mutation commands, check the layer diff
	if testFilesystem {
		differs, onlyA, onlyB, err := compareImageFS(c, a, b)
		if err != nil {
			t.Errorf("can't calculate FS differences %q: %v", a, err)
			return false
		}
		for k, v := range differs {
			if ignoreFuncs(ignoreFns).Ignore(v[0], v[1]) {
				delete(differs, k)
				continue
			}
			t.Errorf("%s %s differs:\n%#v\n%#v", a, k, v[0], v[1])
		}
		for k, v := range onlyA {
			if ignoreFuncs(ignoreFns).Ignore(v, nil) {
				delete(onlyA, k)
				continue
			}
		}
		for k, v := range onlyB {
			if ignoreFuncs(ignoreFns).Ignore(nil, v) {
				delete(onlyB, k)
				continue
			}
		}
		if len(onlyA)+len(onlyB)+len(differs) > 0 {
			t.Errorf("a=%v b=%v diff=%v", onlyA, onlyB, differs)
			return false
		}
	}
	return true
}

// dockerfileWithFrom returns the contents of a new docker file with a different
// FROM as the first line.
func dockerfileWithFrom(from string, steps []*parser.Node) string {
	lines := []string{}
	lines = append(lines, fmt.Sprintf("FROM %s", from))
	for _, step := range steps {
		lines = append(lines, step.Original)
	}
	return strings.Join(lines, "\n")
}

// envMap returns a map from a list of environment variables.
func envMap(env []string) map[string]string {
	out := make(map[string]string)
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			out[envVar] = ""
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}

// differOnlyByFileSize returns true iff the headers differ only by size, but
// that differences is less than within bytes.
func differOnlyByFileSize(a, b *tar.Header, within int64) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Size == b.Size {
		return false
	}

	diff := a.Size - b.Size
	if diff < 0 {
		diff = diff * -1
	}
	if diff < within && a.Size != 0 && b.Size != 0 {
		a.Size = b.Size
		if reflect.DeepEqual(a, b) {
			return true
		}
	}
	return false
}

// ignore Dockerfile being different, artifact of this test
func ignoreDockerfileSize(dockerfile string) ignoreFunc {
	return func(a, b *tar.Header) bool {
		if a == nil || b == nil {
			return false
		}
		if !strings.HasSuffix(a.Name, dockerfile) {
			return false
		}
		if a.Size != b.Size {
			a.Size = b.Size
			return reflect.DeepEqual(a, b)
		}
		return false
	}
}

// compareImageFS exports the file systems of two images and returns a map
// of files that differ in any way (modification time excluded), only exist in
// image A, or only existing in image B.
func compareImageFS(c *docker.Client, a, b string) (differ map[string][]*tar.Header, onlyA, onlyB map[string]*tar.Header, err error) {
	fsA, err := imageFSMetadata(c, a)
	if err != nil {
		return nil, nil, nil, err
	}
	fsB, err := imageFSMetadata(c, b)
	if err != nil {
		return nil, nil, nil, err
	}
	differ = make(map[string][]*tar.Header)
	onlyA = make(map[string]*tar.Header)
	onlyB = fsB
	for k, v1 := range fsA {
		v2, ok := fsB[k]
		if !ok {
			onlyA[k] = v1
			continue
		}
		delete(onlyB, k)
		// we ignore modification time differences
		v1.ModTime = time.Time{}
		v2.ModTime = time.Time{}
		if !reflect.DeepEqual(v1, v2) {
			differ[k] = []*tar.Header{v1, v2}
		}
	}
	return differ, onlyA, onlyB, nil
}

// imageFSMetadata creates a container and reads the filesystem metadata out of the archive.
func imageFSMetadata(c *docker.Client, name string) (map[string]*tar.Header, error) {
	container, err := c.CreateContainer(docker.CreateContainerOptions{Name: name + "-export", Config: &docker.Config{Image: name}})
	if err != nil {
		return nil, err
	}
	defer c.RemoveContainer(docker.RemoveContainerOptions{ID: container.ID, RemoveVolumes: true, Force: true})

	ch := make(chan struct{})
	result := make(map[string]*tar.Header)
	r, w := io.Pipe()
	go func() {
		defer close(ch)
		out := tar.NewReader(r)
		for {
			h, err := out.Next()
			if err != nil {
				if err == io.EOF {
					w.Close()
				} else {
					w.CloseWithError(err)
				}
				break
			}
			result[h.Name] = h
		}
	}()
	if err := c.ExportContainer(docker.ExportContainerOptions{ID: container.ID, OutputStream: w}); err != nil {
		return nil, err
	}
	<-ch
	return result, nil
}

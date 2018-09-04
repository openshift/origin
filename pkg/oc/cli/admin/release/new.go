package release

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	digest "github.com/opencontainers/go-digest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	imageapi "github.com/openshift/api/image/v1"
	imageclient "github.com/openshift/client-go/image/clientset/versioned"
	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	imageappend "github.com/openshift/origin/pkg/oc/cli/image/append"
	"github.com/openshift/origin/pkg/oc/cli/image/extract"
)

func NewNewOptions(streams genericclioptions.IOStreams) *NewOptions {
	return &NewOptions{
		IOStreams:      streams,
		MaxPerRegistry: 4,
	}
}

func NewRelease(f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNewOptions(streams)
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new OpenShift release",
		Long: templates.LongDesc(`
			OpenShift uses long-running active management processes called "operators" to
			keep the cluster running and manage component lifecycle. This command assists
			composing a set of images and operator definitions into a single update payload
			that can be used to update a cluster.

			Operators are expected to host the config they need to be installed to a cluster
			in the '/manifests' directory in their image. This command iterates over a set of
			operator images and extracts those manifests into a single, ordered list of
			Kubernetes objects that can then be iteratively updated on a cluster by the
			cluster version operator when it is time to perform an update.

			Experimental: This command is under active development and may change without notice.
		`),
		Example: templates.Examples(`
			# Create a release from the latest origin images and push to a DockerHub repo
			%[1] new --from-image-stream=origin-v3.11 -n openshift --to-image docker.io/mycompany/myrepo:latest
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flag := cmd.Flags()

	flag.StringSliceVarP(&o.Filenames, "filename", "f", o.Filenames, "A file defining a mapping of input images to use to build the release")
	flag.StringVarP(&o.ImagePattern, "pattern", "p", o.ImagePattern, "The default image pattern.")
	flag.StringVar(&o.Name, "name", o.Name, "The name of the release. Will default to the current time.")
	flag.StringVar(&o.FromImageStream, "from-image-stream", o.FromImageStream, "Look at all tags in the provided image stream and build a release payload from them.")
	flag.StringVar(&o.FromDirectory, "from-dir", o.FromDirectory, "Use this directory as the source for the release payload.")

	flag.StringVarP(&o.Output, "output", "o", o.Output, "Output the mapping definition in this format.")
	flag.StringVar(&o.Directory, "dir", o.Directory, "Directory to write release contents to, will default to a temporary directory.")

	flag.BoolVar(&o.AllowMissingImages, "allow-missing-images", o.AllowMissingImages, "Ignore errors when an operator references a release image that is not included.")

	flag.IntVar(&o.MaxPerRegistry, "max-per-registry", o.MaxPerRegistry, "Number of concurrent images that will be extracted at a time.")

	flag.StringVar(&o.ToFile, "to-file", o.ToFile, "Output the release to a tar file instead of creating an image.")
	flag.StringVar(&o.ToImage, "to-image", o.ToImage, "The location to upload the release image to.")
	flag.StringVar(&o.ToImageBase, "to-image-base", o.ToImageBase, "If specified, the image to add the release layer on top of.")

	return cmd
}

type NewOptions struct {
	genericclioptions.IOStreams

	FromDirectory string
	Directory     string
	Filenames     []string
	Output        string
	Name          string

	FromImageStream string
	Namespace       string

	ToFile string

	ToImage     string
	ToImageBase string

	MaxPerRegistry int

	AllowMissingImages bool

	ImagePattern string
	Mappings     []Mapping

	ImageClient imageclient.Interface
}

func (o *NewOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	overlap := make(map[string]string)
	var mappings []Mapping
	for _, filename := range o.Filenames {
		fileMappings, err := parseFile(filename, overlap)
		if err != nil {
			return err
		}
		mappings = append(mappings, fileMappings...)
	}
	argMappings, err := parseArgs(args, overlap)
	if err != nil {
		return err
	}
	mappings = append(mappings, argMappings...)
	o.Mappings = mappings

	if len(o.FromImageStream) > 0 {
		cfg, err := f.ToRESTConfig()
		if err != nil {
			return err
		}
		client, err := imageclient.NewForConfig(cfg)
		if err != nil {
			return err
		}
		o.ImageClient = client
		if len(o.Namespace) == 0 {
			namespace, _, err := f.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return err
			}
			o.Namespace = namespace
		}
	}
	return nil
}

type imageData struct {
	Ref       imagereference.DockerImageReference
	Config    *docker10.DockerImageConfig
	Digest    digest.Digest
	Directory string
}

var matchPrefix = regexp.MustCompile(`^(\d+[-_])?(.*)$`)

func findStatusTagEvent(tags []imageapi.NamedTagEventList, name string) *imageapi.TagEvent {
	for _, tag := range tags {
		if tag.Tag != name {
			continue
		}
		if len(tag.Items) == 0 {
			return nil
		}
		return &tag.Items[0]
	}
	return nil
}
func findSpecTag(tags []imageapi.TagReference, name string) *imageapi.TagReference {
	for i, tag := range tags {
		if tag.Name != name {
			continue
		}
		return &tags[i]
	}
	return nil
}

func (o *NewOptions) Run() error {
	if len(o.FromImageStream) > 0 && len(o.FromDirectory) > 0 {
		return fmt.Errorf("only one of --from-image-stream and --from-dir may be specified")
	}
	if len(o.FromDirectory) == 0 && len(o.FromImageStream) == 0 {
		if len(o.Mappings) == 0 && len(o.ImagePattern) == 0 {
			return fmt.Errorf("must specify an image pattern and/or mappings")
		}
		if len(o.ImagePattern) > 0 && strings.Count(o.ImagePattern, "*") != 1 {
			return fmt.Errorf("image pattern must have exactly one wildcard character, representing the component name")
		}
	}

	metadata := make(map[string]imageData)
	var ordered []string

	var is *imageapi.ImageStream

	switch {
	case len(o.FromImageStream) > 0:
		inputIS, err := o.ImageClient.ImageV1().ImageStreams(o.Namespace).Get(o.FromImageStream, metav1.GetOptions{})
		if err != nil {
			return err
		}
		is = &imageapi.ImageStream{}
		is.Annotations = map[string]string{
			"release.openshift.io/from-image-stream":         fmt.Sprintf("%s/%s", o.Namespace, o.FromImageStream),
			"release.openshift.io/from-image-stream-version": inputIS.ResourceVersion,
		}
		switch {
		case len(inputIS.Status.PublicDockerImageRepository) > 0:
			for _, tag := range inputIS.Status.Tags {
				if len(tag.Items) == 0 {
					continue
				}
				if len(tag.Items[0].Image) == 0 {
					glog.V(2).Infof("Ignored tag %q because it had no image id", tag.Tag)
					continue
				}
				ref := findSpecTag(inputIS.Spec.Tags, tag.Tag)
				if ref == nil {
					ref = &imageapi.TagReference{Name: tag.Tag}
				} else {
					ref = ref.DeepCopy()
				}
				ref.From = &corev1.ObjectReference{Kind: "DockerImage", Name: inputIS.Status.PublicDockerImageRepository + "@" + tag.Items[0].Image}
				is.Spec.Tags = append(is.Spec.Tags, *ref)
				ordered = append(ordered, tag.Tag)
			}
		default:
			// TODO: add support for internal and referential
			return fmt.Errorf("only image streams with public image repositories can be the source for a release payload")
		}
	case len(o.FromDirectory) > 0:
		fmt.Fprintf(o.ErrOut, "info: Using %s as the input to the release\n", o.FromDirectory)
		files, err := ioutil.ReadDir(o.FromDirectory)
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.IsDir() {
				name := f.Name()
				if matches := matchPrefix.FindStringSubmatch(name); matches != nil {
					name = matches[2]
				}
				metadata[name] = imageData{Directory: filepath.Join(o.FromDirectory, f.Name())}
				ordered = append(ordered, name)
			}
			if f.Name() == "images" {
				data, err := ioutil.ReadFile(filepath.Join(o.FromDirectory, "images"))
				if err != nil {
					return err
				}
				overrideIS := &imageapi.ImageStream{}
				if err := json.Unmarshal(data, overrideIS); err != nil {
					return fmt.Errorf("unable to load image data from release directory")
				}
				if overrideIS.TypeMeta.Kind != "ImageStream" || overrideIS.APIVersion != "image.openshift.io/v1" {
					return fmt.Errorf("could not parse images: invalid kind/apiVersion")
				}
				is = overrideIS
				continue
			}
		}
	default:
		for _, m := range o.Mappings {
			ordered = append(ordered, m.Source)
		}
	}

	if is == nil {
		is = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{},
		}
	}

	is.TypeMeta = metav1.TypeMeta{APIVersion: "image.openshift.io/v1", Kind: "ImageStream"}

	now := time.Now()
	name := o.Name
	if len(name) == 0 {
		name = now.Format("2006-01-02T150405Z")
	}
	is.CreationTimestamp = metav1.Time{Time: now}
	is.Name = name
	if is.Annotations == nil {
		is.Annotations = make(map[string]string)
	}
	if len(o.ImagePattern) > 0 {
		is.Annotations["release.openshift.io/imagePattern"] = o.ImagePattern
	}

	for _, m := range o.Mappings {
		tag := hasTag(is.Spec.Tags, m.Source)
		if tag == nil {
			is.Spec.Tags = append(is.Spec.Tags, imageapi.TagReference{
				Name: m.Source,
			})
			tag = &is.Spec.Tags[len(is.Spec.Tags)-1]
		}
		tag.From = &corev1.ObjectReference{
			Name: m.Destination,
			Kind: "DockerImage",
		}
	}

	if o.Output == "json" {
		sort.Slice(is.Spec.Tags, func(i, j int) bool {
			return is.Spec.Tags[i].Name < is.Spec.Tags[j].Name
		})
		data, err := json.MarshalIndent(is, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "%s\n", string(data))
		return nil
	}

	if len(o.FromDirectory) == 0 {
		if len(is.Spec.Tags) == 0 {
			return fmt.Errorf("no component images defined, unable to build a release payload")
		}

		dir := o.Directory
		if len(dir) == 0 {
			var err error
			dir, err = ioutil.TempDir("", fmt.Sprintf("release-image-%s", name))
			if err != nil {
				return err
			}
			defer func() { os.RemoveAll(dir) }()
			fmt.Fprintf(o.ErrOut, "info: Manifests will be extracted to %s\n", dir)
		}

		opts := extract.NewOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
		opts.OnlyFiles = true
		opts.MaxPerRegistry = o.MaxPerRegistry
		opts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {
			metadata[m.Name] = imageData{
				Directory: m.To,
				Ref:       m.ImageRef,
				Config:    config,
				Digest:    dgst,
			}
		}

		for _, tag := range is.Spec.Tags {
			dstDir := filepath.Join(dir, tag.Name)
			if err := os.MkdirAll(dstDir, 0770); err != nil {
				return err
			}
			src := tag.From.Name
			ref, err := imagereference.Parse(src)
			if err != nil {
				return err
			}
			opts.Mappings = append(opts.Mappings, extract.Mapping{
				Name:     tag.Name,
				ImageRef: ref,

				From: "manifests/",
				To:   dstDir,

				LayerFilter: extract.NewPositionLayerFilter(-1),

				ConditionFn: func(m *extract.Mapping, dgst digest.Digest, imageConfig *docker10.DockerImageConfig) (bool, error) {
					if imageConfig.Config == nil || len(imageConfig.Config.Labels["io.openshift.release.operator"]) == 0 {
						glog.V(2).Infof("Image %s has no io.openshift.release.operator label, skipping", m.ImageRef)
						return false, nil
					}
					fmt.Fprintf(o.Out, "Loading manifests from %s ...\n", src)
					return true, nil
				},
			})
		}
		if err := opts.Run(); err != nil {
			return err
		}
		var filteredNames []string
		for _, s := range ordered {
			if _, ok := metadata[s]; ok {
				filteredNames = append(filteredNames, s)
			}
		}
		ordered = filteredNames
	}

	var operators []string
	pr, pw := io.Pipe()
	go func() {
		var err error
		operators, err = writePayload(pw, now, is, ordered, metadata, o.AllowMissingImages)
		pw.CloseWithError(err)
	}()

	switch {
	case len(o.ToFile) > 0:
		f, err := os.OpenFile(o.ToFile, os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_WRONLY, 0750)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, pr); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	case len(o.ToImage) > 0:
		toRef, err := imagereference.Parse(o.ToImage)
		if err != nil {
			return fmt.Errorf("--to-image was not valid: %v", err)
		}
		if len(toRef.ID) > 0 {
			return fmt.Errorf("--to-image may only point to a repository or tag, not a digest")
		}
		if len(toRef.Tag) == 0 {
			toRef.Tag = o.Name
		}
		options := imageappend.NewAppendImageOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
		options.From = o.ToImageBase
		options.DropHistory = true
		options.ConfigPatch = fmt.Sprintf(`{"Labels":{"io.openshift.release":"%s"}}`, is.Name)
		options.MetaPatch = `{"architecture":"amd64","os":"Linux"}`
		options.LayerStream = pr
		options.To = toRef.Exact()
		if err := options.Run(); err != nil {
			return err
		}
	default:
		if _, err := io.Copy(ioutil.Discard, pr); err != nil {
			return err
		}
	}

	sort.Strings(operators)
	if len(operators) == 0 {
		fmt.Fprintf(o.ErrOut, "warning: No manifest metadata was found in the provided images or directory, no top-level operators will be created.\n")
	} else {
		fmt.Fprintf(o.Out, "Built update image content from %d operators in %d components: %s\n", len(operators), len(metadata), strings.Join(operators, ", "))
	}

	sort.Slice(is.Spec.Tags, func(i, j int) bool {
		return is.Spec.Tags[i].Name < is.Spec.Tags[j].Name
	})

	return nil
}

var hasNumberPrefix = regexp.MustCompile(`^\d+_`)

// writeNestedTarHeader writes a series of nested tar headers, starting with parts[0] and joining each
// successive part, but only if the path does not exist already.
func writeNestedTarHeader(tw *tar.Writer, parts []string, existing map[string]struct{}, hdr tar.Header) error {
	for i := range parts {
		componentDir := path.Join(parts[:i+1]...)
		if _, ok := existing[componentDir]; ok {
			continue
		}
		existing[componentDir] = struct{}{}
		hdr.Name = componentDir
		if err := tw.WriteHeader(&hdr); err != nil {
			return err
		}
	}
	return nil
}

func writePayload(w io.Writer, now time.Time, is *imageapi.ImageStream, ordered []string, metadata map[string]imageData, allowMissingImages bool) ([]string, error) {
	var operators []string
	directories := make(map[string]struct{})
	files := make(map[string]int)

	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	parts := []string{"release-manifests"}

	// ensure the directory exists in the tar bundle
	if err := writeNestedTarHeader(tw, parts, directories, tar.Header{Mode: 0777, ModTime: now, Typeflag: tar.TypeDir}); err != nil {
		return nil, err
	}

	// write image metadata to release-manifests/image-references
	data, err := json.MarshalIndent(is, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: path.Join(append(append([]string{}, parts...), "image-references")...), Size: int64(len(data))}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(data); err != nil {
		return nil, err
	}

	// we iterate over each input directory in order to ensure the output is stable
	for _, name := range ordered {
		data, ok := metadata[name]
		if !ok {
			return nil, fmt.Errorf("missing image data %s", name)
		}

		// process each manifest in the given directory
		contents, err := ioutil.ReadDir(data.Directory)
		if err != nil {
			return nil, err
		}
		if len(contents) == 0 {
			continue
		}

		transform := func(data []byte) ([]byte, error) {
			return data, nil
		}

		if fi := takeFileByName(&contents, "image-references"); fi != nil {
			path := filepath.Join(data.Directory, fi.Name())
			glog.V(2).Infof("Perform image replacement based on inclusion of %s", path)
			transform, err = NewImageMapperFromImageStreamFile(path, is, allowMissingImages)
			if err != nil {
				return nil, fmt.Errorf("operator %q failed to map images: %s", name, err)
			}
		}

		for _, fi := range contents {
			if fi.IsDir() {
				continue
			}
			filename := fi.Name()

			// give every file a unique name and ordering
			if !hasNumberPrefix.MatchString(filename) {
				filename = fmt.Sprintf("99_%s_%s", name, filename)
			}
			if count, ok := files[filename]; ok {
				count++
				ext := path.Ext(path.Base(filename))
				filename = fmt.Sprintf("%s_%d%s", strings.TrimSuffix(filename, ext), count, ext)
				files[filename] = count
			} else {
				files[filename] = 1
			}
			src := filepath.Join(data.Directory, fi.Name())
			dst := path.Join(append(append([]string{}, parts...), filename)...)
			glog.V(4).Infof("Copying %s to %s", src, dst)

			data, err := ioutil.ReadFile(src)
			if err != nil {
				return nil, err
			}
			modified, err := transform(data)
			if err != nil {
				return nil, err
			}
			if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: dst, Size: int64(len(modified))}); err != nil {
				return nil, err
			}
			if _, err := tw.Write(modified); err != nil {
				return nil, err
			}
		}
		operators = append(operators, name)
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return operators, nil
}

func hasTag(tags []imageapi.TagReference, tag string) *imageapi.TagReference {
	for i := range tags {
		if tag == tags[i].Name {
			return &tags[i]
		}
	}
	return nil
}

func pruneEmptyDirectories(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		names, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		if len(names) > 0 {
			return nil
		}
		glog.V(4).Infof("Component %s does not have any manifests", path)
		return os.Remove(path)
	})
}

type Mapping struct {
	Source      string
	Destination string
}

func parseArgs(args []string, overlap map[string]string) ([]Mapping, error) {
	var mappings []Mapping
	for _, s := range args {
		parts := strings.SplitN(s, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("all arguments must be valid SRC=DST mappings")
		}
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return nil, fmt.Errorf("all arguments must be valid SRC=DST mappings")
		}
		src := parts[0]
		dst := parts[1]
		if _, ok := overlap[src]; ok {
			return nil, fmt.Errorf("each source tag may only be specified once: %s", dst)
		}
		overlap[dst] = src

		mappings = append(mappings, Mapping{Source: src, Destination: dst})
	}
	return mappings, nil
}

func parseFile(filename string, overlap map[string]string) ([]Mapping, error) {
	var fileMappings []Mapping
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	lineNumber := 0
	for s.Scan() {
		line := s.Text()
		lineNumber++

		// remove comments and whitespace
		if i := strings.Index(line, "#"); i != -1 {
			line = line[0:i]
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		args := strings.Split(line, " ")
		mappings, err := parseArgs(args, overlap)
		if err != nil {
			return nil, fmt.Errorf("file %s, line %d: %v", filename, lineNumber, err)
		}
		fileMappings = append(fileMappings, mappings...)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return fileMappings, nil
}

func takeFileByName(files *[]os.FileInfo, name string) os.FileInfo {
	for i, fi := range *files {
		if fi.IsDir() || fi.Name() != name {
			continue
		}
		*files = append((*files)[:i], (*files)[i+1:]...)
		return fi
	}
	return nil
}

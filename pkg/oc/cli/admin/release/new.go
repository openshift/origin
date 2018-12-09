package release

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/docker/docker/pkg/archive"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
		// TODO: only cluster-version-operator and maybe CLI should be in this list,
		//   the others should always be referenced by the cluster-bootstrap or
		//   another operator.
		AlwaysInclude:  []string{"cluster-version-operator", "cli", "installer"},
		ToImageBaseTag: "cluster-version-operator",
	}
}

func NewRelease(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNewOptions(streams)
	cmd := &cobra.Command{
		Use:   "new [SRC=DST ...]",
		Short: "Create a new OpenShift release",
		Long: templates.LongDesc(`
			Build a new OpenShift release image that will update a cluster

			OpenShift uses long-running active management processes called "operators" to
			keep the cluster running and manage component lifecycle. This command
			composes a set of images and operator definitions into a single update payload
			that can be used to update a cluster.

			Operators are expected to host the config they need to be installed to a cluster
			in the '/manifests' directory in their image. This command iterates over a set of
			operator images and extracts those manifests into a single, ordered list of
			Kubernetes objects that can then be iteratively updated on a cluster by the
			cluster version operator when it is time to perform an update. Manifest files are
			renamed to '99_<image_name>_<filename>' by default, and an operator author that
			needs to provide a global-ordered file (before or after other operators) should
			prepend '0000_' to their filename, which instructs the release builder to not
			assign a component prefix. Only images with the label
			'release.openshift.io/operator=true' are considered to be included.

			Mappings specified via SRC=DST positional arguments allows overriding particular
			operators with a specific image.  For example:

			cluster-version-operator=registry.example.com/openshift/cluster-version-operator:test-123

			will override the default cluster-version-operator image with one pulled from
			registry.example.com.

			Experimental: This command is under active development and may change without notice.
		`),
		Example: templates.Examples(fmt.Sprintf(`
			# Create a release from the latest origin images and push to a DockerHub repo
			%[1]s new --from-image-stream=origin-v4.0 -n openshift --to-image docker.io/mycompany/myrepo:latest
		`, parentName)),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()

	// image inputs
	flags.StringSliceVarP(&o.Filenames, "filename", "f", o.Filenames, "A file defining a mapping of input images to use to build the release")
	flags.StringVar(&o.FromImageStream, "from-image-stream", o.FromImageStream, "Look at all tags in the provided image stream and build a release payload from them.")
	flags.StringVar(&o.FromDirectory, "from-dir", o.FromDirectory, "Use this directory as the source for the release payload.")
	flags.StringVar(&o.FromReleaseImage, "from-release", o.FromReleaseImage, "Use an existing release image as input.")

	// properties of the release
	flags.StringVar(&o.Name, "name", o.Name, "The name of the release. Will default to the current time.")
	flags.StringSliceVar(&o.PreviousVersions, "previous", o.PreviousVersions, "A list of semantic versions that should preceed this version in the release manifest.")
	flags.StringVar(&o.ReleaseMetadata, "metadata", o.ReleaseMetadata, "A JSON object to attach as the metadata for the release manifest.")
	flags.BoolVar(&o.ForceManifest, "release-manifest", o.ForceManifest, "If true, a release manifest will be created using --name as the semantic version.")

	// validation
	flags.BoolVar(&o.AllowMissingImages, "allow-missing-images", o.AllowMissingImages, "Ignore errors when an operator references a release image that is not included.")
	flags.BoolVar(&o.SkipManifestCheck, "skip-manifest-check", o.SkipManifestCheck, "Ignore errors when an operator includes a yaml/yml/json file that is not parseable.")

	flags.StringSliceVar(&o.Exclude, "exclude", o.Exclude, "A list of image names or tags to exclude. It is applied after all inputs. Comma separated or individual arguments.")
	flags.StringSliceVar(&o.AlwaysInclude, "include", o.AlwaysInclude, "A list of image tags that should not be pruned. Excluding a tag takes precedence. Comma separated or individual arguments.")

	// destination
	flags.BoolVar(&o.DryRun, "dry-run", o.DryRun, "Skips changes to external registries via mirroring or pushing images.")
	flags.StringVar(&o.Mirror, "mirror", o.Mirror, "Mirror the contents of the release to this repository.")
	flags.StringVar(&o.ToDir, "to-dir", o.ToDir, "Output the release manifests to a directory instead of creating an image.")
	flags.StringVar(&o.ToFile, "to-file", o.ToFile, "Output the release to a tar file instead of creating an image.")
	flags.StringVar(&o.ToImage, "to-image", o.ToImage, "The location to upload the release image to.")
	flags.StringVar(&o.ToImageBase, "to-image-base", o.ToImageBase, "If specified, the image to add the release layer on top of.")
	flags.StringVar(&o.ToImageBaseTag, "to-image-base-tag", o.ToImageBaseTag, "If specified, the image tag in the input to add the release layer on top of. Defaults to cluster-version-operator.")

	// misc
	flags.StringVarP(&o.Output, "output", "o", o.Output, "Output the mapping definition in this format.")
	flags.StringVar(&o.Directory, "dir", o.Directory, "Directory to write release contents to, will default to a temporary directory.")
	flags.IntVar(&o.MaxPerRegistry, "max-per-registry", o.MaxPerRegistry, "Number of concurrent images that will be extracted at a time.")

	return cmd
}

type NewOptions struct {
	genericclioptions.IOStreams

	FromDirectory string
	Directory     string
	Filenames     []string
	Output        string
	Name          string

	FromReleaseImage string

	FromImageStream string
	Namespace       string

	Exclude       []string
	AlwaysInclude []string

	ForceManifest    bool
	ReleaseMetadata  string
	PreviousVersions []string

	DryRun bool

	ToFile         string
	ToDir          string
	ToImage        string
	ToImageBase    string
	ToImageBaseTag string

	Mirror string

	MaxPerRegistry int

	AllowMissingImages bool
	SkipManifestCheck  bool

	Mappings []Mapping

	ImageClient imageclient.Interface

	cleanupFns []func()
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

type CincinnatiMetadata struct {
	Kind string `json:"kind"`

	Version  string   `json:"version"`
	Previous []string `json:"previous"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (o *NewOptions) cleanup() {
	for _, fn := range o.cleanupFns {
		fn()
	}
	o.cleanupFns = nil
}

func (o *NewOptions) Run() error {
	defer o.cleanup()

	if len(o.FromImageStream) > 0 && len(o.FromDirectory) > 0 {
		return fmt.Errorf("only one of --from-image-stream and --from-dir may be specified")
	}
	if len(o.FromDirectory) == 0 && len(o.FromImageStream) == 0 && len(o.FromReleaseImage) == 0 {
		if len(o.Mappings) == 0 {
			return fmt.Errorf("must specify image mappings")
		}
	}

	now := time.Now().UTC()
	name := o.Name
	if len(name) == 0 {
		name = "0.0.1-" + now.Format("2006-01-02-150405")
	}

	var cm *CincinnatiMetadata
	// TODO: remove this once all code creates semantic versions
	if _, err := semver.Parse(name); err == nil {
		o.ForceManifest = true
	}
	if len(o.PreviousVersions) > 0 || len(o.ReleaseMetadata) > 0 || o.ForceManifest {
		cm = &CincinnatiMetadata{Kind: "cincinnati-metadata-v0"}
		semverName, err := semver.Parse(name)
		if err != nil {
			return fmt.Errorf("when release metadata is added, the --name must be a semantic version")
		}
		cm.Version = semverName.String()
	}
	if len(o.ReleaseMetadata) > 0 {
		if err := json.Unmarshal([]byte(o.ReleaseMetadata), &cm.Metadata); err != nil {
			return fmt.Errorf("invalid --metadata: %v", err)
		}
	}
	if cm != nil {
		for _, previous := range o.PreviousVersions {
			if len(previous) == 0 {
				continue
			}
			v, err := semver.Parse(previous)
			if err != nil {
				return fmt.Errorf("%q is not a valid semantic version: %v", previous, err)
			}
			cm.Previous = append(cm.Previous, v.String())
		}
		sort.Strings(cm.Previous)
		if cm.Previous == nil {
			cm.Previous = []string{}
		}
	}
	glog.V(4).Infof("Release metadata:\n%s", toJSONString(cm))

	exclude := sets.NewString()
	for _, s := range o.Exclude {
		exclude.Insert(s)
	}

	metadata := make(map[string]imageData)
	var ordered []string
	var payload *Payload
	var is *imageapi.ImageStream

	switch {
	case len(o.FromReleaseImage) > 0:
		dir := o.Directory
		if len(dir) == 0 {
			tempDir, err := ioutil.TempDir("", "release-manifests")
			if err != nil {
				return err
			}
			defer func() {
				if err := os.RemoveAll(tempDir); err != nil {
					glog.Warningf("Unable to remove temporary payload directory %s: %v", dir, err)
				}
			}()
			dir = tempDir
		}

		var baseDigest string
		buf := &bytes.Buffer{}
		extractOpts := NewExtractOptions(genericclioptions.IOStreams{Out: buf, ErrOut: o.ErrOut})
		extractOpts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {
			if config.Config != nil {
				baseDigest = config.Config.Labels["io.openshift.release.base-image-digest"]
				glog.V(4).Infof("Release image was built on top of %s", baseDigest)
			}
		}
		extractOpts.From = o.FromReleaseImage
		extractOpts.Directory = dir
		if err := extractOpts.Run(); err != nil {
			return fmt.Errorf("unable to retrieve release image info: %v", err)
		}

		payload = NewPayload(dir)

		inputIS, err := payload.References()
		if err != nil {
			return fmt.Errorf("unable to load payload from release contents: %v", err)
		}
		is = inputIS.DeepCopy()
		if is.Annotations == nil {
			is.Annotations = map[string]string{}
		}
		is.Annotations["release.openshift.io/from-release"] = o.FromReleaseImage

		// default the base image to a matching release payload digest or error
		if len(o.ToImageBase) == 0 && len(baseDigest) > 0 {
			for _, tag := range is.Spec.Tags {
				if tag.From == nil || tag.From.Kind != "DockerImage" {
					continue
				}
				ref, err := imagereference.Parse(tag.From.Name)
				if err != nil {
					return fmt.Errorf("release image contains unparseable reference for %q: %v", tag.Name, err)
				}
				if ref.ID == baseDigest {
					o.ToImageBase = tag.From.Name
					break
				}
			}
			if len(o.ToImageBase) == 0 {
				return fmt.Errorf("unable to find an image within the release that matches the base image manifest %q, please specify --to-image-base", baseDigest)
			}
		}

		if len(o.Name) == 0 {
			o.Name = is.Name
		}

	case len(o.FromImageStream) > 0:
		is = &imageapi.ImageStream{}
		is.Annotations = map[string]string{}
		if len(o.FromImageStream) > 0 && len(o.Namespace) > 0 {
			is.Annotations["release.openshift.io/from-image-stream"] = fmt.Sprintf("%s/%s", o.Namespace, o.FromImageStream)
		}

		inputIS, err := o.ImageClient.ImageV1().ImageStreams(o.Namespace).Get(o.FromImageStream, metav1.GetOptions{})
		if err != nil {
			return err
		}

		switch {
		case len(inputIS.Status.PublicDockerImageRepository) > 0:
			for _, tag := range inputIS.Status.Tags {
				if exclude.Has(tag.Tag) {
					glog.V(2).Infof("Excluded status tag %s", tag.Tag)
					continue
				}
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
			fmt.Fprintf(o.ErrOut, "info: Found %d images in image stream\n", len(inputIS.Status.Tags))
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
				if exclude.Has(name) {
					glog.V(2).Infof("Excluded directory %#v", f)
					continue
				}
				metadata[name] = imageData{Directory: filepath.Join(o.FromDirectory, f.Name())}
				ordered = append(ordered, name)
			}
			if f.Name() == "image-references" {
				data, err := ioutil.ReadFile(filepath.Join(o.FromDirectory, "image-references"))
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
		fmt.Fprintf(o.ErrOut, "info: Found %d operator manifest directories on disk\n", len(ordered))

	default:
		for _, m := range o.Mappings {
			if exclude.Has(m.Source) {
				glog.V(2).Infof("Excluded mapping %s", m.Source)
				continue
			}
			ordered = append(ordered, m.Source)
		}
	}

	if is == nil {
		is = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{},
		}
	}

	is.TypeMeta = metav1.TypeMeta{APIVersion: "image.openshift.io/v1", Kind: "ImageStream"}
	is.CreationTimestamp = metav1.Time{Time: now}
	is.Name = name
	if is.Annotations == nil {
		is.Annotations = make(map[string]string)
	}

	// update any custom mappings and then sort the spec tags
	for _, m := range o.Mappings {
		if exclude.Has(m.Source) {
			glog.V(2).Infof("Excluded mapping %s", m.Source)
			continue
		}
		tag := hasTag(is.Spec.Tags, m.Source)
		if tag == nil {
			is.Spec.Tags = append(is.Spec.Tags, imageapi.TagReference{
				Name: m.Source,
			})
			tag = &is.Spec.Tags[len(is.Spec.Tags)-1]
		} else {
			// when we override the spec, we have to reset any annotations
			tag.Annotations = nil
		}
		tag.From = &corev1.ObjectReference{
			Name: m.Destination,
			Kind: "DockerImage",
		}
	}
	sort.Slice(is.Spec.Tags, func(i, j int) bool {
		return is.Spec.Tags[i].Name < is.Spec.Tags[j].Name
	})

	if o.Output == "json" {
		data, err := json.MarshalIndent(is, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "%s\n", string(data))
		return nil
	}

	if len(o.FromDirectory) == 0 && payload == nil {
		if err := o.extractManifests(is, name, metadata); err != nil {
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

	if len(o.Mirror) > 0 {
		if err := o.mirrorImages(is, payload); err != nil {
			return err
		}

	} else if payload != nil && len(o.Mappings) > 0 {
		glog.V(4).Infof("Rewriting payload for the input mappings")
		targetFn, err := ComponentReferencesForImageStream(is)
		if err != nil {
			return err
		}
		if err := payload.Rewrite(true, targetFn); err != nil {
			return fmt.Errorf("failed to update contents for input mappings: %v", err)
		}
	}

	var verifiers []PayloadVerifier
	if !o.SkipManifestCheck {
		verifiers = append(verifiers, func(filename string, data []byte) error {
			for _, suffix := range []string{".json", ".yml", ".yaml"} {
				if !strings.HasSuffix(filename, suffix) {
					continue
				}
				var obj interface{}
				if err := yaml.Unmarshal(data, &obj); err != nil {
					// strip the slightly verbose prefix for the error message
					msg := err.Error()
					for _, s := range []string{"error converting YAML to JSON: ", "error unmarshaling JSON: ", "yaml: "} {
						msg = strings.TrimPrefix(msg, s)
					}
					return fmt.Errorf("%s: invalid YAML/JSON: %s", filename, msg)
				}
				break
			}
			return nil
		})
	}

	if payload == nil {
		if err := pruneUnreferencedImageStreams(o.ErrOut, is, metadata, o.AlwaysInclude); err != nil {
			return err
		}
	}

	var operators []string
	pr, pw := io.Pipe()
	go func() {
		var err error
		if payload != nil {
			err = copyPayload(pw, now, is, cm, payload.Path(), verifiers)
		} else {
			operators, err = writePayload(pw, now, is, cm, ordered, metadata, o.AllowMissingImages, verifiers)
		}
		pw.CloseWithError(err)
	}()

	br := bufio.NewReaderSize(pr, 500*1024)
	_, err := br.Peek(br.Size())
	if err != nil && err != io.EOF {
		return fmt.Errorf("unable to create a release: %v", err)
	}

	if err := o.write(br, is, now); err != nil {
		return err
	}

	sort.Strings(operators)
	switch {
	case operators == nil:
	case len(operators) == 0:
		fmt.Fprintf(o.ErrOut, "warning: No operator metadata was found, no operators will be part of the release.\n")
	default:
		fmt.Fprintf(o.Out, "Built release image from %d operators\n", len(operators))
	}

	return nil
}

func (o *NewOptions) extractManifests(is *imageapi.ImageStream, name string, metadata map[string]imageData) error {
	if len(is.Spec.Tags) == 0 {
		return fmt.Errorf("no component images defined, unable to build a release payload")
	}

	glog.V(4).Infof("Extracting manifests for release from input images")

	dir := o.Directory
	if len(dir) == 0 {
		var err error
		dir, err = ioutil.TempDir("", fmt.Sprintf("release-image-%s", name))
		if err != nil {
			return err
		}
		o.cleanupFns = append(o.cleanupFns, func() { os.RemoveAll(dir) })
		fmt.Fprintf(o.ErrOut, "info: Manifests will be extracted to %s\n", dir)
	}

	if len(is.Spec.Tags) > 0 {
		if err := os.MkdirAll(dir, 0770); err != nil {
			return err
		}
		data, err := json.MarshalIndent(is, "", "  ")
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(dir, "image-references"), data, 0640); err != nil {
			return err
		}
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

	for i := range is.Spec.Tags {
		tag := &is.Spec.Tags[i]
		dstDir := filepath.Join(dir, tag.Name)
		if tag.From.Kind != "DockerImage" {
			continue
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
				var labels map[string]string
				if imageConfig.Config != nil {
					labels = imageConfig.Config.Labels
				}

				if len(labels["vcs-ref"]) > 0 {
					if tag.Annotations == nil {
						tag.Annotations = make(map[string]string)
					}
					tag.Annotations["io.openshift.build.commit.id"] = labels["vcs-ref"]
					tag.Annotations["io.openshift.build.commit.ref"] = labels["io.openshift.build.commit.ref"]
					tag.Annotations["io.openshift.build.source-location"] = labels["vcs-url"]
				}

				if len(labels["io.openshift.release.operator"]) == 0 {
					glog.V(2).Infof("Image %s has no io.openshift.release.operator label, skipping", m.ImageRef)
					return false, nil
				}
				if err := os.MkdirAll(dstDir, 0770); err != nil {
					return false, err
				}
				fmt.Fprintf(o.Out, "Loading manifests from %s: %s ...\n", tag.Name, m.ImageRef.ID)
				return true, nil
			},
		})
	}
	return opts.Run()
}

func (o *NewOptions) mirrorImages(is *imageapi.ImageStream, payload *Payload) error {
	glog.V(4).Infof("Mirroring release contents to %s", o.Mirror)
	copied := is.DeepCopy()
	opts := NewMirrorOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.DryRun = o.DryRun
	opts.ImageStream = copied
	opts.To = o.Mirror
	opts.SkipRelease = true

	if err := opts.Run(); err != nil {
		return err
	}

	if payload == nil {
		return nil
	}

	glog.V(4).Infof("Rewriting payload to point to mirror")
	targetFn, err := ComponentReferencesForImageStream(copied)
	if err != nil {
		return err
	}
	if err := payload.Rewrite(false, targetFn); err != nil {
		return fmt.Errorf("failed to update contents after mirroring: %v", err)
	}
	updated, err := payload.References()
	if err != nil {
		return fmt.Errorf("unable to recalculate image references: %v", err)
	}
	*is = *updated
	if glog.V(4) {
		data, _ := json.MarshalIndent(is, "", "  ")
		glog.Infof("Image references updated to:\n%s", string(data))
	}
	return nil
}

func (o *NewOptions) write(r io.Reader, is *imageapi.ImageStream, now time.Time) error {
	switch {
	case len(o.ToDir) > 0:
		glog.V(4).Infof("Writing release contents to directory %s", o.ToDir)
		if err := os.MkdirAll(o.ToDir, 0755); err != nil {
			return err
		}
		r, err := archive.DecompressStream(r)
		if err != nil {
			return err
		}
		tr := tar.NewReader(r)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if !strings.HasPrefix(hdr.Name, "release-manifests/") || hdr.Typeflag&tar.TypeReg != tar.TypeReg {
				continue
			}
			name := strings.TrimPrefix(hdr.Name, "release-manifests/")
			if strings.Count(name, "/") > 0 || name == "." || name == ".." || len(name) == 0 {
				continue
			}
			f, err := os.OpenFile(filepath.Join(o.ToDir, name), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	case len(o.ToFile) > 0:
		glog.V(4).Infof("Writing release contents to file %s", o.ToFile)
		var w io.WriteCloser
		if o.ToFile == "-" {
			w = nopCloser{o.Out}
		} else {
			f, err := os.OpenFile(o.ToFile, os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_WRONLY, 0750)
			if err != nil {
				return err
			}
			w = f
		}
		if _, err := io.Copy(w, r); err != nil {
			w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	case len(o.ToImage) > 0:
		glog.V(4).Infof("Writing release contents to image %s", o.ToImage)
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
		toImageBase := o.ToImageBase
		if len(toImageBase) == 0 && len(o.ToImageBaseTag) > 0 {
			for _, tag := range is.Spec.Tags {
				if tag.From != nil && tag.From.Kind == "DockerImage" && tag.Name == o.ToImageBaseTag {
					toImageBase = tag.From.Name
				}
			}
			if len(toImageBase) == 0 {
				return fmt.Errorf("--to-image-base-tag did not point to a tag in the input")
			}
		}

		options := imageappend.NewAppendImageOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
		options.DryRun = o.DryRun
		options.From = toImageBase
		options.ConfigurationCallback = func(dgst digest.Digest, config *docker10.DockerImageConfig) error {
			// reset any base image info
			if len(config.OS) == 0 {
				config.OS = "linux"
			}
			if len(config.Architecture) == 0 {
				config.Architecture = "amd64"
			}
			config.Container = ""
			config.Parent = ""
			config.Created = now
			config.ContainerConfig = docker10.DockerConfig{}
			config.Config.Labels = make(map[string]string)

			// explicitly set release info
			config.Config.Labels["io.openshift.release"] = is.Name
			config.History = []docker10.DockerConfigHistory{
				{Comment: "Release image for OpenShift", Created: now},
			}
			if len(dgst) > 0 {
				config.Config.Labels["io.openshift.release.base-image-digest"] = dgst.String()
			}
			return nil
		}

		options.LayerStream = r
		options.To = toRef.Exact()
		if err := options.Run(); err != nil {
			return err
		}
	default:
		fmt.Fprintf(o.ErrOut, "info: Extracting operator contents to disk without building a release artifact\n")
		if _, err := io.Copy(ioutil.Discard, r); err != nil {
			return err
		}
	}
	return nil
}

func toJSONString(obj interface{}) string {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return string(data)
}

type nopCloser struct {
	io.Writer
}

func (_ nopCloser) Close() error { return nil }

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

func writePayload(w io.Writer, now time.Time, is *imageapi.ImageStream, cm *CincinnatiMetadata, ordered []string, metadata map[string]imageData, allowMissingImages bool, verifiers []PayloadVerifier) ([]string, error) {
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

	// write cincinnati metadata to release-manifests/release-metadata
	if cm != nil {
		data, err := json.MarshalIndent(cm, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: path.Join(append(append([]string{}, parts...), "release-metadata")...), Size: int64(len(data))}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}

	// we iterate over each input directory in order to ensure the output is stable
	for _, name := range ordered {
		image, ok := metadata[name]
		if !ok {
			return nil, fmt.Errorf("missing image data %s", name)
		}

		// process each manifest in the given directory
		contents, err := ioutil.ReadDir(image.Directory)
		if err != nil {
			return nil, err
		}
		if len(contents) == 0 {
			continue
		}

		transform := NopManifestMapper

		if fi := takeFileByName(&contents, "image-references"); fi != nil {
			path := filepath.Join(image.Directory, fi.Name())
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

			// components that don't declare that they need to be part of the global order
			// get put in a scoped bucket at the end. Only a few components should need to
			// be in the global order.
			if !strings.HasPrefix(filename, "0000_") {
				filename = fmt.Sprintf("0000_70_%s_%s", name, filename)
			}
			if count, ok := files[filename]; ok {
				count++
				ext := path.Ext(path.Base(filename))
				filename = fmt.Sprintf("%s_%d%s", strings.TrimSuffix(filename, ext), count, ext)
				files[filename] = count
			} else {
				files[filename] = 1
			}
			src := filepath.Join(image.Directory, fi.Name())
			dst := path.Join(append(append([]string{}, parts...), filename)...)
			glog.V(4).Infof("Copying %s to %s", src, dst)

			data, err := ioutil.ReadFile(src)
			if err != nil {
				return nil, err
			}

			for _, fn := range verifiers {
				if err := fn(filepath.Join(filepath.Base(image.Directory), fi.Name()), data); err != nil {
					return nil, err
				}
			}

			modified, err := transform(data)
			if err != nil {
				return nil, err
			}
			if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: dst, Size: int64(len(modified))}); err != nil {
				return nil, err
			}
			glog.V(6).Infof("Writing payload to %s\n%s", dst, string(modified))
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

func copyPayload(w io.Writer, now time.Time, is *imageapi.ImageStream, cm *CincinnatiMetadata, directory string, verifiers []PayloadVerifier) error {
	directories := make(map[string]struct{})

	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	parts := []string{"release-manifests"}

	// ensure the directory exists in the tar bundle
	if err := writeNestedTarHeader(tw, parts, directories, tar.Header{Mode: 0777, ModTime: now, Typeflag: tar.TypeDir}); err != nil {
		return err
	}

	// copy each manifest in the given directory
	contents, err := ioutil.ReadDir(directory)
	if err != nil {
		return err
	}

	// write image metadata to release-manifests/image-references
	takeFileByName(&contents, "image-references")
	data, err := json.MarshalIndent(is, "", "  ")
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: path.Join(append(append([]string{}, parts...), "image-references")...), Size: int64(len(data))}); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return err
	}

	// write cincinnati if passed to us
	takeFileByName(&contents, "release-metadata")
	if cm != nil {
		data, err := json.MarshalIndent(cm, "", "  ")
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: path.Join(append(append([]string{}, parts...), "release-metadata")...), Size: int64(len(data))}); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}

	for _, fi := range contents {
		if fi.IsDir() {
			continue
		}
		filename := fi.Name()
		src := filepath.Join(directory, filename)
		dst := path.Join(append(append([]string{}, parts...), filename)...)
		glog.V(4).Infof("Copying %s to %s", src, dst)

		data, err := ioutil.ReadFile(src)
		if err != nil {
			return err
		}

		for _, fn := range verifiers {
			if err := fn(filename, data); err != nil {
				return err
			}
		}

		if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: now, Typeflag: tar.TypeReg, Name: dst, Size: int64(len(data))}); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return nil
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

type PayloadVerifier func(filename string, data []byte) error

func pruneUnreferencedImageStreams(out io.Writer, is *imageapi.ImageStream, metadata map[string]imageData, include []string) error {
	referenced := make(map[string]struct{})
	for _, v := range metadata {
		is, err := parseImageStream(filepath.Join(v.Directory, "image-references"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		for _, tag := range is.Spec.Tags {
			referenced[tag.Name] = struct{}{}
		}
	}
	for _, name := range include {
		referenced[name] = struct{}{}
	}
	var updated []imageapi.TagReference
	for _, tag := range is.Spec.Tags {
		_, ok := referenced[tag.Name]
		if !ok {
			glog.V(3).Infof("Excluding tag %s which is not referenced by an operator", tag.Name)
			continue
		}
		updated = append(updated, tag)
	}
	if len(updated) != len(is.Spec.Tags) {
		fmt.Fprintf(out, "info: Included %d referenced images into the payload\n", len(updated))
		is.Spec.Tags = updated
	}
	return nil
}

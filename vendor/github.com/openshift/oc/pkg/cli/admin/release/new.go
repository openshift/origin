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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver"
	"github.com/docker/docker/pkg/archive"
	"github.com/ghodss/yaml"
	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"
	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/pkg/version"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/api/image/docker10"
	imageapi "github.com/openshift/api/image/v1"
	imageclient "github.com/openshift/client-go/image/clientset/versioned"
	"github.com/openshift/library-go/pkg/image/dockerv1client"
	imagereference "github.com/openshift/library-go/pkg/image/reference"
	imageappend "github.com/openshift/oc/pkg/cli/image/append"
	"github.com/openshift/oc/pkg/cli/image/extract"
	imagemanifest "github.com/openshift/oc/pkg/cli/image/manifest"
)

func NewNewOptions(streams genericclioptions.IOStreams) *NewOptions {
	return &NewOptions{
		IOStreams:       streams,
		ParallelOptions: imagemanifest.ParallelOptions{MaxPerRegistry: 4},
		// TODO: only cluster-version-operator and maybe CLI should be in this list,
		//   the others should always be referenced by the cluster-bootstrap or
		//   another operator.
		AlwaysInclude:  []string{"cluster-version-operator", "cli", "installer"},
		ToImageBaseTag: "cluster-version-operator",
		// We strongly control the set of allowed component versions to prevent confusion
		// about what component versions may be used for. Changing this list requires
		// approval from the release architects.
		AllowedComponents: []string{"kubernetes"},
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
			composes a set of images with operator definitions into a single update payload
			that can be used to install or update a cluster.

			Operators are expected to host the config they need to be installed to a cluster
			in the '/manifests' directory in their image. This command iterates over a set of
			operator images and extracts those manifests into a single, ordered list of
			Kubernetes objects that can then be iteratively updated on a cluster by the
			cluster version operator when it is time to perform an update. Manifest files are
			renamed to '0000_70_<image_name>_<filename>' by default, and an operator author that
			needs to provide a global-ordered file (before or after other operators) should
			prepend '0000_NN_<component>_' to their filename, which instructs the release builder
			to not assign a component prefix. Only images in the input that have the image label
			'io.openshift.release.operator=true' will have manifests loaded.

			If an image is in the input but is not referenced by an operator's image-references
			file, the image will not be included in the final release image unless
			--include=NAME is provided.

			Mappings specified via SRC=DST positional arguments allows overriding particular
			operators with a specific image.  For example:

			cluster-version-operator=registry.example.com/openshift/cluster-version-operator:test-123

			will override the default cluster-version-operator image with one pulled from
			registry.example.com.
		`),
		Example: templates.Examples(fmt.Sprintf(`
			# Create a release from the latest origin images and push to a DockerHub repo
			%[1]s new --from-image-stream=4.1 -n origin --to-image docker.io/mycompany/myrepo:latest

			# Create a new release with updated metadata from a previous release
			%[1]s new --from-release registry.svc.ci.openshift.org/origin/release:v4.1 --name 4.1.1 \
				--previous 4.1.0 --metadata ... --to-image docker.io/mycompany/myrepo:latest

			# Create a new release and override a single image
			%[1]s new --from-release registry.svc.ci.openshift.org/origin/release:v4.1 \
				cli=docker.io/mycompany/cli:latest --to-image docker.io/mycompany/myrepo:latest

			# Run a verification pass to ensure the release can be reproduced
			%[1]s new --from-release registry.svc.ci.openshift.org/origin/release:v4.1
				`, parentName)),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	o.SecurityOptions.Bind(flags)
	o.ParallelOptions.Bind(flags)

	// image inputs
	flags.StringSliceVar(&o.MappingFilenames, "mapping-file", o.MappingFilenames, "A file defining a mapping of input images to use to build the release")
	flags.StringVar(&o.FromImageStream, "from-image-stream", o.FromImageStream, "Look at all tags in the provided image stream and build a release payload from them.")
	flags.StringVarP(&o.FromImageStreamFile, "from-image-stream-file", "f", o.FromImageStreamFile, "Take the provided image stream on disk and build a release payload from it.")
	flags.StringVar(&o.FromDirectory, "from-dir", o.FromDirectory, "Use this directory as the source for the release payload.")
	flags.StringVar(&o.FromReleaseImage, "from-release", o.FromReleaseImage, "Use an existing release image as input.")
	flags.StringVar(&o.ReferenceMode, "reference-mode", o.ReferenceMode, "By default, the image reference from an image stream points to the public registry for the stream and the image digest. Pass 'source' to build references to the originating image.")
	flags.StringVar(&o.ExtraComponentVersions, "component-versions", o.ExtraComponentVersions, "Supply additional version strings to the release in key=value[,key=value] form.")

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
	flags.StringVar(&o.ToSignature, "to-signature", o.ToSignature, "If specified, output a message that can be signed that describes this release. Requires --to-image.")

	// misc
	flags.StringVarP(&o.Output, "output", "o", o.Output, "Output the mapping definition in this format.")
	flags.StringVar(&o.Directory, "dir", o.Directory, "Directory to write release contents to, will default to a temporary directory.")

	return cmd
}

type NewOptions struct {
	genericclioptions.IOStreams

	SecurityOptions imagemanifest.SecurityOptions
	ParallelOptions imagemanifest.ParallelOptions

	FromDirectory    string
	Directory        string
	MappingFilenames []string
	Output           string
	Name             string

	FromReleaseImage string

	FromImageStream     string
	FromImageStreamFile string
	Namespace           string
	ReferenceMode       string

	ExtraComponentVersions string
	AllowedComponents      []string

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
	ToSignature    string

	Mirror string

	AllowMissingImages bool
	SkipManifestCheck  bool

	Mappings []Mapping

	ImageClient imageclient.Interface

	VerifyOutputFn func(dgst digest.Digest) error

	cleanupFns []func()
}

func (o *NewOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	overlap := make(map[string]string)
	var mappings []Mapping
	for _, filename := range o.MappingFilenames {
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

func (o *NewOptions) Validate() error {
	sources := 0
	if len(o.FromImageStream) > 0 {
		sources++
	}
	if len(o.FromImageStreamFile) > 0 {
		sources++
	}
	if len(o.FromReleaseImage) > 0 {
		sources++
	}
	if len(o.FromDirectory) > 0 {
		sources++
	}
	if sources > 1 {
		return fmt.Errorf("only one of --from-image-stream, --from-image-stream-file, --from-release, or --from-dir may be specified")
	}
	if sources == 0 {
		if len(o.Mappings) == 0 {
			return fmt.Errorf("must specify image mappings when no other source is defined")
		}
	}
	if len(o.ToSignature) > 0 && len(o.ToImage) == 0 {
		return fmt.Errorf("--to-signature requires --to-image")
	}
	if len(o.Mirror) > 0 && o.ReferenceMode != "" && o.ReferenceMode != "public" {
		return fmt.Errorf("--reference-mode must be public or empty when using --mirror")
	}
	return nil
}

type imageData struct {
	Ref           imagereference.DockerImageReference
	Config        *dockerv1client.DockerImageConfig
	Digest        digest.Digest
	ContentDigest digest.Digest
	Directory     string
}

func findStatusTagEvents(tags []imageapi.NamedTagEventList, name string) *imageapi.NamedTagEventList {
	for i := range tags {
		tag := &tags[i]
		if tag.Tag != name {
			continue
		}
		return tag
	}
	return nil
}

func findStatusTagEvent(tags []imageapi.NamedTagEventList, name string) *imageapi.TagEvent {
	events := findStatusTagEvents(tags, name)
	if events == nil || len(events.Items) == 0 {
		return nil
	}
	return &events.Items[0]
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

	// check parameters
	extraComponentVersions, err := parseComponentVersionsLabel(o.ExtraComponentVersions)
	if err != nil {
		return fmt.Errorf("--component-versions is invalid: %v", err)
	}
	if len(o.Name) > 0 {
		if _, err := semver.Parse(o.Name); err != nil {
			return fmt.Errorf("--name must be a semantic version: %v", err)
		}
	}
	if len(o.ReleaseMetadata) > 0 {
		if err := json.Unmarshal([]byte(o.ReleaseMetadata), &CincinnatiMetadata{}); err != nil {
			return fmt.Errorf("invalid --metadata: %v", err)
		}
	}

	hasMetadataOverrides := len(o.Name) > 0 ||
		len(o.ReleaseMetadata) > 0 ||
		len(o.PreviousVersions) > 0 ||
		len(o.ToImageBase) > 0 ||
		len(o.ExtraComponentVersions) > 0

	exclude := sets.NewString()
	for _, s := range o.Exclude {
		exclude.Insert(s)
	}

	metadata := make(map[string]imageData)
	var ordered []string
	var is *imageapi.ImageStream
	now := time.Now().UTC().Truncate(time.Second)

	switch {
	case len(o.FromReleaseImage) > 0:
		ref, err := imagereference.Parse(o.FromReleaseImage)
		if err != nil {
			return fmt.Errorf("--from-release was not a valid pullspec: %v", err)
		}

		verifier := imagemanifest.NewVerifier()
		var releaseDigest digest.Digest
		var baseDigest string
		var imageReferencesData, releaseMetadata []byte

		buf := &bytes.Buffer{}
		extractOpts := extract.NewOptions(genericclioptions.IOStreams{Out: buf, ErrOut: o.ErrOut})
		extractOpts.SecurityOptions = o.SecurityOptions
		extractOpts.OnlyFiles = true
		extractOpts.Mappings = []extract.Mapping{
			{
				ImageRef: ref,
				From:     "release-manifests/",
			},
		}
		extractOpts.ImageMetadataCallback = func(m *extract.Mapping, dgst, contentDigest digest.Digest, config *dockerv1client.DockerImageConfig) {
			verifier.Verify(dgst, contentDigest)
			releaseDigest = contentDigest
			if config.Config != nil {
				baseDigest = config.Config.Labels[annotationReleaseBaseImageDigest]
				klog.V(4).Infof("Release image was built on top of %s", baseDigest)
			}
		}
		extractOpts.TarEntryCallback = func(hdr *tar.Header, _ extract.LayerInfo, r io.Reader) (bool, error) {
			var err error
			if hdr.Name == "image-references" {
				imageReferencesData, err = ioutil.ReadAll(r)
				if err != nil {
					return false, err
				}
			}
			if hdr.Name == "release-metadata" {
				releaseMetadata, err = ioutil.ReadAll(r)
				if err != nil {
					return false, err
				}
			}
			if len(imageReferencesData) > 0 && len(releaseMetadata) > 0 {
				return false, nil
			}
			return true, nil
		}
		if err := extractOpts.Run(); err != nil {
			return err
		}
		if len(imageReferencesData) == 0 {
			return fmt.Errorf("release image did not contain any image-references content")
		}
		if !verifier.Verified() {
			err := fmt.Errorf("the input release image failed content verification and may have been tampered with")
			if !o.SecurityOptions.SkipVerification {
				return err
			}
			fmt.Fprintf(o.ErrOut, "warning: %v\n", err)
		}

		inputIS, err := readReleaseImageReferences(imageReferencesData)
		if err != nil {
			return fmt.Errorf("unable to load image-references from release contents: %v", err)
		}
		var cm CincinnatiMetadata
		if err := json.Unmarshal(releaseMetadata, &cm); err != nil {
			return fmt.Errorf("unable to load release-metadata from release contents: %v", err)
		}

		is = inputIS.DeepCopy()

		for _, tag := range is.Spec.Tags {
			ordered = append(ordered, tag.Name)
		}

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
		if len(o.ReleaseMetadata) == 0 && cm.Metadata != nil {
			data, err := json.Marshal(cm.Metadata)
			if err != nil {
				return fmt.Errorf("unable to marshal release metadata: %v", err)
			}
			o.ReleaseMetadata = string(data)
		}
		if o.PreviousVersions == nil {
			o.PreviousVersions = cm.Previous
		}

		if hasMetadataOverrides {
			if is.Annotations == nil {
				is.Annotations = map[string]string{}
			}
			is.Annotations[annotationReleaseFromRelease] = o.FromReleaseImage
			fmt.Fprintf(o.ErrOut, "info: Found %d images in release\n", len(is.Spec.Tags))

		} else {
			klog.V(2).Infof("No metadata changes, building canonical release")
			now = is.CreationTimestamp.Time.UTC()
			if o.VerifyOutputFn == nil {
				o.VerifyOutputFn = func(actual digest.Digest) error {
					// TODO: check contents, digests, image stream, the layers, and the manifest
					if actual != releaseDigest {
						return fmt.Errorf("the release could not be reproduced from its inputs")
					}
					return nil
				}
			}
			if len(ref.Tag) > 0 {
				fmt.Fprintf(o.ErrOut, "info: Release %s built from %d images\n", releaseDigest, len(is.Spec.Tags))
			} else {
				fmt.Fprintf(o.ErrOut, "info: Release built from %d images\n", len(is.Spec.Tags))
			}
		}

	case len(o.FromImageStream) > 0, len(o.FromImageStreamFile) > 0:
		is = &imageapi.ImageStream{}
		is.Annotations = map[string]string{}
		if len(o.FromImageStream) > 0 && len(o.Namespace) > 0 {
			is.Annotations[annotationReleaseFromImageStream] = fmt.Sprintf("%s/%s", o.Namespace, o.FromImageStream)
		}

		var inputIS *imageapi.ImageStream
		if len(o.FromImageStreamFile) > 0 {
			data, err := filenameContents(o.FromImageStreamFile, o.IOStreams.In)
			if os.IsNotExist(err) {
				return err
			}
			if err != nil {
				return fmt.Errorf("unable to read input image stream file: %v", err)
			}
			is := &imageapi.ImageStream{}
			if err := yaml.Unmarshal(data, &is); err != nil {
				return fmt.Errorf("unable to load input image stream file: %v", err)
			}
			if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
				return fmt.Errorf("unrecognized input image stream file, must be an ImageStream in image.openshift.io/v1")
			}
			inputIS = is

		} else {
			is, err := o.ImageClient.ImageV1().ImageStreams(o.Namespace).Get(o.FromImageStream, metav1.GetOptions{})
			if err != nil {
				return err
			}
			inputIS = is
		}

		if inputIS.Annotations == nil {
			inputIS.Annotations = make(map[string]string)
		}
		inputIS.Annotations[annotationBuildVersions] = extraComponentVersions.String()
		if err := resolveImageStreamTagsToReferenceMode(inputIS, is, o.ReferenceMode, exclude); err != nil {
			return err
		}

		for _, tag := range is.Spec.Tags {
			ordered = append(ordered, tag.Name)
		}

		fmt.Fprintf(o.ErrOut, "info: Found %d images in image stream\n", len(is.Spec.Tags))

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
					klog.V(2).Infof("Excluded directory %#v", f)
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
				klog.V(2).Infof("Excluded mapping %s", m.Source)
				continue
			}
			ordered = append(ordered, m.Source)
		}
	}

	name := o.Name
	if len(name) == 0 {
		name = "0.0.1-" + now.Format("2006-01-02-150405")
	}

	cm := &CincinnatiMetadata{Kind: "cincinnati-metadata-v0"}
	semverName, err := semver.Parse(name)
	if err != nil {
		return fmt.Errorf("--name must be a semantic version")
	}
	cm.Version = semverName.String()
	if len(o.ReleaseMetadata) > 0 {
		if err := json.Unmarshal([]byte(o.ReleaseMetadata), &cm.Metadata); err != nil {
			return fmt.Errorf("invalid --metadata: %v", err)
		}
	}
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
	klog.V(4).Infof("Release metadata:\n%s", toJSONString(cm))

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
			klog.V(2).Infof("Excluded mapping %s", m.Source)
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
		if tag.Annotations == nil {
			tag.Annotations = make(map[string]string)
		}
		tag.Annotations[annotationReleaseOverride] = "true"
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

	if len(o.FromDirectory) == 0 {
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
		if err := o.mirrorImages(is); err != nil {
			return err
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
				s := string(data)
				if len(s) > 30 {
					s = s[:30] + "..."
				}
				m, ok := obj.(map[string]interface{})
				if !ok {
					return fmt.Errorf("%s: not a valid YAML/JSON object, got: %s", filename, s)
				}
				if s, ok := m["kind"].(string); !ok || s == "" {
					return fmt.Errorf("%s: manifests must contain Kubernetes API objects with 'kind' and 'apiVersion' set: %s", filename, s)
				}
				if s, ok := m["apiVersion"].(string); !ok || s == "" {
					return fmt.Errorf("%s: manifests must contain Kubernetes API objects with 'kind' and 'apiVersion' set: %s", filename, s)
				}
				break
			}
			return nil
		})
	}

	// any input image with content, referenced in AlwaysInclude, or referenced from image-references is
	// included, which guarantees the content of a payload can be reproduced
	forceInclude := append(append([]string{}, o.AlwaysInclude...), ordered...)
	if err := pruneUnreferencedImageStreams(o.ErrOut, is, metadata, forceInclude); err != nil {
		return err
	}

	// use a stable ordering for operators
	sort.Strings(ordered)

	var operators []string
	pr, pw := io.Pipe()
	go func() {
		var err error
		operators, err = writePayload(pw, is, cm, ordered, metadata, o.AllowMissingImages, verifiers)
		pw.CloseWithError(err)
	}()

	br := bufio.NewReaderSize(pr, 500*1024)
	if _, err := br.Peek(br.Size()); err != nil && err != io.EOF {
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
	}

	return nil
}

func resolveImageStreamTagsToReferenceMode(inputIS, is *imageapi.ImageStream, referenceMode string, exclude sets.String) error {
	switch referenceMode {
	case "public", "", "source":
		forceExternal := referenceMode == "public" || referenceMode == ""
		internal := inputIS.Status.DockerImageRepository
		external := inputIS.Status.PublicDockerImageRepository
		if forceExternal && len(external) == 0 {
			return fmt.Errorf("only image streams or releases with public image repositories can be the source for releases when using the default --reference-mode")
		}

		externalFn := func(source, image string) string {
			// filter source URLs
			if len(source) > 0 && len(internal) > 0 && strings.HasPrefix(source, internal) {
				klog.V(2).Infof("Can't use source %s because it points to the internal registry", source)
				source = ""
			}
			// default to the external registry name
			if (forceExternal || len(source) == 0) && len(external) > 0 {
				return external + "@" + image
			}
			return source
		}

		covered := sets.NewString()
		for _, ref := range inputIS.Spec.Tags {
			if exclude.Has(ref.Name) {
				klog.V(2).Infof("Excluded spec tag %s", ref.Name)
				continue
			}

			if ref.From != nil && ref.From.Kind == "DockerImage" {
				switch from, err := imagereference.Parse(ref.From.Name); {
				case err != nil:
					return err

				case len(from.ID) > 0:
					source := externalFn(ref.From.Name, from.ID)
					if len(source) == 0 {
						klog.V(2).Infof("Can't use spec tag %q because we cannot locate or calculate a source location", ref.Name)
						continue
					}

					ref := ref.DeepCopy()
					ref.From = &corev1.ObjectReference{Kind: "DockerImage", Name: source}
					is.Spec.Tags = append(is.Spec.Tags, *ref)
					covered.Insert(ref.Name)

				case len(from.Tag) > 0:
					tag := findStatusTagEvents(inputIS.Status.Tags, ref.Name)
					if tag == nil {
						continue
					}
					if len(tag.Items) == 0 {
						for _, condition := range tag.Conditions {
							if condition.Type == imageapi.ImportSuccess && condition.Status != metav1.StatusSuccess {
								return fmt.Errorf("the tag %q in the source input stream has not been imported yet", tag.Tag)
							}
						}
						continue
					}
					if ref.Generation != nil && *ref.Generation != tag.Items[0].Generation {
						return fmt.Errorf("the tag %q in the source input stream has not been imported yet", tag.Tag)
					}
					if len(tag.Items[0].Image) == 0 {
						return fmt.Errorf("the tag %q in the source input stream has no image id", tag.Tag)
					}

					source := externalFn(tag.Items[0].DockerImageReference, tag.Items[0].Image)
					ref := ref.DeepCopy()
					ref.From = &corev1.ObjectReference{Kind: "DockerImage", Name: source}
					is.Spec.Tags = append(is.Spec.Tags, *ref)
					covered.Insert(ref.Name)
				}
				continue
			}
			// TODO: support ImageStreamTag and ImageStreamImage
		}

		for _, tag := range inputIS.Status.Tags {
			if covered.Has(tag.Tag) {
				continue
			}
			if exclude.Has(tag.Tag) {
				klog.V(2).Infof("Excluded status tag %s", tag.Tag)
				continue
			}

			// error if we haven't imported anything to this tag, or skip otherwise
			if len(tag.Items) == 0 {
				for _, condition := range tag.Conditions {
					if condition.Type == imageapi.ImportSuccess && condition.Status != metav1.StatusSuccess {
						return fmt.Errorf("the tag %q in the source input stream has not been imported yet", tag.Tag)
					}
				}
				continue
			}
			// skip rather than error (user created a reference spec tag, then deleted it)
			if len(tag.Items[0].Image) == 0 {
				klog.V(2).Infof("the tag %q in the source input stream has no image id", tag.Tag)
				continue
			}

			// attempt to identify the source image
			source := externalFn(tag.Items[0].DockerImageReference, tag.Items[0].Image)
			if len(source) == 0 {
				klog.V(2).Infof("Can't use tag %q because we cannot locate or calculate a source location", tag.Tag)
				continue
			}
			sourceRef, err := imagereference.Parse(source)
			if err != nil {
				return fmt.Errorf("the tag %q points to source %q which is not valid", tag.Tag, source)
			}
			sourceRef.Tag = ""
			sourceRef.ID = tag.Items[0].Image
			source = sourceRef.Exact()

			ref := &imageapi.TagReference{Name: tag.Tag}
			ref.From = &corev1.ObjectReference{Kind: "DockerImage", Name: source}
			is.Spec.Tags = append(is.Spec.Tags, *ref)
		}
		return nil
	default:
		return fmt.Errorf("supported reference modes are: \"public\" (default) and \"source\"")
	}
}

func (o *NewOptions) extractManifests(is *imageapi.ImageStream, name string, metadata map[string]imageData) error {
	if len(is.Spec.Tags) == 0 {
		return fmt.Errorf("no component images defined, unable to build a release payload")
	}

	klog.V(4).Infof("Extracting manifests for release from input images")

	dir := o.Directory
	if len(dir) == 0 {
		var err error
		dir, err = ioutil.TempDir("", fmt.Sprintf("release-image-%s", name))
		if err != nil {
			return err
		}
		o.cleanupFns = append(o.cleanupFns, func() { os.RemoveAll(dir) })
		klog.V(2).Infof("Manifests will be extracted to %s\n", dir)
	}

	verifier := imagemanifest.NewVerifier()
	var lock sync.Mutex
	opts := extract.NewOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.SecurityOptions = o.SecurityOptions
	opts.OnlyFiles = true
	opts.ParallelOptions = o.ParallelOptions
	opts.ImageMetadataCallback = func(m *extract.Mapping, dgst, contentDigest digest.Digest, config *dockerv1client.DockerImageConfig) {
		verifier.Verify(dgst, contentDigest)

		lock.Lock()
		defer lock.Unlock()
		metadata[m.Name] = imageData{
			Directory:     m.To,
			Ref:           m.ImageRef,
			Config:        config,
			Digest:        dgst,
			ContentDigest: contentDigest,
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

		// when the user provides an override, look at all layers for manifests
		// in case the user did a layered build and overrode only one. This is
		// an unsupported release configuration
		var custom bool
		filter := extract.NewPositionLayerFilter(-1)
		if tag.Annotations[annotationReleaseOverride] == "true" {
			custom = true
			filter = nil
		}

		opts.Mappings = append(opts.Mappings, extract.Mapping{
			Name:     tag.Name,
			ImageRef: ref,

			From: "manifests/",
			To:   dstDir,

			LayerFilter: filter,

			ConditionFn: func(m *extract.Mapping, dgst digest.Digest, imageConfig *dockerv1client.DockerImageConfig) (bool, error) {
				var labels map[string]string
				if imageConfig.Config != nil {
					labels = imageConfig.Config.Labels
				}
				if tag.Annotations == nil {
					tag.Annotations = make(map[string]string)
				}
				tag.Annotations[annotationBuildSourceCommit] = labels[annotationBuildSourceCommit]
				tag.Annotations[annotationBuildSourceRef] = labels[annotationBuildSourceRef]
				tag.Annotations[annotationBuildSourceLocation] = labels[annotationBuildSourceLocation]

				if versions := labels[annotationBuildVersions]; len(versions) > 0 {
					components, err := parseComponentVersionsLabel(versions)
					if err != nil {
						return false, fmt.Errorf("tag %q has an invalid %s label: %v", tag.Name, annotationBuildVersions, err)
					}
					// TODO: eventually this can be relaxed
					for component := range components {
						if !stringArrContains(o.AllowedComponents, component) {
							return false, fmt.Errorf("tag %q references a component version %q which is not in the allowed list", tag.Name, component)
						}
					}
					tag.Annotations[annotationBuildVersions] = versions
				}

				if len(labels[annotationReleaseOperator]) == 0 {
					klog.V(2).Infof("Image %s has no %s label, skipping", m.ImageRef, annotationReleaseOperator)
					return false, nil
				}
				if err := os.MkdirAll(dstDir, 0777); err != nil {
					return false, err
				}
				if custom {
					fmt.Fprintf(o.ErrOut, "info: Loading override %s %s\n", m.ImageRef.Exact(), tag.Name)
				} else {
					fmt.Fprintf(o.ErrOut, "info: Loading %s %s\n", m.ImageRef.ID, tag.Name)
				}
				return true, nil
			},
		})
	}
	klog.V(4).Infof("Manifests will be extracted from:\n%#v", opts.Mappings)
	if err := opts.Run(); err != nil {
		return err
	}

	if !verifier.Verified() {
		err := fmt.Errorf("one or more input images failed content verification and may have been tampered with")
		if !o.SecurityOptions.SkipVerification {
			return err
		}
		fmt.Fprintf(o.ErrOut, "warning: %v\n", err)
	}

	if len(is.Spec.Tags) > 0 {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return err
		}
		data, err := json.MarshalIndent(is, "", "  ")
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(dir, "image-references"), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (o *NewOptions) mirrorImages(is *imageapi.ImageStream) error {
	klog.V(4).Infof("Mirroring release contents to %s", o.Mirror)
	copied := is.DeepCopy()
	opts := NewMirrorOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})
	opts.DryRun = o.DryRun
	opts.ImageStream = copied
	opts.To = o.Mirror
	opts.SkipRelease = true
	opts.SecurityOptions = o.SecurityOptions

	if err := opts.Run(); err != nil {
		return err
	}

	targetFn, err := ComponentReferencesForImageStream(copied)
	if err != nil {
		return err
	}

	replacements, err := ReplacementsForImageStream(is, false, targetFn)
	if err != nil {
		return err
	}
	for i := range is.Spec.Tags {
		tag := &is.Spec.Tags[i]
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		if value, ok := replacements[tag.From.Name]; ok {
			tag.From.Name = value
		}
	}
	if klog.V(4) {
		data, _ := json.MarshalIndent(is, "", "  ")
		klog.Infof("Image references updated to:\n%s", string(data))
	}

	return nil
}

func (o *NewOptions) write(r io.Reader, is *imageapi.ImageStream, now time.Time) error {
	var exitErr error
	switch {
	case len(o.ToDir) > 0:
		klog.V(4).Infof("Writing release contents to directory %s", o.ToDir)
		if err := os.MkdirAll(o.ToDir, 0777); err != nil {
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
			itemPath := filepath.Join(o.ToDir, name)
			f, err := os.OpenFile(itemPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
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
			if err := os.Chtimes(itemPath, hdr.ModTime, hdr.ModTime); err != nil {
				klog.V(2).Infof("Unable to update extracted file time: %v", err)
			}
		}
	case len(o.ToFile) > 0:
		klog.V(4).Infof("Writing release contents to file %s", o.ToFile)
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
		if o.ToFile != "-" {
			if err := os.Chtimes(o.ToFile, is.CreationTimestamp.Time, is.CreationTimestamp.Time); err != nil {
				klog.V(2).Infof("Unable to set timestamps on output file: %v", err)
			}
		}
	default:
		if len(o.ToImage) == 0 {
			o.DryRun = true
			o.ToImage = "release:latest"
		}
		klog.V(4).Infof("Writing release contents to image %s", o.ToImage)
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

		verifier := imagemanifest.NewVerifier()
		options := imageappend.NewAppendImageOptions(genericclioptions.IOStreams{Out: ioutil.Discard, ErrOut: o.ErrOut})
		options.SecurityOptions = o.SecurityOptions
		options.DryRun = o.DryRun
		options.From = toImageBase
		options.ConfigurationCallback = func(dgst, contentDigest digest.Digest, config *dockerv1client.DockerImageConfig) error {
			verifier.Verify(dgst, contentDigest)
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
			config.History = []dockerv1client.DockerConfigHistory{
				{Comment: "Release image for OpenShift", Created: now},
			}
			if len(dgst) > 0 {
				config.Config.Labels[annotationReleaseBaseImageDigest] = dgst.String()
			}
			return nil
		}

		options.LayerStream = r
		options.To = toRef.Exact()
		if err := options.Run(); err != nil {
			return err
		}
		if !verifier.Verified() {
			err := fmt.Errorf("the base image failed content verification and may have been tampered with")
			if !o.SecurityOptions.SkipVerification {
				return err
			}
			fmt.Fprintf(o.ErrOut, "warning: %v\n", err)
		}
		if !o.DryRun {
			fmt.Fprintf(o.ErrOut, "info: Pushed to %s\n", o.ToImage)
		}

		if o.VerifyOutputFn != nil {
			if err := o.VerifyOutputFn(options.ToDigest); err != nil {
				if o.DryRun {
					return err
				}
				exitErr = err
			}
		}

		toRefWithDigest := toRef
		toRefWithDigest.Tag = ""
		toRefWithDigest.ID = options.ToDigest.String()
		msg, err := createReleaseSignatureMessage(fmt.Sprintf("oc-adm-release-new/%s", version.Get().GitCommit), now, options.ToDigest.String(), toRefWithDigest.Exact())
		if err != nil {
			return err
		}
		if len(o.ToSignature) > 0 {
			if err := ioutil.WriteFile(o.ToSignature, msg, 0644); err != nil {
				return fmt.Errorf("unable to write signature file: %v", err)
			}
		} else {
			klog.V(2).Infof("Signature for output:\n%s", string(msg))
		}

		fmt.Fprintf(o.Out, "%s %s %s\n", options.ToDigest.String(), is.Name, is.CreationTimestamp.Format(time.RFC3339))
	}
	return exitErr
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

func writePayload(w io.Writer, is *imageapi.ImageStream, cm *CincinnatiMetadata, ordered []string, metadata map[string]imageData, allowMissingImages bool, verifiers []PayloadVerifier) ([]string, error) {
	var operators []string
	directories := make(map[string]struct{})
	files := make(map[string]int)

	parts := []string{"release-manifests"}

	// find the newest content date in the input
	var newest time.Time
	if err := iterateExtractedManifests(ordered, metadata, func(contents []os.FileInfo, name string, image imageData) error {
		for _, fi := range contents {
			if fi.IsDir() {
				continue
			}
			if fi.ModTime().After(newest) {
				newest = fi.ModTime()
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	newest = newest.UTC().Truncate(time.Second)
	klog.V(4).Infof("Most recent content has date %s", newest.Format(time.RFC3339))

	gw := gzip.NewWriter(w)
	tw := tar.NewWriter(gw)

	// ensure the directory exists in the tar bundle
	if err := writeNestedTarHeader(tw, parts, directories, tar.Header{Mode: 0777, ModTime: newest, Typeflag: tar.TypeDir}); err != nil {
		return nil, err
	}

	// write image metadata to release-manifests/image-references
	data, err := json.MarshalIndent(is, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: newest, Typeflag: tar.TypeReg, Name: path.Join(append(append([]string{}, parts...), "image-references")...), Size: int64(len(data))}); err != nil {
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
		if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: newest, Typeflag: tar.TypeReg, Name: path.Join(append(append([]string{}, parts...), "release-metadata")...), Size: int64(len(data))}); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}

	// read each directory, processing the manifests in order and updating the contents into the tar output
	if err := iterateExtractedManifests(ordered, metadata, func(contents []os.FileInfo, name string, image imageData) error {
		transform := NopManifestMapper

		if fi := takeFileByName(&contents, "image-references"); fi != nil {
			path := filepath.Join(image.Directory, fi.Name())
			klog.V(2).Infof("Perform image replacement based on inclusion of %s", path)
			transform, err = NewTransformFromImageStreamFile(path, is, allowMissingImages)
			if err != nil {
				return fmt.Errorf("operator %q contained an invalid image-references file: %s", name, err)
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
				filename = fmt.Sprintf("0000_50_%s_%s", name, filename)
			}
			if count, ok := files[filename]; ok {
				ext := path.Ext(path.Base(filename))
				files[filename] = count + 1
				filename = fmt.Sprintf("%s_%d%s", strings.TrimSuffix(filename, ext), count+1, ext)
				files[filename] = 1
			} else {
				files[filename] = 1
			}
			src := filepath.Join(image.Directory, fi.Name())
			dst := path.Join(append(append([]string{}, parts...), filename)...)
			klog.V(4).Infof("Copying %s to %s", src, dst)

			data, err := ioutil.ReadFile(src)
			if err != nil {
				return err
			}

			for _, fn := range verifiers {
				if err := fn(filepath.Join(filepath.Base(image.Directory), fi.Name()), data); err != nil {
					return err
				}
			}

			modified, err := transform(data)
			if err != nil {
				return err
			}
			if err := tw.WriteHeader(&tar.Header{Mode: 0444, ModTime: fi.ModTime(), Typeflag: tar.TypeReg, Name: dst, Size: int64(len(modified))}); err != nil {
				return err
			}
			klog.V(6).Infof("Writing payload to %s\n%s", dst, string(modified))
			if _, err := tw.Write(modified); err != nil {
				return err
			}
		}
		operators = append(operators, name)
		return nil
	}); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return operators, nil
}

func iterateExtractedManifests(ordered []string, metadata map[string]imageData, fn func(contents []os.FileInfo, name string, image imageData) error) error {
	for _, name := range ordered {
		image, ok := metadata[name]
		if !ok {
			return fmt.Errorf("missing image data %s", name)
		}

		// process each manifest in the given directory
		contents, err := ioutil.ReadDir(image.Directory)
		if err != nil {
			return err
		}
		if len(contents) == 0 {
			continue
		}

		if err := fn(contents, name, image); err != nil {
			return err
		}
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
		klog.V(4).Infof("Component %s does not have any manifests", path)
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
			klog.V(3).Infof("Excluding tag %s which is not referenced by an operator", tag.Name)
			continue
		}
		updated = append(updated, tag)
	}
	if len(updated) != len(is.Spec.Tags) {
		fmt.Fprintf(out, "info: Included %d images from %d input operators into the release\n", len(updated), len(metadata))
		is.Spec.Tags = updated
	}
	return nil
}

func filenameContents(s string, in io.Reader) ([]byte, error) {
	switch {
	case s == "-":
		return ioutil.ReadAll(in)
	case strings.Index(s, "http://") == 0 || strings.Index(s, "https://") == 0:
		resp, err := http.Get(s)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			return ioutil.ReadAll(resp.Body)
		default:
			return nil, fmt.Errorf("unable to load URL: server returned %d: %s", resp.StatusCode, resp.Status)
		}
	default:
		return ioutil.ReadFile(s)
	}
}

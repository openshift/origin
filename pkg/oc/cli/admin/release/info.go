package release

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"path"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/blang/semver"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	digest "github.com/opencontainers/go-digest"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	imageapi "github.com/openshift/api/image/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/oc/cli/image/extract"
)

func NewInfoOptions(streams genericclioptions.IOStreams) *InfoOptions {
	return &InfoOptions{
		IOStreams: streams,
	}
}

func NewInfo(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewInfoOptions(streams)
	cmd := &cobra.Command{
		Use:   "info IMAGE [--changes-from=IMAGE]",
		Short: "Display information about a release",
		Long: templates.LongDesc(`
			Show information about an OpenShift release

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&o.From, "changes-from", o.From, "Show changes from this image to the requested image.")
	flags.BoolVar(&o.ShowCommit, "commits", o.ShowCommit, "Display information about the source an image was created with.")
	flags.BoolVar(&o.ShowPullSpec, "pullspecs", o.ShowPullSpec, "Display the pull spec of each image instead of the digest.")
	flags.StringVar(&o.ImageFor, "image-for", o.ImageFor, "Print the pull spec of the specified image or an error if it does not exist.")
	flags.StringVarP(&o.Output, "output", "o", o.Output, "Display the release info in an alternative format: json")
	return cmd
}

type InfoOptions struct {
	genericclioptions.IOStreams

	Images []string
	From   string

	Output       string
	ImageFor     string
	ShowCommit   bool
	ShowPullSpec bool
	Verify       bool
}

func (o *InfoOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		cfg, err := f.ToRESTConfig()
		if err != nil {
			return fmt.Errorf("info expects one argument, or a connection to a 4.0 OpenShift server: %v", err)
		}
		client, err := configv1client.NewForConfig(cfg)
		if err != nil {
			return fmt.Errorf("info expects one argument, or a connection to a 4.0 OpenShift server: %v", err)
		}
		cv, err := client.Config().ClusterVersions().Get("version", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return fmt.Errorf("you must be connected to a v4.0 OpenShift server to fetch the current version")
			}
			return fmt.Errorf("info expects one argument, or a connection to a 4.0 OpenShift server: %v", err)
		}
		image := cv.Status.Desired.Image
		if len(image) == 0 && cv.Spec.DesiredUpdate != nil {
			image = cv.Spec.DesiredUpdate.Image
		}
		if len(image) == 0 {
			return fmt.Errorf("the server is not reporting a release image at this time, please specify an image to view")
		}
		args = []string{image}
	}
	if len(args) < 1 {
		return fmt.Errorf("info expects at least one argument, a release image pull spec")
	}
	o.Images = args
	return nil
}

func (o *InfoOptions) Validate() error {
	if len(o.ImageFor) > 0 && len(o.Output) > 0 {
		return fmt.Errorf("--output and --image-for may not both be specified")
	}
	switch o.Output {
	case "", "json":
	default:
		return fmt.Errorf("--output only supports 'json'")
	}
	return nil
}

func (o *InfoOptions) Run() error {
	if len(o.Images) == 0 {
		return fmt.Errorf("must specify a release image as an argument")
	}
	if len(o.From) > 0 && len(o.Images) != 1 {
		return fmt.Errorf("must specify a single release image as argument when comparing to another release image")
	}

	if len(o.From) > 0 {
		var baseRelease *ReleaseInfo
		var baseErr error
		done := make(chan struct{})
		go func() {
			defer close(done)
			baseRelease, baseErr = o.LoadReleaseInfo(o.From)
		}()

		release, err := o.LoadReleaseInfo(o.Images[0])
		if err != nil {
			return err
		}

		<-done
		if baseErr != nil {
			return baseErr
		}
		diff, err := calculateDiff(baseRelease, release)
		if err != nil {
			return err
		}
		return describeReleaseDiff(o.Out, diff, o.ShowCommit)
	}

	var exitErr error
	for _, image := range o.Images {
		if err := o.describeImage(image); err != nil {
			exitErr = kcmdutil.ErrExit
			fmt.Fprintf(o.ErrOut, "error: %v\n", err)
			continue
		}
	}
	return exitErr
}

func (o *InfoOptions) describeImage(image string) error {
	release, err := o.LoadReleaseInfo(image)
	if err != nil {
		return err
	}
	if len(o.Output) > 0 {
		data, err := json.MarshalIndent(release, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(o.Out, string(data))
		return nil
	}
	if len(o.ImageFor) > 0 {
		spec, err := findImageSpec(release.References, o.ImageFor, image)
		if err != nil {
			return err
		}
		fmt.Fprintln(o.Out, spec)
		return nil
	}
	return describeReleaseInfo(o.Out, release, o.ShowCommit, o.ShowPullSpec)
}

func findImageSpec(image *imageapi.ImageStream, tagName, imageName string) (string, error) {
	for _, tag := range image.Spec.Tags {
		if tag.Name == tagName {
			if tag.From != nil && tag.From.Kind == "DockerImage" && len(tag.From.Name) > 0 {
				return tag.From.Name, nil
			}
		}
	}
	return "", fmt.Errorf("no image tag %q exists in the release image %s", tagName, imageName)
}

func calculateDiff(from, to *ReleaseInfo) (*ReleaseDiff, error) {
	diff := &ReleaseDiff{
		From:             from,
		To:               to,
		ChangedImages:    make(map[string]*ImageReferenceDiff),
		ChangedManifests: make(map[string]*ReleaseManifestDiff),
	}
	for i := range from.References.Spec.Tags {
		tag := &from.References.Spec.Tags[i]
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		diff.ChangedImages[tag.Name] = &ImageReferenceDiff{
			Name: tag.Name,
			From: tag,
		}
	}
	for i := range to.References.Spec.Tags {
		tag := &to.References.Spec.Tags[i]
		if tag.From == nil || tag.From.Kind != "DockerImage" {
			continue
		}
		if exists, ok := diff.ChangedImages[tag.Name]; ok {
			exists.To = tag
			continue
		}
		diff.ChangedImages[tag.Name] = &ImageReferenceDiff{
			Name: tag.Name,
			To:   tag,
		}
	}
	for k, v := range diff.ChangedImages {
		if v.From != nil && v.To != nil && v.From.From.Name == v.To.From.Name {
			delete(diff.ChangedImages, k)
		}
	}
	for name, manifest := range from.ManifestFiles {
		diff.ChangedManifests[name] = &ReleaseManifestDiff{
			Filename: name,
			From:     manifest,
		}
	}
	for name, manifest := range to.ManifestFiles {
		if exists, ok := diff.ChangedManifests[name]; ok {
			exists.To = manifest
			continue
		}
		diff.ChangedManifests[name] = &ReleaseManifestDiff{
			Filename: name,
			From:     manifest,
		}
	}
	for k, v := range diff.ChangedManifests {
		if bytes.Equal(v.From, v.To) {
			delete(diff.ChangedManifests, k)
		}
	}

	return diff, nil
}

type ReleaseDiff struct {
	From, To *ReleaseInfo

	ChangedImages    map[string]*ImageReferenceDiff
	ChangedManifests map[string]*ReleaseManifestDiff
}

type ImageReferenceDiff struct {
	Name string

	From, To *imageapi.TagReference
}

type ReleaseManifestDiff struct {
	Filename string

	From, To []byte
}

type ReleaseInfo struct {
	Image      string                              `json:"image"`
	ImageRef   imagereference.DockerImageReference `json:"-"`
	Digest     digest.Digest                       `json:"digest"`
	Config     *docker10.DockerImageConfig         `json:"config"`
	Metadata   *CincinnatiMetadata                 `json:"metadata"`
	References *imageapi.ImageStream               `json:"references"`

	ManifestFiles map[string][]byte `json:"-"`
	UnknownFiles  []string          `json:"-"`
}

func (i *ReleaseInfo) PreferredName() string {
	if i.Metadata != nil {
		return i.Metadata.Version
	}
	return i.References.Name
}

func (i *ReleaseInfo) Platform() string {
	os := i.Config.OS
	if len(os) > 0 {
		os = "unknown"
	}
	arch := i.Config.Architecture
	if len(arch) == 0 {
		arch = "unknown"
	}
	return fmt.Sprintf("%s/%s", os, arch)
}

func (o *InfoOptions) LoadReleaseInfo(image string) (*ReleaseInfo, error) {
	ref, err := imagereference.Parse(image)
	if err != nil {
		return nil, err
	}
	opts := extract.NewOptions(genericclioptions.IOStreams{Out: o.Out, ErrOut: o.ErrOut})

	release := &ReleaseInfo{
		Image: image,
	}

	opts.ImageMetadataCallback = func(m *extract.Mapping, dgst digest.Digest, config *docker10.DockerImageConfig) {
		release.Digest = dgst
		release.Config = config
	}
	opts.OnlyFiles = true
	opts.Mappings = []extract.Mapping{
		{
			ImageRef: ref,

			From:        "release-manifests/",
			To:          ".",
			LayerFilter: extract.NewPositionLayerFilter(-1),
		},
	}
	var errs []error
	opts.TarEntryCallback = func(hdr *tar.Header, _ extract.LayerInfo, r io.Reader) (bool, error) {
		switch hdr.Name {
		case "image-references":
			data, err := ioutil.ReadAll(r)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to read release image-references: %v", err))
				return true, nil
			}
			is, err := readReleaseImageReferences(data)
			if err != nil {
				errs = append(errs, err)
				return true, nil
			}
			release.References = is
		case "release-metadata":
			data, err := ioutil.ReadAll(r)
			if err != nil {
				errs = append(errs, fmt.Errorf("unable to read release metadata: %v", err))
				return true, nil
			}
			m := &CincinnatiMetadata{}
			if err := json.Unmarshal(data, m); err != nil {
				errs = append(errs, fmt.Errorf("invalid release metadata: %v", err))
				return true, nil
			}
			release.Metadata = m
		default:
			if ext := path.Ext(hdr.Name); len(ext) > 0 && (ext == ".yaml" || ext == ".yml" || ext == ".json") {
				glog.V(4).Infof("Found manifest %s", hdr.Name)
				data, err := ioutil.ReadAll(r)
				if err != nil {
					errs = append(errs, fmt.Errorf("unable to read release manifest %q: %v", hdr.Name, err))
					return true, nil
				}
				if release.ManifestFiles == nil {
					release.ManifestFiles = make(map[string][]byte)
				}
				release.ManifestFiles[hdr.Name] = data
			} else {
				release.UnknownFiles = append(release.UnknownFiles, hdr.Name)
			}
		}
		return true, nil
	}
	if err := opts.Run(); err != nil {
		return nil, err
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("release image could not be read: %s", errorList(errs))
	}
	if release.References == nil {
		return nil, fmt.Errorf("release image did not contain an image-references file")
	}
	return release, nil
}

func errorList(errs []error) string {
	if len(errs) == 1 {
		return errs[0].Error()
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "\n\n")
	for _, err := range errs {
		fmt.Fprintf(buf, "* %v\n", err)
	}
	return buf.String()
}

func stringArrContains(arr []string, s string) bool {
	for _, item := range arr {
		if item == s {
			return true
		}
	}
	return false
}

func describeReleaseDiff(out io.Writer, diff *ReleaseDiff, showCommit bool) error {
	if diff.To.Digest == diff.From.Digest {
		fmt.Fprintf(out, "Releases are identical\n")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	defer w.Flush()
	now := time.Now()
	fmt.Fprintf(w, "\tFROM\tTO\n")
	fmt.Fprintf(w, "Name:\t%s\t%s\n", diff.From.PreferredName(), diff.To.PreferredName())
	fmt.Fprintf(w, "Created:\t%s\t%s\n", duration.ShortHumanDuration(now.Sub(diff.From.Config.Created)), duration.ShortHumanDuration(now.Sub(diff.To.Config.Created)))
	if from, to := diff.From.Platform(), diff.To.Platform(); from != to {
		fmt.Fprintf(w, "OS/Arch:\t%s\t%s\n", from, to)
	}

	switch {
	case diff.From.Metadata != nil && diff.To.Metadata != nil:
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Version:\t%s\t%s\n", diff.From.Metadata.Version, diff.To.Metadata.Version)
		canUpgrade := "No"
		if stringArrContains(diff.To.Metadata.Previous, diff.From.Metadata.Version) {
			canUpgrade = "Yes"
		}
		fmt.Fprintf(w, "Upgrade From:\t\t%s\n", canUpgrade)
	case diff.From.Metadata != nil && diff.To.Metadata == nil:
		fmt.Fprintf(w, "Has Release Metadata:\tYes\t\n")
	case diff.From.Metadata == nil && diff.To.Metadata != nil:
		fmt.Fprintf(w, "Has Release Metadata:\t\tYes\n")
	}

	if len(diff.ChangedImages) > 0 {
		var keys []string
		for k := range diff.ChangedImages {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		writeTabSection(w, func(w io.Writer) {
			count := 0
			for _, k := range keys {
				if image := diff.ChangedImages[k]; image.To != nil && image.From != nil {
					if count == 0 {
						fmt.Fprintln(w)
						fmt.Fprintf(w, "Images Changed:\n")
					}
					count++
					old, new := digestOrRef(image.From.From.Name), digestOrRef(image.To.From.Name)
					if old != new {
						if showCommit {
							fmt.Fprintf(w, "  %s\t%s\n", image.Name, gitDiffOrCommit(image.From, image.To))
						} else {
							fmt.Fprintf(w, "  %s\t%s\t%s\n", image.Name, old, new)
						}
					}
				}
			}
		})

		writeTabSection(w, func(w io.Writer) {
			count := 0
			for _, k := range keys {
				if image := diff.ChangedImages[k]; image.From == nil {
					if count == 0 {
						fmt.Fprintln(w)
						fmt.Fprintf(w, "Images Added:\n")
					}
					count++
					if showCommit {
						fmt.Fprintf(w, "  %s\t%s\n", image.Name, repoAndCommit(image.To))
					} else {
						fmt.Fprintf(w, "  %s\t%s\n", image.Name, digestOrRef(image.To.From.Name))
					}
				}
			}
		})

		writeTabSection(w, func(w io.Writer) {
			count := 0
			for _, k := range keys {
				if image := diff.ChangedImages[k]; image.To == nil {
					if count == 0 {
						fmt.Fprintln(w)
						fmt.Fprintf(w, "Images Removed:\n")
					}
					count++
					fmt.Fprintf(w, "  %s\n", image.Name)
				}
			}
		})
	}
	fmt.Fprintln(w)
	return nil
}

func repoAndCommit(ref *imageapi.TagReference) string {
	repo := ref.Annotations["io.openshift.build.source-location"]
	commit := ref.Annotations["io.openshift.build.commit.id"]
	if len(repo) == 0 || len(commit) == 0 {
		return "<unknown>"
	}
	return fmt.Sprintf("%s %s", repo, commit)
}

func gitDiffOrCommit(from, to *imageapi.TagReference) string {
	oldRepo, newRepo := from.Annotations["io.openshift.build.source-location"], to.Annotations["io.openshift.build.source-location"]
	oldCommit, newCommit := from.Annotations["io.openshift.build.commit.id"], to.Annotations["io.openshift.build.commit.id"]
	if len(newRepo) == 0 || len(newCommit) == 0 {
		return "<unknown>"
	}
	if oldRepo == newRepo {
		if oldCommit == newCommit {
			return fmt.Sprintf("%s %s", newRepo, newCommit)
		}
		if strings.HasPrefix(newRepo, "https://github.com/") {
			if u, err := url.Parse(newRepo); err == nil {
				u.Path = path.Join(u.Path, "compare", fmt.Sprintf("%s...%s", oldCommit, newCommit))
				return u.String()
			}
		}
		return fmt.Sprintf("%s %s %s", newRepo, oldCommit, newCommit)
	}
	if len(oldCommit) == 0 {
		return fmt.Sprintf("%s <unknown> %s", newRepo, newCommit)
	}
	return fmt.Sprintf("%s %s %s", newRepo, oldCommit, newCommit)
}

func describeReleaseInfo(out io.Writer, release *ReleaseInfo, showCommit, pullSpec bool) error {
	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "Name:\t%s\n", release.PreferredName())
	fmt.Fprintf(w, "Digest:\t%s\n", release.Digest)
	fmt.Fprintf(w, "Created:\t%s\n", release.Config.Created.Local().Truncate(time.Second))
	fmt.Fprintf(w, "OS/Arch:\t%s/%s\n", release.Config.OS, release.Config.Architecture)
	fmt.Fprintf(w, "Manifests:\t%d\n", len(release.ManifestFiles))
	if len(release.UnknownFiles) > 0 {
		fmt.Fprintf(w, "Unknown files:\t%d\n", len(release.UnknownFiles))
	}
	if m := release.Metadata; m != nil {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Release Metadata:\n")
		fmt.Fprintf(w, "  Version:\t%s\n", m.Version)
		if len(m.Previous) > 0 {
			fmt.Fprintf(w, "  Upgrades:\t%s\n", strings.Join(sortSemanticVersions(m.Previous), ", "))
		} else {
			fmt.Fprintf(w, "  Upgrades:\t<none>\n")
		}
		var keys []string
		for k := range m.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		writeTabSection(w, func(w io.Writer) {
			for _, k := range keys {
				fmt.Fprintf(w, "  Metadata:\n")
				fmt.Fprintf(w, "    %s:\t%s\n", k, m.Metadata[k])
			}
		})
	}
	writeTabSection(w, func(w io.Writer) {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Images:\n")
		switch {
		case showCommit:
			fmt.Fprintf(w, "  NAME\tREPO\tCOMMIT\t\n")
			for _, tag := range release.References.Spec.Tags {
				if tag.From == nil || tag.From.Kind != "DockerImage" {
					continue
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\n", tag.Name, tag.Annotations["io.openshift.build.source-location"], tag.Annotations["io.openshift.build.commit.id"])
			}
		case pullSpec:
			fmt.Fprintf(w, "  NAME\tPULL SPEC\n")
			for _, tag := range release.References.Spec.Tags {
				if tag.From == nil || tag.From.Kind != "DockerImage" {
					continue
				}
				fmt.Fprintf(w, "  %s\t%s\n", tag.Name, tag.From.Name)
			}
		default:
			fmt.Fprintf(w, "  NAME\tDIGEST\n")
			for _, tag := range release.References.Spec.Tags {
				if tag.From == nil || tag.From.Kind != "DockerImage" {
					continue
				}
				var id string
				if ref, err := imagereference.Parse(tag.From.Name); err == nil {
					id = ref.ID
				}
				if len(id) == 0 {
					id = tag.From.Name
				}
				fmt.Fprintf(w, "  %s\t%s\n", tag.Name, id)
			}
		}
	})
	fmt.Fprintln(w)
	return nil
}

func writeTabSection(out io.Writer, fn func(w io.Writer)) {
	w := tabwriter.NewWriter(out, 0, 4, 1, ' ', 0)
	fn(w)
	w.Flush()
}

func sortSemanticVersions(versionStrings []string) []string {
	var versions []semver.Version
	for _, version := range versionStrings {
		v, err := semver.Parse(version)
		if err != nil {
			return versionStrings
		}
		versions = append(versions, v)
	}
	semver.Sort(versions)
	versionStrings = make([]string, 0, len(versions))
	for _, v := range versions {
		versionStrings = append(versionStrings, v.String())
	}
	return versionStrings
}

func digestOrRef(ref string) string {
	if ref, err := imagereference.Parse(ref); err == nil && len(ref.ID) > 0 {
		return ref.ID
	}
	return ref
}

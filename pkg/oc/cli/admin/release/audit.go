package release

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/oc/cli/image/workqueue"

	digest "github.com/opencontainers/go-digest"
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	imagereference "github.com/openshift/origin/pkg/image/apis/image/reference"
	"github.com/openshift/origin/pkg/oc/cli/image/info"
	imagemanifest "github.com/openshift/origin/pkg/oc/cli/image/manifest"
)

func NewAuditOptions(streams genericclioptions.IOStreams) *AuditOptions {
	return &AuditOptions{
		IOStreams: streams,
		ParallelOptions: imagemanifest.ParallelOptions{
			MaxPerRegistry: 4,
		},
	}
}

func NewAudit(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAuditOptions(streams)
	cmd := &cobra.Command{
		Use:   "audit REPOSITORY",
		Short: "Perform consistency checks against a release repository",
		Long: templates.LongDesc(`
			Perform consistency checks against a release repository

			Experimental: This command is under active development and may change without notice.
		`),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	flags := cmd.Flags()
	o.SecurityOptions.Bind(flags)
	o.ParallelOptions.Bind(flags)

	return cmd
}

type AuditOptions struct {
	genericclioptions.IOStreams

	SecurityOptions imagemanifest.SecurityOptions
	ParallelOptions imagemanifest.ParallelOptions

	Images []string
	From   string
}

func (o *AuditOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 1:
		o.From = args[0]
	default:
		return fmt.Errorf("a single image repository argument must be provided to audit")
	}
	return nil
}

func (o *AuditOptions) Validate() error {
	return nil
}

func (o *AuditOptions) Run() error {
	from, err := imagereference.Parse(o.From)
	if err != nil {
		return fmt.Errorf("--from must point to a repository: %v", err)
	}
	if len(from.ID) > 0 || len(from.Tag) > 0 {
		return fmt.Errorf("--from must point to a repository, not a tag or digest")
	}

	ctx := context.Background()
	context, err := o.SecurityOptions.Context()
	if err != nil {
		return err
	}

	repo, err := context.Repository(ctx, from.DockerClientDefaults().RegistryURL(), from.RepositoryName(), o.SecurityOptions.Insecure)
	if err != nil {
		return fmt.Errorf("unable to connect to image repository %s: %v", from.Exact(), err)
	}

	stopCh := make(chan struct{})
	defer close(stopCh)

	tags := newTagStore()
	images := newImageStore()

	fmt.Fprintf(o.ErrOut, "Retrieving tags ... ")
	tagSvc := repo.Tags(ctx)
	tagNames, err := tagSvc.All(ctx)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "failed\n")
		return fmt.Errorf("unable to retrieve tags: %v", err)
	}
	fmt.Fprintf(o.ErrOut, "%d found\n", len(tagNames))

	work := workqueue.New(o.ParallelOptions.MaxPerRegistry, stopCh)

	retriever := &info.ImageRetriever{
		SecurityOptions: o.SecurityOptions,
		ParallelOptions: o.ParallelOptions,

		Image: make(map[string]imagereference.DockerImageReference),
	}
	for _, name := range tagNames {
		copy := from
		copy.Tag = name
		retriever.Image[name] = copy
	}
	infoOptions := NewInfoOptions(o.IOStreams)
	infoOptions.SecurityOptions = o.SecurityOptions
	infoOptions.ParallelOptions = o.ParallelOptions

	retriever.ImageMetadataCallback = func(name string, image *info.Image, err error) error {
		if err != nil {
			tags.AddError(fmt.Errorf("unable to retrieve tag %s: %v", name, err))
			return nil
		}
		tags.Add(name, image.Digest)
		images.Add(image)
		if !isReleaseCandidate(image) {
			fmt.Fprintf(o.Out, "%s %s image %s\n", image.Digest, image.Config.Created.UTC().Format(time.RFC3339), name)
			return nil
		}

		work.Queue(func(_ workqueue.Work) {
			release, err := infoOptions.LoadReleaseInfo(image.Name, true)
			if err != nil {
				tags.AddError(fmt.Errorf("the tag %s could not be loaded as a release: %v", name, err))
				return
			}
			preferredName := release.PreferredName()
			if preferredName != name {
				tags.AddError(fmt.Errorf("the release at tag %s should be tagged %s", name, preferredName))
				return
			}
			if len(release.Warnings) > 0 {
				for _, warning := range release.Warnings {
					tags.AddError(fmt.Errorf("the release at tag %s is not valid: %s", name, warning))
				}
				return
			}
			fmt.Fprintf(o.Out, "%s %s release %s\n", image.Digest, image.Config.Created.UTC().Format(time.RFC3339), release.PreferredName())
		})
		return nil
	}
	if err := retriever.Run(); err != nil {
		return err
	}

	work.Done()

	tagErrs, imageErrs := tags.Errors(), images.Errors()
	errs := make([]string, 0, len(tagErrs)+len(imageErrs))
	for _, err := range tagErrs {
		errs = append(errs, err.Error())
	}
	for _, err := range imageErrs {
		errs = append(errs, err.Error())
	}
	sort.Strings(errs)
	uniqueStrings(errs)
	for _, err := range errs {
		fmt.Fprintf(o.ErrOut, "error: %v\n", err)
	}

	return nil
}

func isReleaseCandidate(image *info.Image) bool {
	if image.Config != nil && image.Config.Config != nil {
		return len(image.Config.Config.Labels["io.openshift.release"]) > 0
	}
	return false
}

type tagStore struct {
	lock sync.Mutex
	tags map[string]digest.Digest
	errs []error
}

func newTagStore() *tagStore {
	return &tagStore{
		tags: make(map[string]digest.Digest),
	}
}

func (s *tagStore) Add(name string, dgst digest.Digest) {
	s.lock.Lock()
	defer s.lock.Unlock()
	existing, ok := s.tags[name]
	if ok {
		if existing != dgst {
			s.errs = append(s.errs, fmt.Errorf("tag %s changed from digest %s to %s at %s", name, existing, dgst, time.Now().UTC().Format(time.RFC3339)))
		}
		return
	}
	s.tags[name] = dgst
}

func (s *tagStore) AddError(err error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.errs = append(s.errs, err)
}

func (s *tagStore) Errors() []error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.errs
}

type imageStore struct {
	lock   sync.Mutex
	images map[digest.Digest]*info.Image
	errs   []error
}

func newImageStore() *imageStore {
	return &imageStore{
		images: make(map[digest.Digest]*info.Image),
	}
}

func (s *imageStore) Add(image *info.Image) {
	s.lock.Lock()
	defer s.lock.Unlock()
	existing, ok := s.images[image.Digest]
	if ok {
		if !reflect.DeepEqual(existing.Config, image.Config) {
			s.errs = append(s.errs, fmt.Errorf("image %s has inconsistent internal contents", image.Digest))
		}
		return
	}
	s.images[image.Digest] = image
}

func (s *imageStore) Errors() []error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.errs
}

func uniqueStrings(arr []string) []string {
	var last int
	for i := 1; i < len(arr); i++ {
		if arr[i] == arr[last] {
			continue
		}
		last++
		if last != i {
			arr[last] = arr[i]
		}
	}
	if last < len(arr) {
		last++
	}
	return arr[:last]
}

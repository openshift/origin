package imageprune

import (
	"fmt"
	"io"
	"net/http"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/prune"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/dockerregistry"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const longDesc = `
`

type registryURLs []string

func (u *registryURLs) Type() string {
	return "string"
}

func (u *registryURLs) String() string {
	return fmt.Sprintf("%v", *u)
}

func (u *registryURLs) Set(value string) error {
	*u = append(*u, value)
	return nil
}

type config struct {
	DryRun                    bool
	RegistryURLs              registryURLs
	MinimumResourcePruningAge int
	TagRevisionsToKeep        int
}

func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &config{
		DryRun:                    true,
		RegistryURLs:              registryURLs{"docker-registry.default.local"},
		MinimumResourcePruningAge: 60,
		TagRevisionsToKeep:        3,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Prune images",
		Long:  fmt.Sprintf(longDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				glog.Fatalf("No arguments are allowed to this command")
			}

			osClient, kClient, err := f.Clients()
			if err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}

			if registryService, err := kClient.Services(kapi.NamespaceDefault).Get("docker-registry"); err != nil {
				glog.Errorf("Error getting docker-registry service: %v", err)
			} else {
				cfg.RegistryURLs = append(cfg.RegistryURLs, fmt.Sprintf("%s:%d", registryService.Spec.PortalIP, registryService.Spec.Ports[0].Port))
			}

			pruner, err := prune.NewImagePruner(cfg.RegistryURLs, cfg.MinimumResourcePruningAge, cfg.TagRevisionsToKeep, osClient, osClient, kClient, kClient, osClient, osClient, osClient)
			if err != nil {
				glog.Fatalf("Error creating image pruner: %v", err)
			}

			pruner = prune.NewSummarizingImagePruner(pruner, out)

			var (
				imagePruneFunc prune.ImagePruneFunc
				layerPruneFunc prune.LayerPruneFunc
			)

			switch cfg.DryRun {
			case false:
				fmt.Fprintln(out, "Dry run *disabled* - images will be pruned and data will be deleted!")
				imagePruneFunc = func(image *imageapi.Image, referencedStreams []*imageapi.ImageStream) []error {
					prune.DescribingImagePruneFunc(out)(image, referencedStreams)
					return prune.DeletingImagePruneFunc(osClient.Images(), osClient)(image, referencedStreams)
				}
				layerPruneFunc = func(registryURL string, req dockerregistry.DeleteLayersRequest) []error {
					prune.DescribingLayerPruneFunc(out)(registryURL, req)
					return prune.DeletingLayerPruneFunc(http.DefaultClient)(registryURL, req)
				}
			default:
				fmt.Fprintln(out, "Dry run enabled - no modifications will be made.")
				imagePruneFunc = prune.DescribingImagePruneFunc(out)
				layerPruneFunc = prune.DescribingLayerPruneFunc(out)
			}

			pruner.Run(imagePruneFunc, layerPruneFunc)
		},
	}

	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Perform an image pruning dry-run, displaying what would be deleted but not actually deleting anything (default=true).")
	cmd.Flags().Var(&cfg.RegistryURLs, "registry-urls", "TODO")
	cmd.Flags().IntVar(&cfg.MinimumResourcePruningAge, "older-than", cfg.MinimumResourcePruningAge, "TODO")
	cmd.Flags().IntVar(&cfg.TagRevisionsToKeep, "keep-tag-revisions", cfg.TagRevisionsToKeep, "TODO")

	return cmd
}

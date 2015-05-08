package imageprune

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/dockerregistry/server"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/prune"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const longDesc = `
`

type config struct {
	DryRun             bool
	KeepYoungerThan    time.Duration
	TagRevisionsToKeep int
}

func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &config{
		DryRun:             true,
		KeepYoungerThan:    60 * time.Minute,
		TagRevisionsToKeep: 3,
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

			pruner, err := prune.NewImagePruner(cfg.KeepYoungerThan, cfg.TagRevisionsToKeep, osClient, osClient, kClient, kClient, osClient, osClient, osClient)
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
				layerPruneFunc = func(registryURL string, req server.DeleteLayersRequest) (error, map[string][]error) {
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
	cmd.Flags().DurationVar(&cfg.KeepYoungerThan, "keep-younger-than", cfg.KeepYoungerThan, "Specify the minimum age for an image to be prunable, as well as the minimum age for an image stream or pod that references an image to be prunable.")
	cmd.Flags().IntVar(&cfg.TagRevisionsToKeep, "keep-tag-revisions", cfg.TagRevisionsToKeep, "Specify the number of image revisions for a tag in an image stream that will be preserved.")

	return cmd
}

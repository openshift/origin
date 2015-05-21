package prune

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/prune"
	"github.com/spf13/cobra"
)

const imagesLongDesc = `
`

const PruneImagesRecommendedName = "images"

type pruneImagesConfig struct {
	DryRun             bool
	KeepYoungerThan    time.Duration
	TagRevisionsToKeep int
}

func NewCmdPruneImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &pruneImagesConfig{
		DryRun:             true,
		KeepYoungerThan:    60 * time.Minute,
		TagRevisionsToKeep: 3,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Prune images",
		Long:  fmt.Sprintf(imagesLongDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				glog.Fatalf("No arguments are allowed to this command")
			}

			osClient, kClient, err := f.Clients()
			cmdutil.CheckErr(err)

			allImages, err := osClient.Images().List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allStreams, err := osClient.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allPods, err := kClient.Pods(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allRCs, err := kClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
			cmdutil.CheckErr(err)

			allBCs, err := osClient.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allBuilds, err := osClient.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			allDCs, err := osClient.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			cmdutil.CheckErr(err)

			pruner := prune.NewImagePruner(
				cfg.KeepYoungerThan,
				cfg.TagRevisionsToKeep,
				allImages,
				allStreams,
				allPods,
				allRCs,
				allBCs,
				allBuilds,
				allDCs,
			)

			w := tabwriter.NewWriter(out, 10, 4, 3, ' ', 0)
			defer w.Flush()

			printImageHeader := true
			describingImagePruneFunc := func(image *imageapi.Image, streams []*imageapi.ImageStream) []error {
				if printImageHeader {
					printImageHeader = false
					fmt.Fprintf(w, "IMAGE\tSTREAMS\n")
				}
				streamNames := util.NewStringSet()
				for _, stream := range streams {
					streamNames.Insert(fmt.Sprintf("%s/%s", stream.Namespace, stream.Name))
				}
				fmt.Fprintf(w, "%s\t%s\n", image.Name, strings.Join(streamNames.List(), ", "))
				return nil
			}

			printLayerHeader := true
			describingLayerPruneFunc := func(registryURL, repo, layer string) error {
				if printLayerHeader {
					printLayerHeader = false
					fmt.Fprintf(w, "\nREGISTRY\tSTREAM\tLAYER\n")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", registryURL, repo, layer)
				return nil
			}

			var (
				imagePruneFunc    prune.ImagePruneFunc
				layerPruneFunc    prune.LayerPruneFunc
				blobPruneFunc     prune.BlobPruneFunc
				manifestPruneFunc prune.ManifestPruneFunc
			)

			switch cfg.DryRun {
			case false:
				imagePruneFunc = func(image *imageapi.Image, referencedStreams []*imageapi.ImageStream) []error {
					describingImagePruneFunc(image, referencedStreams)
					return prune.DeletingImagePruneFunc(osClient.Images(), osClient)(image, referencedStreams)
				}
				layerPruneFunc = func(registryURL, repo, layer string) error {
					describingLayerPruneFunc(registryURL, repo, layer)
					return prune.DeletingLayerPruneFunc(http.DefaultClient)(registryURL, repo, layer)
				}
				blobPruneFunc = prune.DeletingBlobPruneFunc(http.DefaultClient)
				manifestPruneFunc = prune.DeletingManifestPruneFunc(http.DefaultClient)
			default:
				fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made.")
				imagePruneFunc = describingImagePruneFunc
				layerPruneFunc = describingLayerPruneFunc
				blobPruneFunc = func(registryURL, blob string) error {
					return nil
				}
				manifestPruneFunc = func(registryURL, repo, manifest string) error {
					return nil
				}
			}

			pruner.Run(imagePruneFunc, layerPruneFunc, blobPruneFunc, manifestPruneFunc)
		},
	}

	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Perform a build pruning dry-run, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().DurationVar(&cfg.KeepYoungerThan, "keep-younger-than", cfg.KeepYoungerThan, "Specify the minimum age of a build for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&cfg.TagRevisionsToKeep, "keep-tag-revisions", cfg.TagRevisionsToKeep, "Specify the number of image revisions for a tag in an image stream that will be preserved.")

	return cmd
}

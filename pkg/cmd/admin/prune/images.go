package prune

import (
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
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

const imagesLongDesc = `%s %s - prunes images`

const PruneImagesRecommendedName = "images"

type pruneImagesConfig struct {
	DryRun             bool
	KeepYoungerThan    time.Duration
	TagRevisionsToKeep int
	CABundle           string
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

			var streams util.StringSet
			printImageHeader := true
			describingImagePruneFunc := func(image *imageapi.Image) error {
				if printImageHeader {
					printImageHeader = false
					fmt.Fprintf(w, "IMAGE\tSTREAMS")
				}

				if streams.Len() > 0 {
					fmt.Fprintf(w, strings.Join(streams.List(), ", "))
				}

				fmt.Fprintf(w, "\n%s\t", image.Name)
				streams = util.NewStringSet()

				return nil
			}

			describingImageStreamPruneFunc := func(stream *imageapi.ImageStream, image *imageapi.Image) (*imageapi.ImageStream, error) {
				streams.Insert(stream.Status.DockerImageRepository)
				return stream, nil
			}

			printLayerHeader := true
			describingLayerPruneFunc := func(registryURL, repo, layer string) error {
				if printLayerHeader {
					printLayerHeader = false
					// need to print the remaining streams for the last image
					if streams.Len() > 0 {
						fmt.Fprintf(w, strings.Join(streams.List(), ", "))
					}
					fmt.Fprintf(w, "\n\nREGISTRY\tSTREAM\tLAYER\n")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", registryURL, repo, layer)
				return nil
			}

			var (
				imagePruneFunc       prune.ImagePruneFunc
				imageStreamPruneFunc prune.ImageStreamPruneFunc
				layerPruneFunc       prune.LayerPruneFunc
				blobPruneFunc        prune.BlobPruneFunc
				manifestPruneFunc    prune.ManifestPruneFunc
			)

			// get the client config so we can get the TLS config
			clientConfig, err := f.OpenShiftClientConfig.ClientConfig()
			cmdutil.CheckErr(err)

			tlsConfig, err := kclient.TLSConfigFor(clientConfig)
			cmdutil.CheckErr(err)

			// if the user specified a CA on the command line, add it to the
			// client config's CA roots
			if len(cfg.CABundle) > 0 {
				data, err := ioutil.ReadFile(cfg.CABundle)
				cmdutil.CheckErr(err)
				if tlsConfig.RootCAs == nil {
					tlsConfig.RootCAs = x509.NewCertPool()
				}
				tlsConfig.RootCAs.AppendCertsFromPEM(data)
			}

			registryClient := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: tlsConfig,
				},
			}

			switch cfg.DryRun {
			case false:
				imagePruneFunc = func(image *imageapi.Image) error {
					describingImagePruneFunc(image)
					return prune.DeletingImagePruneFunc(osClient.Images())(image)
				}
				imageStreamPruneFunc = func(stream *imageapi.ImageStream, image *imageapi.Image) (*imageapi.ImageStream, error) {
					describingImageStreamPruneFunc(stream, image)
					return prune.DeletingImageStreamPruneFunc(osClient)(stream, image)
				}
				layerPruneFunc = func(registryURL, repo, layer string) error {
					describingLayerPruneFunc(registryURL, repo, layer)
					return prune.DeletingLayerPruneFunc(registryClient)(registryURL, repo, layer)
				}
				blobPruneFunc = prune.DeletingBlobPruneFunc(registryClient)
				manifestPruneFunc = prune.DeletingManifestPruneFunc(registryClient)
			default:
				fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made.")
				imagePruneFunc = describingImagePruneFunc
				imageStreamPruneFunc = describingImageStreamPruneFunc
				layerPruneFunc = describingLayerPruneFunc
				blobPruneFunc = func(registryURL, blob string) error {
					return nil
				}
				manifestPruneFunc = func(registryURL, repo, manifest string) error {
					return nil
				}
			}

			pruner.Run(imagePruneFunc, imageStreamPruneFunc, layerPruneFunc, blobPruneFunc, manifestPruneFunc)
		},
	}

	cmd.Flags().BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Perform a build pruning dry-run, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().DurationVar(&cfg.KeepYoungerThan, "keep-younger-than", cfg.KeepYoungerThan, "Specify the minimum age of a build for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&cfg.TagRevisionsToKeep, "keep-tag-revisions", cfg.TagRevisionsToKeep, "Specify the number of image revisions for a tag in an image stream that will be preserved.")
	cmd.Flags().StringVar(&cfg.CABundle, "certificate-authority", cfg.CABundle, "The path to a certificate authority bundle to use when communicating with the OpenShift-managed registries. Defaults to the certificate authority data from the current user's config file.")

	return cmd
}

package top

import (
	"fmt"
	"io"
	"sort"

	units "github.com/docker/go-units"
	gonum "github.com/gonum/graph"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/graph/genericgraph"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
)

const TopImageStreamsRecommendedName = "imagestreams"

var (
	topImageStreamsLong = templates.LongDesc(`
		Show usage statistics for ImageStreams

		This command analyzes all the ImageStreams managed by the platform and presents current
		usage statistics.`)

	topImageStreamsExample = templates.Examples(`
		# Show usage statistics for ImageStreams
  	%[1]s %[2]s`)
)

// NewCmdTopImageStreams implements the OpenShift cli top imagestreams command.
func NewCmdTopImageStreams(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	opts := &TopImageStreamsOptions{}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Show usage statistics for ImageStreams",
		Long:    topImageStreamsLong,
		Example: fmt.Sprintf(topImageStreamsExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}

	return cmd
}

type TopImageStreamsOptions struct {
	// internal values
	Images  *imageapi.ImageList
	Streams *imageapi.ImageStreamList

	// helpers
	out io.Writer
}

// Complete turns a partially defined TopImageStreamsOptions into a solvent structure
// which can be validated and used for showing limits usage.
func (o *TopImageStreamsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	namespace := cmd.Flag("namespace").Value.String()
	if len(namespace) == 0 {
		namespace = metav1.NamespaceAll
	}
	o.out = out
	imageClient, err := f.OpenshiftInternalImageClient()
	if err != nil {
		return err
	}

	allImages, err := imageClient.Image().Images().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	o.Images = allImages

	allStreams, err := imageClient.Image().ImageStreams(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	o.Streams = allStreams

	return nil
}

// Validate ensures that a TopImageStreamsOptions is valid and can be used to execute command.
func (o TopImageStreamsOptions) Validate(cmd *cobra.Command) error {
	return nil
}

// Run contains all the necessary functionality to show current image references.
func (o TopImageStreamsOptions) Run() error {
	infos := o.imageStreamsTop()
	Print(o.out, ImageStreamColumns, infos)
	return nil
}

var ImageStreamColumns = []string{"NAME", "STORAGE", "IMAGES", "LAYERS"}

// imageStreamInfo contains contains statistic information about ImageStream usage.
type imageStreamInfo struct {
	ImageStream string
	Storage     int64
	Images      int
	Layers      int
}

var _ Info = &imageStreamInfo{}

func (i imageStreamInfo) PrintLine(out io.Writer) {
	printValue(out, i.ImageStream)
	printValue(out, units.BytesSize(float64(i.Storage)))
	printValue(out, i.Images)
	printValue(out, i.Layers)
}

// imageStreamsTop generates ImageStream information from a graph and
// returns this as a list of imageStreamInfo array.
func (o TopImageStreamsOptions) imageStreamsTop() []Info {
	g := genericgraph.New()
	addImagesToGraph(g, o.Images)
	addImageStreamsToGraph(g, o.Streams)

	infos := []Info{}
	streamNodes := getImageStreamNodes(g.Nodes())
	for _, sn := range streamNodes {
		storage, images, layers := getImageStreamSize(g, sn)
		infos = append(infos, imageStreamInfo{
			ImageStream: fmt.Sprintf("%s/%s", sn.ImageStream.Namespace, sn.ImageStream.Name),
			Storage:     storage,
			Images:      images,
			Layers:      layers,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		a, b := infos[i].(imageStreamInfo), infos[j].(imageStreamInfo)
		if a.Storage < b.Storage {
			return false
		}
		if a.Storage > b.Storage {
			return true
		}
		return a.Images > b.Images
	})

	return infos
}

func getImageStreamSize(g genericgraph.Graph, node *imagegraph.ImageStreamNode) (int64, int, int) {
	imageEdges := g.OutboundEdges(node, ImageStreamImageEdgeKind)
	storage := int64(0)
	images := len(imageEdges)
	layers := 0
	blobSet := sets.NewString()
	for _, e := range imageEdges {
		imageNode, ok := e.To().(*imagegraph.ImageNode)
		if !ok {
			continue
		}
		image := imageNode.Image
		layers += len(image.DockerImageLayers)
		// we're counting only unique layers per the entire stream
		for _, layer := range image.DockerImageLayers {
			if blobSet.Has(layer.Name) {
				continue
			}
			blobSet.Insert(layer.Name)
			storage += layer.LayerSize
		}
		if len(image.DockerImageConfig) > 0 && !blobSet.Has(image.DockerImageMetadata.ID) {
			blobSet.Insert(image.DockerImageMetadata.ID)
			storage += int64(len(image.DockerImageConfig))
		}
	}

	return storage, images, layers
}

func getImageStreamNodes(nodes []gonum.Node) []*imagegraph.ImageStreamNode {
	ret := []*imagegraph.ImageStreamNode{}
	for i := range nodes {
		if node, ok := nodes[i].(*imagegraph.ImageStreamNode); ok {
			ret = append(ret, node)
		}
	}
	return ret
}

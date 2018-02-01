package top

import (
	"fmt"
	"io"
	"sort"
	"strings"

	units "github.com/docker/go-units"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

const (
	TopImagesRecommendedName = "images"
	maxImageIDLength         = 20
)

var (
	topImagesLong = templates.LongDesc(`
		Show usage statistics for Images

		This command analyzes all the Images managed by the platform and presents current
		usage statistics.`)

	topImagesExample = templates.Examples(`
		# Show usage statistics for Images
  	%[1]s %[2]s`)
)

// NewCmdTopImages implements the OpenShift cli top images command.
func NewCmdTopImages(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	opts := &TopImagesOptions{}
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Show usage statistics for Images",
		Long:    topImagesLong,
		Example: fmt.Sprintf(topImagesExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate(cmd))
			kcmdutil.CheckErr(opts.Run())
		},
	}

	return cmd
}

type TopImagesOptions struct {
	// internal values
	Images  *imageapi.ImageList
	Streams *imageapi.ImageStreamList
	Pods    *kapi.PodList

	// helpers
	out io.Writer
}

// Complete turns a partially defined TopImagesOptions into a solvent structure
// which can be validated and used for showing limits usage.
func (o *TopImagesOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	kClient, err := f.ClientSet()
	if err != nil {
		return err
	}

	imageClient, err := f.OpenshiftInternalImageClient()
	if err != nil {
		return err
	}

	namespace := cmd.Flag("namespace").Value.String()
	if len(namespace) == 0 {
		namespace = metav1.NamespaceAll
	}
	o.out = out

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

	allPods, err := kClient.Core().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	o.Pods = allPods

	return nil
}

// Validate ensures that a TopImagesOptions is valid and can be used to execute command.
func (o TopImagesOptions) Validate(cmd *cobra.Command) error {
	return nil
}

// Run contains all the necessary functionality to show current image references.
func (o TopImagesOptions) Run() error {
	infos := o.imagesTop()
	Print(o.out, ImageColumns, infos)
	return nil
}

var ImageColumns = []string{"NAME", "IMAGESTREAMTAG", "PARENTS", "USAGE", "METADATA", "STORAGE"}

// imageInfo contains statistic information about Image usage.
type imageInfo struct {
	Image           string
	ImageStreamTags []string
	Parents         []string
	Usage           []string
	Metadata        bool
	Storage         int64
}

var _ Info = &imageInfo{}

func (i imageInfo) PrintLine(out io.Writer) {
	printValue(out, i.Image)
	printArray(out, i.ImageStreamTags)
	shortParents := make([]string, len(i.Parents))
	for i, p := range i.Parents {
		if len(p) > maxImageIDLength {
			shortParents[i] = p[:maxImageIDLength-3] + "..."
		} else {
			shortParents[i] = p
		}
	}
	printArray(out, shortParents)
	printArray(out, i.Usage)
	printBool(out, i.Metadata)
	printValue(out, units.BytesSize(float64(i.Storage)))
}

// imagesTop generates Image information from a graph and returns this as a list
// of imageInfo array.
func (o TopImagesOptions) imagesTop() []Info {
	g := genericgraph.New()
	addImagesToGraph(g, o.Images)
	addImageStreamsToGraph(g, o.Streams)
	addPodsToGraph(g, o.Pods)
	markParentsInGraph(g)

	infos := []Info{}
	imageNodes := getImageNodes(g.Nodes())
	for _, in := range imageNodes {
		image := in.Image
		istags := getImageStreamTags(g, in)
		parents := getImageParents(g, in)
		usage := getImageUsage(g, in)
		metadata := len(image.DockerImageManifest) != 0 && len(image.DockerImageLayers) != 0
		storage := getStorage(image)
		infos = append(infos, imageInfo{
			Image:           image.Name,
			ImageStreamTags: istags,
			Parents:         parents,
			Usage:           usage,
			Metadata:        metadata,
			Storage:         storage,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		a, b := infos[i].(imageInfo), infos[j].(imageInfo)
		if len(a.ImageStreamTags) < len(b.ImageStreamTags) {
			return false
		}
		if len(a.ImageStreamTags) > len(b.ImageStreamTags) {
			return true
		}
		return a.Storage > b.Storage
	})

	return infos
}

func getStorage(image *imageapi.Image) int64 {
	storage := int64(0)
	blobSet := sets.NewString()
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
	return storage
}

func getImageStreamTags(g genericgraph.Graph, node *imagegraph.ImageNode) []string {
	istags := []string{}
	for _, e := range g.InboundEdges(node, ImageStreamImageEdgeKind) {
		streamNode, ok := e.From().(*imagegraph.ImageStreamNode)
		if !ok {
			continue
		}
		stream := streamNode.ImageStream
		tags := getTags(stream, node.Image)
		istags = append(istags, fmt.Sprintf("%s/%s (%s)", stream.Namespace, stream.Name, strings.Join(tags, ",")))
	}
	return istags
}

func getTags(stream *imageapi.ImageStream, image *imageapi.Image) []string {
	tags := []string{}
	for tag, history := range stream.Status.Tags {
		if len(history.Items) > 0 && history.Items[0].Image == image.Name {
			tags = append(tags, tag)
		}
	}
	imageapi.PrioritizeTags(tags)
	return tags
}

func getImageParents(g genericgraph.Graph, node *imagegraph.ImageNode) []string {
	parents := []string{}
	for _, e := range g.InboundEdges(node, ParentImageEdgeKind) {
		imageNode, ok := e.From().(*imagegraph.ImageNode)
		if !ok {
			continue
		}
		parents = append(parents, imageNode.Image.Name)
	}
	return parents
}

func getImageUsage(g genericgraph.Graph, node *imagegraph.ImageNode) []string {
	usage := []string{}
	for _, e := range g.InboundEdges(node, PodImageEdgeKind) {
		podNode, ok := e.From().(*kubegraph.PodNode)
		if !ok {
			continue
		}
		usage = append(usage, getController(podNode.Pod))
	}
	return usage
}

func getController(pod *kapi.Pod) string {
	controller := "<unknown>"
	if pod.Annotations == nil {
		return controller
	}

	if bc, ok := pod.Annotations[buildapi.BuildAnnotation]; ok {
		return fmt.Sprintf("Build: %s/%s", pod.Namespace, bc)
	}
	if dc, ok := pod.Annotations[appsapi.DeploymentAnnotation]; ok {
		return fmt.Sprintf("Deployment: %s/%s", pod.Namespace, dc)
	}
	if dc, ok := pod.Annotations[appsapi.DeploymentPodAnnotation]; ok {
		return fmt.Sprintf("Deployer: %s/%s", pod.Namespace, dc)
	}

	return controller
}

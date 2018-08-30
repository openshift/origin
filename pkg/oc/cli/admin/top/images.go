package top

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	appsv1 "github.com/openshift/api/apps/v1"
	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageutil "github.com/openshift/origin/pkg/image/util"
	"github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	imagegraph "github.com/openshift/origin/pkg/oc/lib/graph/imagegraph/nodes"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
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

type TopImagesOptions struct {
	// internal values
	Images  *imagev1.ImageList
	Streams *imagev1.ImageStreamList
	Pods    *corev1.PodList

	genericclioptions.IOStreams
}

func NewTopImagesOptions(streams genericclioptions.IOStreams) *TopImagesOptions {
	return &TopImagesOptions{
		IOStreams: streams,
	}
}

// NewCmdTopImages implements the OpenShift cli top images command.
func NewCmdTopImages(f kcmdutil.Factory, parentName, name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTopImagesOptions(streams)
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Show usage statistics for Images",
		Long:    topImagesLong,
		Example: fmt.Sprintf(topImagesExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate(cmd))
			kcmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

// Complete turns a partially defined TopImagesOptions into a solvent structure
// which can be validated and used for showing limits usage.
func (o *TopImagesOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	kClient, err := corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	imageClient, err := imagev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	namespace := cmd.Flag("namespace").Value.String()
	if len(namespace) == 0 {
		namespace = metav1.NamespaceAll
	}

	allImages, err := imageClient.Images().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	o.Images = allImages

	allStreams, err := imageClient.ImageStreams(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	o.Streams = allStreams

	allPods, err := kClient.Pods(namespace).List(metav1.ListOptions{})
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
	Print(o.Out, ImageColumns, infos)
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

func getStorage(image *imagev1.Image) int64 {
	storage := int64(0)
	blobSet := sets.NewString()
	for _, layer := range image.DockerImageLayers {
		if blobSet.Has(layer.Name) {
			continue
		}
		blobSet.Insert(layer.Name)
		storage += layer.LayerSize
	}
	if err := imageutil.ImageWithMetadata(image); err != nil {
		return storage
	}
	dockerImage, ok := image.DockerImageMetadata.Object.(*dockerv10.DockerImage)
	if !ok {
		return storage
	}
	if len(image.DockerImageConfig) > 0 && !blobSet.Has(dockerImage.ID) {
		blobSet.Insert(dockerImage.ID)
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

func getTags(stream *imagev1.ImageStream, image *imagev1.Image) []string {
	tags := []string{}
	for _, tag := range stream.Status.Tags {
		if len(tag.Items) > 0 && tag.Items[0].Image == image.Name {
			tags = append(tags, tag.Tag)
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

func getController(pod *corev1.Pod) string {
	controller := "<unknown>"
	if pod.Annotations == nil {
		return controller
	}

	if bc, ok := pod.Annotations[buildapi.BuildAnnotation]; ok {
		return fmt.Sprintf("Build: %s/%s", pod.Namespace, bc)
	}
	if dc, ok := pod.Annotations[appsv1.DeploymentAnnotation]; ok {
		return fmt.Sprintf("Deployment: %s/%s", pod.Namespace, dc)
	}
	if dc, ok := pod.Annotations[appsv1.DeploymentPodAnnotation]; ok {
		return fmt.Sprintf("Deployer: %s/%s", pod.Namespace, dc)
	}

	return controller
}

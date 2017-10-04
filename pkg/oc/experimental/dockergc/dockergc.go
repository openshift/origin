package dockergc

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	dockerapi "github.com/docker/engine-api/client"
	dockertypes "github.com/docker/engine-api/types"
	dockerfilters "github.com/docker/engine-api/types/filters"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultImageGCHighThresholdPercent = int32(80)
	DefaultImageGCLowThresholdPercent  = int32(60)
)

var (
	DefaultMinimumGCAge = metav1.Duration{Duration: time.Hour}
)

// DockerGCConfigCmdOptions are options supported by the dockergc admin command.
type dockerGCConfigCmdOptions struct {
	Action configcmd.BulkAction

	// MinimumGCAge is the minimum age for a container or unused image before
	// it is garbage collected.
	MinimumGCAge metav1.Duration
	// ImageGCHighThresholdPercent is the percent of disk usage after which
	// image garbage collection is always run.
	ImageGCHighThresholdPercent int32
	// ImageGCLowThresholdPercent is the percent of disk usage before which
	// image garbage collection is never run. Lowest disk usage to garbage
	// collect to.
	ImageGCLowThresholdPercent int32
}

var (
	dockerGC_long = templates.LongDesc(`
		Perform garbage collection to free space in docker storage

		If the OpenShift node is configured to use a container runtime other than docker,
		docker will still be used to do builds.  However OpenShift itself may not
		manage the docker storage since it is not the container runtime for pods.
		
		This utility allows garbage collection to do be done on the docker storage.`)

	dockerGC_example = templates.Examples(`
	  # Perform garbage collection with the default settings
	  %[1]s %[2]s`)
)

func NewCmdDockerGCConfig(f *clientcmd.Factory, parentName, name string, out, errout io.Writer) *cobra.Command {
	options := &dockerGCConfigCmdOptions{
		Action: configcmd.BulkAction{
			Out:    out,
			ErrOut: errout,
		},
		MinimumGCAge:                DefaultMinimumGCAge,
		ImageGCHighThresholdPercent: DefaultImageGCHighThresholdPercent,
		ImageGCLowThresholdPercent:  DefaultImageGCLowThresholdPercent,
	}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [NAME]", name),
		Short:   "Perform garbage collection to free space in docker storage",
		Long:    dockerGC_long,
		Example: fmt.Sprintf(dockerGC_example, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			err := Run(f, options, cmd, args)
			if err == kcmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().DurationVar(&options.MinimumGCAge.Duration, "minimum-ttl-duration", options.MinimumGCAge.Duration, "Minimum age for a container or unused image before it is garbage collected.  Examples: '300ms', '10s' or '2h45m'.")
	cmd.Flags().Int32Var(&options.ImageGCHighThresholdPercent, "image-gc-high-threshold", options.ImageGCHighThresholdPercent, "The percent of disk usage after which image garbage collection is always run.")
	cmd.Flags().Int32Var(&options.ImageGCLowThresholdPercent, "image-gc-low-threshold", options.ImageGCLowThresholdPercent, "The percent of disk usage before which image garbage collection is never run. Lowest disk usage to garbage collect to.")

	options.Action.BindForOutput(cmd.Flags())

	return cmd
}

// parseInfo parses df output to return capacity and used in bytes
func parseInfo(str string) (int64, int64, error) {
	fields := strings.Fields(str)
	if len(fields) != 4 {
		return 0, 0, fmt.Errorf("unable to parse df output")
	}
	value, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	capacityKBytes := int64(value)
	value, err = strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	usageKBytes := int64(value)
	return capacityKBytes * 1024, usageKBytes * 1024, nil
}

// getRootDirInfo returns the capacity and usage in bytes for the docker root directory
func getRootDirInfo(rootDir string) (int64, int64, error) {
	cmd := exec.Command("df", "-k", "--output=size,used", rootDir)
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	return parseInfo(string(output))
}

func bytesToMB(bytes int64) int64 {
	return bytes / 1024 / 1024
}

type oldestContainersFirst []dockertypes.Container

func (s oldestContainersFirst) Len() int           { return len(s) }
func (s oldestContainersFirst) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s oldestContainersFirst) Less(i, j int) bool { return s[i].Created < s[j].Created }

type oldestImagesFirst []dockertypes.Image

func (s oldestImagesFirst) Len() int           { return len(s) }
func (s oldestImagesFirst) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s oldestImagesFirst) Less(i, j int) bool { return s[i].Created < s[j].Created }

// parseDockerTimestamp parses the timestamp returned by Interface from string to time.Time
func parseDockerTimestamp(s string) (time.Time, error) {
	// Timestamp returned by Docker is in time.RFC3339Nano format.
	return time.Parse(time.RFC3339Nano, s)
}

func doGarbageCollection(ctx context.Context, client *dockerapi.Client, options *dockerGCConfigCmdOptions, rootDir string) error {
	fmt.Println("gathering disk usage data")
	capacityBytes, usageBytes, err := getRootDirInfo(rootDir)
	if err != nil {
		return err
	}

	highThresholdBytes := capacityBytes * int64(options.ImageGCHighThresholdPercent) / 100
	lowThresholdBytes := capacityBytes * int64(options.ImageGCLowThresholdPercent) / 100
	if usageBytes < highThresholdBytes {
		fmt.Printf("usage is under high threshold (%vMB < %vMB)\n", bytesToMB(usageBytes), bytesToMB(highThresholdBytes))
		return nil
	}

	attemptToFreeBytes := usageBytes - lowThresholdBytes
	freedBytes := int64(0)
	fmt.Printf("usage exceeds high threshold (%vMB > %vMB), attempting to free %vMB\n", bytesToMB(usageBytes), bytesToMB(highThresholdBytes), bytesToMB(attemptToFreeBytes))

	// conatiners
	exitedFilter := dockerfilters.NewArgs()
	exitedFilter.Add("status", "exited")
	containers, err := client.ContainerList(ctx, dockertypes.ContainerListOptions{All: true, Filter: exitedFilter})
	if ctx.Err() == context.DeadlineExceeded {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	fmt.Println(len(containers), "exited containers found")
	sort.Sort(oldestContainersFirst(containers))
	for _, c := range containers {
		if freedBytes > attemptToFreeBytes {
			fmt.Printf("usage is below low threshold, freed %vMB\n", bytesToMB(freedBytes))
			return nil
		}
		age := time.Now().Sub(time.Unix(c.Created, 0))
		if age < options.MinimumGCAge.Duration {
			fmt.Println("remaining containers are too young")
			break
		}
		fmt.Printf("removing container %v (size: %v, age: %v)\n", c.ID, c.SizeRw, age)
		err := client.ContainerRemove(ctx, c.ID, dockertypes.ContainerRemoveOptions{RemoveVolumes: true})
		if err != nil {
			fmt.Printf("unable to remove container: %v", err)
		} else {
			freedBytes += c.SizeRw
		}
	}

	// images
	images, err := client.ImageList(ctx, dockertypes.ImageListOptions{})
	if ctx.Err() == context.DeadlineExceeded {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	sort.Sort(oldestImagesFirst(images))
	for _, i := range images {
		if freedBytes > attemptToFreeBytes {
			fmt.Printf("usage is below low threshold, freed %vMB\n", bytesToMB(freedBytes))
			return nil
		}
		// filter openshift infra images
		if strings.HasPrefix(i.RepoTags[0], "registry.ops.openshift.com/openshift3") ||
			strings.HasPrefix(i.RepoTags[0], "docker.io/openshift") {
			fmt.Println("skipping infra image", i.RepoTags[0])
		}
		// filter young images
		age := time.Now().Sub(time.Unix(i.Created, 0))
		if age < options.MinimumGCAge.Duration {
			fmt.Println("remaining images are too young")
			break
		}
		fmt.Printf("removing image %v (size: %v, age: %v)\n", i.ID, i.Size, age)
		_, err := client.ImageRemove(ctx, i.ID, dockertypes.ImageRemoveOptions{PruneChildren: true})
		if err != nil {
			fmt.Printf("unable to remove container: %v", err)
		} else {
			freedBytes += i.Size
		}
	}

	return nil
}

// Run runs the dockergc command.
func Run(f *clientcmd.Factory, options *dockerGCConfigCmdOptions, cmd *cobra.Command, args []string) error {
	fmt.Println("docker build garbage collection daemon")
	fmt.Printf("MinimumGCAge: %v, ImageGCHighThresholdPercent: %v, ImageGCLowThresholdPercent: %v\n", options.MinimumGCAge, options.ImageGCHighThresholdPercent, options.ImageGCLowThresholdPercent)
	client, err := dockerapi.NewEnvClient()
	if err != nil {
		return err
	}
	timeout := time.Duration(2 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	info, err := client.Info(ctx)
	if err != nil {
		return err
	}
	rootDir := info.DockerRootDir
	if rootDir == "" {
		return fmt.Errorf("unable to determine docker root directory")
	}

	for {
		err := doGarbageCollection(ctx, client, options, rootDir)
		if err != nil {
			return err
		}
		<-time.After(time.Minute)
		return nil
	}
}

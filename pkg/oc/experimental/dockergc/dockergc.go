package dockergc

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	dockertypes "github.com/docker/docker/api/types"
	dockerfilters "github.com/docker/docker/api/types/filters"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultImageGCHighThresholdPercent = int32(80)
	DefaultImageGCLowThresholdPercent  = int32(60)
)

var (
	DefaultMinimumGCAge = metav1.Duration{Duration: time.Hour}

	dockerTimeout = time.Duration(2 * time.Minute)
)

// DockerGCConfigCmdOptions are options supported by the dockergc admin command.
type dockerGCConfigCmdOptions struct {
	// DryRun is true if the command was invoked with --dry-run=true
	DryRun bool
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

		This utility allows garbage collection to do be done on the docker storage.

		Only the overlay2 docker storage driver is supported at this time.`)

	dockerGC_example = templates.Examples(`
	  # Perform garbage collection with the default settings
	  %[1]s %[2]s`)
)

func NewCmdDockerGCConfig(f *clientcmd.Factory, parentName, name string, out, errout io.Writer) *cobra.Command {
	options := &dockerGCConfigCmdOptions{
		DryRun:                      false,
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
	cmd.Flags().BoolVar(&options.DryRun, "dry-run", options.DryRun, "Run in single-pass mode with no effect.")

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

type oldestImagesFirst []dockertypes.ImageSummary

func (s oldestImagesFirst) Len() int           { return len(s) }
func (s oldestImagesFirst) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s oldestImagesFirst) Less(i, j int) bool { return s[i].Created < s[j].Created }

// parseDockerTimestamp parses the timestamp returned by Interface from string to time.Time
func parseDockerTimestamp(s string) (time.Time, error) {
	// Timestamp returned by Docker is in time.RFC3339Nano format.
	return time.Parse(time.RFC3339Nano, s)
}

func doGarbageCollection(client *dockerClient, options *dockerGCConfigCmdOptions, rootDir string) error {
	glog.Infof("gathering disk usage data")
	capacityBytes, usageBytes, err := getRootDirInfo(rootDir)
	if err != nil {
		return err
	}

	highThresholdBytes := capacityBytes * int64(options.ImageGCHighThresholdPercent) / 100
	lowThresholdBytes := capacityBytes * int64(options.ImageGCLowThresholdPercent) / 100
	if usageBytes < highThresholdBytes {
		glog.Infof("usage is under high threshold (%vMB < %vMB)", bytesToMB(usageBytes), bytesToMB(highThresholdBytes))
		return nil
	}

	attemptToFreeBytes := usageBytes - lowThresholdBytes
	freedBytes := int64(0)
	glog.Infof("usage exceeds high threshold (%vMB > %vMB), attempting to free %vMB", bytesToMB(usageBytes), bytesToMB(highThresholdBytes), bytesToMB(attemptToFreeBytes))

	// conatiners
	exitedFilter := dockerfilters.NewArgs()
	exitedFilter.Add("status", "exited")
	containers, err := client.ContainerList(dockertypes.ContainerListOptions{All: true, Filters: exitedFilter})
	if err != nil {
		return err
	}
	glog.Infof("%d exited containers found", len(containers))
	sort.Sort(oldestContainersFirst(containers))
	for _, c := range containers {
		if freedBytes > attemptToFreeBytes {
			glog.Infof("usage is below low threshold, freed %vMB", bytesToMB(freedBytes))
			return nil
		}
		age := time.Now().Sub(time.Unix(c.Created, 0))
		if age < options.MinimumGCAge.Duration {
			glog.Infof("remaining containers are too young")
			break
		}
		glog.Infof("removing container %v (size: %v, age: %v)", c.ID, c.SizeRw, age)
		var err error
		if !options.DryRun {
			err = client.ContainerRemove(c.ID, dockertypes.ContainerRemoveOptions{RemoveVolumes: true})
		}
		if err != nil {
			glog.Infof("unable to remove container: %v", err)
		} else {
			freedBytes += c.SizeRw
		}
	}

	// images
	images, err := client.ImageList(dockertypes.ImageListOptions{})
	if err != nil {
		return err
	}
	sort.Sort(oldestImagesFirst(images))
	glog.Infof("%d images found", len(images))
	for _, i := range images {
		if freedBytes > attemptToFreeBytes {
			glog.Infof("usage is below low threshold, freed %vMB", bytesToMB(freedBytes))
			return nil
		}
		// filter openshift infra images
		if len(i.RepoTags) > 0 {
			if strings.HasPrefix(i.RepoTags[0], "registry.ops.openshift.com/openshift3") ||
				strings.HasPrefix(i.RepoTags[0], "docker.io/openshift") {
				glog.Infof("skipping infra image: %v", i.RepoTags[0])
				continue
			}
		}
		// filter young images
		age := time.Now().Sub(time.Unix(i.Created, 0))
		if age < options.MinimumGCAge.Duration {
			glog.Infof("remaining images are too young")
			break
		}
		glog.Infof("removing image %v (size: %v, age: %v)", i.ID, i.Size, age)
		var err error
		if !options.DryRun {
			err = client.ImageRemove(i.ID, dockertypes.ImageRemoveOptions{PruneChildren: true})
		}
		if err != nil {
			glog.Infof("unable to remove image: %v", err)
		} else {
			freedBytes += i.Size
		}
	}
	glog.Infof("unable to get below low threshold, %vMB freed", bytesToMB(freedBytes))

	return nil
}

// Run runs the dockergc command.
func Run(f *clientcmd.Factory, options *dockerGCConfigCmdOptions, cmd *cobra.Command, args []string) error {
	glog.Infof("docker build garbage collection daemon")
	if options.DryRun {
		glog.Infof("Running in dry-run mode")
	}
	glog.Infof("MinimumGCAge: %v, ImageGCHighThresholdPercent: %v, ImageGCLowThresholdPercent: %v", options.MinimumGCAge, options.ImageGCHighThresholdPercent, options.ImageGCLowThresholdPercent)
	client, err := newDockerClient(dockerTimeout)
	if err != nil {
		return err
	}

	info, err := client.Info()
	if err != nil {
		return err
	}
	if info.Driver != "overlay2" {
		return fmt.Errorf("%s storage driver is not supported", info.Driver)
	}
	rootDir := info.DockerRootDir
	if rootDir == "" {
		return fmt.Errorf("unable to determine docker root directory")
	}

	for {
		err := doGarbageCollection(client, options, rootDir)
		if err != nil {
			glog.Errorf("garbage collection attempt failed: %v", err)
		}
		if options.DryRun {
			return nil
		}
		<-time.After(time.Minute)
	}
}

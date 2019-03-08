package prune

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
)

type PruneOptions struct {
	MaxEligibleRevision int
	ProtectedRevisions  []int

	ResourceDir   string
	StaticPodName string
}

func NewPruneOptions() *PruneOptions {
	return &PruneOptions{}
}

func NewPrune() *cobra.Command {
	o := NewPruneOptions()

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune static pod installer revisions",
		Run: func(cmd *cobra.Command, args []string) {
			glog.V(1).Info(cmd.Flags())
			glog.V(1).Info(spew.Sdump(o))

			if err := o.Validate(); err != nil {
				glog.Fatal(err)
			}
			if err := o.Run(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	o.AddFlags(cmd.Flags())

	return cmd
}

func (o *PruneOptions) AddFlags(fs *pflag.FlagSet) {
	fs.IntVar(&o.MaxEligibleRevision, "max-eligible-revision", o.MaxEligibleRevision, "highest revision ID to be eligible for pruning")
	fs.IntSliceVar(&o.ProtectedRevisions, "protected-revisions", o.ProtectedRevisions, "list of revision IDs to preserve (not delete)")
	fs.StringVar(&o.ResourceDir, "resource-dir", o.ResourceDir, "directory for all files supporting the static pod manifest")
	fs.StringVar(&o.StaticPodName, "static-pod-name", o.StaticPodName, "name of the static pod")
}

func (o *PruneOptions) Validate() error {
	if len(o.ResourceDir) == 0 {
		return fmt.Errorf("--resource-dir is required")
	}
	if o.MaxEligibleRevision == 0 {
		return fmt.Errorf("--max-eligible-id is required")
	}
	if len(o.StaticPodName) == 0 {
		return fmt.Errorf("--static-pod-name is required")
	}

	return nil
}

func (o *PruneOptions) Run() error {
	protectedIDs := sets.NewInt(o.ProtectedRevisions...)

	files, err := ioutil.ReadDir(o.ResourceDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		// If the file is not a resource directory...
		if !file.IsDir() {
			continue
		}
		// And doesn't match our static pod prefix...
		if !strings.HasPrefix(file.Name(), o.StaticPodName) {
			continue
		}

		// Split file name to get just the integer revision ID
		fileSplit := strings.Split(file.Name(), o.StaticPodName+"-")
		revisionID, err := strconv.Atoi(fileSplit[len(fileSplit)-1])
		if err != nil {
			return err
		}

		// And is not protected...
		if protected := protectedIDs.Has(revisionID); protected {
			continue
		}
		// And is less than or equal to the maxEligibleRevisionID
		if revisionID > o.MaxEligibleRevision {
			continue
		}

		err = os.RemoveAll(path.Join(o.ResourceDir, file.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

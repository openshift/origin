package recycle

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/templates"
)

var (
	recyclerLong = templates.LongDesc(`
		Recycle a volume

		This command will recycle a single volume provided as an argument.`)
)

// NewCommandRecycle provides a CLI handler for recycling volumes
func NewCommandRecycle(name string, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s DIRNAME", name),
		Short: "Recycle a directory",
		Long:  recyclerLong,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, "a directory to recycle is required as the only argument"))
			}
			if err := Recycle(args[0]); err != nil {
				kcmdutil.CheckErr(fmt.Errorf("recycle failed: %v", err))
			}
			fmt.Fprintln(out, "Scrub ok")
		},
	}
	return cmd
}

// Recycle recursively deletes files and folders within the given path. It does not delete the path itself.
func Recycle(dir string) error {
	return newWalker(func(path string, info os.FileInfo) error {
		// Leave the root dir alone
		if path == dir {
			return nil
		}

		// Delete all subfiles/subdirs
		return os.Remove(path)
	}).Walk(dir)
}

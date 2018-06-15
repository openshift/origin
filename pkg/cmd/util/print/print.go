package print

import (
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// VersionedPrintObject handles printing an object in the appropriate version by looking at 'output-version'
// on the command
func VersionedPrintObject(scheme *runtime.Scheme, fn func(*cobra.Command, runtime.Object, io.Writer) error, c *cobra.Command, out io.Writer) func(runtime.Object) error {
	return func(obj runtime.Object) error {
		// TODO: fold into the core printer functionality (preferred output version)

		if items, err := meta.ExtractList(obj); err == nil {
			items, err = convertItemsForDisplayFromDefaultCommand(scheme, c, items)
			if err != nil {
				return err
			}
			if err := meta.SetList(obj, items); err != nil {
				return err
			}
		} else {
			result, err := convertItemsForDisplayFromDefaultCommand(scheme, c, []runtime.Object{obj})
			if err != nil {
				return err
			}
			obj = result[0]
		}
		return fn(c, obj, out)
	}
}

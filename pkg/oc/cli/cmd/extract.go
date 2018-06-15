package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	extractLong = templates.LongDesc(`
		Extract files out of secrets and config maps

		The extract command makes it easy to download the contents of a config map or secret into a directory.
		Each key in the config map or secret is created as a separate file with the name of the key, as it
		is when you mount a secret or config map into a container.

		You may extract the contents of a secret or config map to standard out by passing '-' to --to. The
		names of each key will be written to stdandard error.

		You can limit which keys are extracted with the --keys=NAME flag, or set the directory to extract to
		with --to=DIRECTORY.`)

	extractExample = templates.Examples(`
		# extract the secret "test" to the current directory
	  %[1]s extract secret/test

	  # extract the config map "nginx" to the /tmp directory
	  %[1]s extract configmap/nginx --to=/tmp

		# extract the config map "nginx" to STDOUT
	  %[1]s extract configmap/nginx --to=-

	  # extract only the key "nginx.conf" from config map "nginx" to the /tmp directory
	  %[1]s extract configmap/nginx --to=/tmp --keys=nginx.conf`)
)

type ExtractOptions struct {
	Out, Err        io.Writer
	Filenames       []string
	OnlyKeys        []string
	TargetDirectory string
	Overwrite       bool

	VisitorFn             func(resource.VisitorFunc) error
	ExtractFileContentsFn func(runtime.Object) (map[string][]byte, bool, error)
}

func NewCmdExtract(fullName string, f *clientcmd.Factory, in io.Reader, out, errOut io.Writer) *cobra.Command {
	options := &ExtractOptions{
		Out: out,
		Err: errOut,

		TargetDirectory: ".",
	}
	cmd := &cobra.Command{
		Use:     "extract RESOURCE/NAME [--to=DIRECTORY] [--keys=KEY ...]",
		Short:   "Extract secrets or config maps to disk",
		Long:    extractLong,
		Example: fmt.Sprintf(extractExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, in, out, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			// TODO: move me to kcmdutil
			err := options.Run()
			if err == kcmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().BoolVar(&options.Overwrite, "confirm", options.Overwrite, "If true, overwrite files that already exist.")
	cmd.Flags().StringVar(&options.TargetDirectory, "to", options.TargetDirectory, "Directory to extract files to.")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to identify to extract the resource.")
	cmd.MarkFlagFilename("filename")
	cmd.Flags().StringSliceVar(&options.OnlyKeys, "keys", options.OnlyKeys, "An optional list of keys to extract (default is all keys).")
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *ExtractOptions) Complete(f *clientcmd.Factory, in io.Reader, out io.Writer, cmd *cobra.Command, args []string) error {
	o.ExtractFileContentsFn = f.ExtractFileContents

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	b := f.NewBuilder().
		Internal().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		ResourceNames("", args...).
		ContinueOnError().
		Flatten()

	o.VisitorFn = b.Do().Visit
	return nil
}

func (o *ExtractOptions) Validate() error {
	if o.TargetDirectory != "-" {
		// determine if output location is valid before continuing
		if _, err := os.Stat(o.TargetDirectory); err != nil {
			return err
		}
	}
	return nil
}

func name(info *resource.Info) string {
	return fmt.Sprintf("%s/%s", info.Mapping.Resource, info.Name)
}

func (o *ExtractOptions) Run() error {
	count := 0
	contains := sets.NewString(o.OnlyKeys...)
	err := o.VisitorFn(func(info *resource.Info, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %v", name(info), err)
		}
		contents, ok, err := o.ExtractFileContentsFn(info.Object)
		if err != nil {
			return fmt.Errorf("%s: %v", name(info), err)
		}
		if !ok {
			fmt.Fprintf(o.Err, "warning: %s does not support extraction\n", name(info))
			return nil
		}
		count++
		var errs []error
		for k, v := range contents {
			if contains.Len() == 0 || contains.Has(k) {
				switch {
				case o.TargetDirectory == "-":
					fmt.Fprintf(o.Err, "# %s\n", k)
					o.Out.Write(v)
					if !bytes.HasSuffix(v, []byte("\n")) {
						fmt.Fprintln(o.Out)
					}
				default:
					target := filepath.Join(o.TargetDirectory, k)
					if err := writeToDisk(target, v, o.Overwrite, o.Out); err != nil {
						if os.IsExist(err) {
							err = fmt.Errorf("file exists, pass --confirm to overwrite")
						}
						errs = append(errs, fmt.Errorf("%s: %v", k, err))
					}
				}
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf(kcmdutil.MultipleErrors("error: ", errs))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("you must specify at least one resource to extract")
	}
	return nil
}

func writeToDisk(path string, data []byte, overwrite bool, out io.Writer) error {
	if overwrite {
		if err := ioutil.WriteFile(path, data, 0600); err != nil {
			return err
		}
	} else {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, bytes.NewBuffer(data)); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "%s\n", path)
	return nil
}

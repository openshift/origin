package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/editor"
	fileutil "github.com/openshift/origin/pkg/util/file"
	"github.com/openshift/origin/pkg/util/jsonmerge"
)

// EditOptions is a struct that contains all variables needed for cli edit command.
type EditOptions struct {
	windowsLineEndings bool

	out       io.Writer
	printer   kubectl.ResourcePrinter
	namespace string
	rmap      *resource.Mapper
	args      []string
	builder   *resource.Builder

	ext       string
	filenames []string
	version   string
	fullName  string
}

const (
	editLong = `
Edit a resource from the default editor

The edit command allows you to directly edit any API resource you can retrieve via the
command line tools. It will open the editor defined by your OC_EDITOR, GIT_EDITOR,
or EDITOR environment variables, or fall back to 'vi' for Linux or 'notepad' for Windows.
You can edit multiple objects, although changes are applied one at a time. The command
accepts filenames as well as command line arguments, although the files you point to must
be previously saved versions of resources.

The files to edit will be output in the default API version, or a version specified
by --output-version. The default format is YAML - if you would like to edit in JSON
pass -o json. The flag --windows-line-endings can be used to force Windows line endings,
otherwise the default for your operating system will be used.

In the event an error occurs while updating, a temporary file will be created on disk
that contains your unapplied changes. The most common error when updating a resource
is another editor changing the resource on the server. When this occurs, you will have
to apply your changes to the newer version of the resource, or update your temporary
saved copy to include the latest resource version.`

	editExample = `  # Edit the service named 'docker-registry':
  $ %[1]s edit svc/docker-registry

  # Edit the DeploymentConfig named 'my-deployment':
  $ %[1]s edit dc/my-deployment

  # Use an alternative editor
  $ OC_EDITOR="nano" %[1]s edit dc/my-deployment

  # Edit the service 'docker-registry' in JSON using the v1beta3 API format:
  $ %[1]s edit svc/docker-registry --output-version=v1beta3 -o json`
)

// NewCmdEdit implements the OpenShift cli edit command.
func NewCmdEdit(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	var options EditOptions
	cmd := &cobra.Command{
		Use:     "edit (RESOURCE/NAME | -f FILENAME)",
		Short:   "Edit a resource on the server",
		Long:    editLong,
		Example: fmt.Sprintf(editExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(fullName, f, out, cmd, args); err != nil {
				cmdutil.CheckErr(err)
			}
			err := options.RunEdit()
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}
	usage := "Filename, directory, or URL to file to use to edit the resource"
	kubectl.AddJsonFilenameFlag(cmd, &options.filenames, usage)
	cmd.Flags().StringP("output", "o", "yaml", "Output format. One of: yaml|json.")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().BoolVar(&options.windowsLineEndings, "windows-line-endings", runtime.GOOS == "windows", "Use Windows line-endings (default Unix line-endings)")

	return cmd
}

// Complete completes struct variables.
func (o *EditOptions) Complete(fullName string, f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	o.fullName = fullName
	o.args = args
	o.out = out
	switch format := cmdutil.GetFlagString(cmd, "output"); format {
	case "json":
		o.printer = &kubectl.JSONPrinter{}
		o.ext = ".json"
	case "yaml":
		o.printer = &kubectl.YAMLPrinter{}
		o.ext = ".yaml"
	default:
		return cmdutil.UsageError(cmd, "The flag 'output' must be one of yaml|json")
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.namespace = cmdNamespace

	mapper, typer := f.Object()
	o.rmap = &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: f.ClientMapperForCommand(),
	}

	o.builder = resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		NamespaceParam(o.namespace).DefaultNamespace().
		FilenameParam(explicit, o.filenames...).
		// SelectorParam(selector).
		ResourceTypeOrNameArgs(true, o.args...).
		Latest()

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	o.version = cmdutil.OutputVersion(cmd, clientConfig.Version)
	return nil
}

// RunEdit contains all the necessary functionality for the OpenShift cli edit command.
func (o *EditOptions) RunEdit() error {
	r := o.builder.Flatten().Do()
	results := editResults{}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	for {
		obj, err := resource.AsVersionedObject(infos, false, o.version)
		if err != nil {
			return preservedFile(err, results.file, o.out)
		}

		// TODO: add an annotating YAML printer that can print inline comments on each field,
		//   including descriptions or validation errors

		// generate the file to edit
		buf := &bytes.Buffer{}
		var w io.Writer = buf
		if o.windowsLineEndings {
			w = fileutil.NewCRLFWriter(w)
		}
		if _, err := results.header.WriteTo(w); err != nil {
			return preservedFile(err, results.file, o.out)
		}
		if err := o.printer.PrintObj(obj, w); err != nil {
			return preservedFile(err, results.file, o.out)
		}

		original := buf.Bytes()

		// launch the editor
		edit := editor.NewDefaultEditor()
		edited, file, err := edit.LaunchTempFile("oc-edit-", o.ext, buf)
		if err != nil {
			return preservedFile(err, results.file, o.out)
		}

		// cleanup any file from the previous pass
		if len(results.file) > 0 {
			os.Remove(results.file)
		}

		glog.V(4).Infof("User edited:\n%s", string(edited))
		lines, err := hasLines(bytes.NewBuffer(edited))
		if err != nil {
			return preservedFile(err, file, o.out)
		}
		if bytes.Equal(original, edited) {
			if len(results.edit) > 0 {
				preservedFile(nil, file, o.out)
			} else {
				os.Remove(file)
			}
			fmt.Fprintln(o.out, "Edit cancelled, no changes made.")
			return nil
		}
		if !lines {
			if len(results.edit) > 0 {
				preservedFile(nil, file, o.out)
			} else {
				os.Remove(file)
			}
			fmt.Fprintln(o.out, "Edit cancelled, saved file was empty.")
			return nil
		}

		results = editResults{
			file: file,
		}

		// parse the edited file
		updates, err := o.rmap.InfoForData(edited, "edited-file")
		if err != nil {
			results.header.reasons = append(results.header.reasons, editReason{
				head: fmt.Sprintf("The edited file had a syntax error: %v", err),
			})
			continue
		}

		visitor := resource.NewFlattenListVisitor(updates, o.rmap)

		// need to make sure the original namespace wasn't changed while editing
		if err = visitor.Visit(resource.RequireNamespace(o.namespace)); err != nil {
			return preservedFile(err, file, o.out)
		}

		// attempt to calculate a delta for merging conflicts
		delta, err := jsonmerge.NewDelta(original, edited)
		if err != nil {
			glog.V(4).Infof("Unable to calculate diff, no merge is possible: %v", err)
			delta = nil
		} else {
			delta.AddPreconditions(jsonmerge.RequireKeyUnchanged("apiVersion"))
			results.delta = delta
			results.version = o.version
		}

		err = visitor.Visit(func(info *resource.Info, err error) error {
			if err != nil {
				return err
			}
			updated, err := resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, false, info.Object)
			if err != nil {
				fmt.Fprintln(o.out, results.AddError(err, info))
				return nil
			}
			info.Refresh(updated, true)
			fmt.Fprintf(o.out, "%s/%s\n", info.Mapping.Resource, info.Name)
			return nil
		})
		if err != nil {
			return preservedFile(err, file, o.out)
		}

		if results.retryable > 0 {
			fmt.Fprintf(o.out, "You can run `%s update -f %s` to try this update again.\n", o.fullName, file)
			return errExit
		}
		if results.conflict > 0 {
			fmt.Fprintf(o.out, "You must update your local resource version and run `%s update -f %s` to overwrite the remote changes.\n", o.fullName, file)
			return errExit
		}
		if len(results.edit) == 0 {
			if results.notfound == 0 {
				os.Remove(file)
			} else {
				fmt.Fprintf(o.out, "The edits you made on deleted resources have been saved to %q\n", file)
			}
			return nil
		}

		// loop again and edit the remaining items
		infos = results.edit
	}
}

// editReason preserves a message about the reason this file must be edited again
type editReason struct {
	head  string
	other []string
}

// editHeader includes a list of reasons the edit must be retried
type editHeader struct {
	reasons []editReason
}

// WriteTo outputs the current header information into a stream
func (h *editHeader) WriteTo(w io.Writer) (int64, error) {
	var buffer bytes.Buffer
	buffer.WriteString(`# Please edit the object below. Lines beginning with a '#' will be ignored,
# and an empty file will abort the edit. If an error occurs while saving this file will be
# reopened with the relevant failures.
#
`)
	for _, r := range h.reasons {
		if len(r.other) > 0 {
			buffer.WriteString(fmt.Sprintf("# %s:\n", r.head))
		} else {
			buffer.WriteString(fmt.Sprintf("# %s\n", r.head))
		}

		for _, o := range r.other {
			buffer.WriteString(fmt.Sprintf("# * %s\n", o))
		}
		buffer.WriteString("#")
	}
	fmt.Fprintln(w, buffer.String())
	return int64(buffer.Len()), nil
}

// editResults capture the result of an update
type editResults struct {
	header    editHeader
	retryable int
	notfound  int
	conflict  int
	edit      []*resource.Info
	file      string

	delta   *jsonmerge.Delta
	version string
}

func (r *editResults) AddError(err error, info *resource.Info) string {
	switch {
	case errors.IsInvalid(err):
		r.edit = append(r.edit, info)
		reason := editReason{
			head: fmt.Sprintf("%s %s was not valid", info.Mapping.Kind, info.Name),
		}
		if err, ok := err.(kclient.APIStatus); ok {
			if details := err.Status().Details; details != nil {
				for _, cause := range details.Causes {
					reason.other = append(reason.other, cause.Message)
				}
			}
		}
		r.header.reasons = append(r.header.reasons, reason)
		return fmt.Sprintf("Error: the %s %s is invalid", info.Mapping.Kind, info.Name)
	case errors.IsNotFound(err):
		r.notfound++
		return fmt.Sprintf("Error: the %s %s has been deleted on the server", info.Mapping.Kind, info.Name)

	case errors.IsConflict(err):
		if r.delta != nil {
			v1 := info.ResourceVersion
			if perr := applyPatch(r.delta, info, r.version); perr != nil {
				// the error was related to the patching process
				if nerr, ok := perr.(patchError); ok {
					r.conflict++
					if jsonmerge.IsPreconditionFailed(nerr.error) {
						return fmt.Sprintf("Error: the API version of the provided object cannot be changed")
					}
					// the patch is in conflict, report to user and exit
					if jsonmerge.IsConflicting(nerr.error) {
						// TODO: read message
						return fmt.Sprintf("Error: a conflicting change was made to the %s %s on the server", info.Mapping.Kind, info.Name)
					}
					glog.V(4).Infof("Attempted to patch the resource, but failed: %v", perr)
					return fmt.Sprintf("Error: %v", err)
				}
				// try processing this server error and unset delta so we don't recurse
				r.delta = nil
				return r.AddError(err, info)
			}
			return fmt.Sprintf("Applied your changes to %s from version %s onto %s", info.Name, v1, info.ResourceVersion)
		}
		// no delta was available
		r.conflict++
		return fmt.Sprintf("Error: %v", err)
	default:
		r.retryable++
		return fmt.Sprintf("Error: the %s %s could not be updated: %v", info.Mapping.Kind, info.Name, err)
	}
}

type patchError struct {
	error
}

// applyPatch reads the latest version of the object, writes it to version, then attempts to merge
// the changes onto it without conflict. If a conflict occurs jsonmerge.IsConflicting(err) is
// true. The info object is mutated
func applyPatch(delta *jsonmerge.Delta, info *resource.Info, version string) error {
	if err := info.Get(); err != nil {
		return patchError{err}
	}
	obj, err := resource.AsVersionedObject([]*resource.Info{info}, false, version)
	if err != nil {
		return patchError{err}
	}
	data, err := info.Mapping.Codec.Encode(obj)
	if err != nil {
		return patchError{err}
	}
	merged, err := delta.Apply(data)
	if err != nil {
		return patchError{err}
	}
	mergedObj, err := info.Mapping.Codec.Decode(merged)
	if err != nil {
		return patchError{err}
	}
	updated, err := resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, false, mergedObj)
	if err != nil {
		return err
	}
	info.Refresh(updated, true)
	return nil
}

// preservedFile writes out a message about the provided file if it exists to the
// provided output stream when an error happens. Used to notify the user where
// their updates were preserved.
func preservedFile(err error, path string, out io.Writer) error {
	if len(path) > 0 {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			fmt.Fprintf(out, "A copy of your changes has been stored to %q\n", path)
		}
	}
	return err
}

// hasLines returns true if any line in the provided stream is non empty - has non-whitespace
// characters, or the first non-whitespace character is a '#' indicating a comment. Returns
// any errors encountered reading the stream.
func hasLines(r io.Reader) (bool, error) {
	// TODO: if any files we read have > 64KB lines, we'll need to switch to bytes.ReadLine
	// TODO: probably going to be secrets
	s := bufio.NewScanner(r)
	for s.Scan() {
		if line := strings.TrimSpace(s.Text()); len(line) > 0 && line[0] != '#' {
			return true, nil
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return false, err
	}
	return false, nil
}

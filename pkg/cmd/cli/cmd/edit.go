package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/editor"
)

const (
	edit_long = `Edit a resource from the default editor.

The edit command allows you to directly edit any API resource you can retrieve via the
command line tools. It will open the editor defined by your OSC_EDITOR, GIT_EDITOR,
or EDITOR environment variables, or fall back to 'vi'. You can edit multiple objects,
although changes are applied one at a time. The command accepts filenames as well as
command line arguments, although the files you point to must be previously saved
versions of resources.

The files to edit will be output in the default API version, or a version specified
by --output-version. The default format is YAML - if you would like to edit in JSON
pass -o json.

In the event an error occurs while updating, a temporary file will be created on disk
that contains your unapplied changes. The most common error when updating a resource
is another editor changing the resource on the server. When this occurs, you will have
to apply your changes to the newer version of the resource, or update your temporary
saved copy to include the latest resource version.

Examples:

	# Edit the service named 'docker-registry':
	$ %[1]s edit svc/docker-registry

	# Edit the deployment config named 'my-deployment':
	$ %[1]s edit dc/my-deployment

	# Use an alternative editor
	$ OSC_EDITOR="nano" %[1]s edit dc/my-deployment

	# Edit the service 'docker-registry' in JSON using the v1beta3 API format:
	$ %[1]s edit svc/docker-registry --output-version=v1beta3 -o json
`
)

func NewCmdEdit(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	var filenames util.StringList
	cmd := &cobra.Command{
		Use:   "edit -f FILENAME",
		Short: "Edit a resource on the server and apply the update.",
		Long:  fmt.Sprintf(edit_long, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunEdit(fullName, f, out, cmd, args, filenames)
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringP("output", "o", "yaml", "Output format. One of: yaml|json.")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().VarP(&filenames, "filename", "f", "Filename, directory, or URL to file to use to edit the resource.")
	return cmd
}

func RunEdit(fullName string, f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string, filenames util.StringList) error {
	var printer kubectl.ResourcePrinter
	var ext string
	switch format := cmdutil.GetFlagString(cmd, "output"); format {
	case "json":
		printer = &kubectl.JSONPrinter{}
		ext = ".json"
	case "yaml":
		printer = &kubectl.YAMLPrinter{}
		ext = ".yaml"
	default:
		return cmdutil.UsageError(cmd, "The flag 'output' must be one of yaml|json")
	}

	cmdNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	rmap := &resource.Mapper{
		ObjectTyper:  typer,
		RESTMapper:   mapper,
		ClientMapper: f.ClientMapperForCommand(),
	}

	b := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(filenames...).
		//SelectorParam(selector).
		ResourceTypeOrNameArgs(true, args...).
		Latest()
	if err != nil {
		return err
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	r := b.Flatten().Do()
	infos, err := r.Infos()
	if err != nil {
		return err
	}

	defaultVersion := cmdutil.OutputVersion(cmd, clientConfig.Version)
	results := editResults{}
	for {
		obj, err := resource.AsVersionedObject(infos, defaultVersion)
		if err != nil {
			return preservedFile(err, results.file, cmd.Out())
		}

		// TODO: add an annotating YAML printer that can print inline comments on each field,
		//   including descriptions or validation errors

		// generate the file to edit
		buf := &bytes.Buffer{}
		if err := results.header.WriteTo(buf); err != nil {
			return preservedFile(err, results.file, cmd.Out())
		}
		if err := printer.PrintObj(obj, buf); err != nil {
			return preservedFile(err, results.file, cmd.Out())
		}
		original := buf.Bytes()

		// launch the editor
		edit := editor.NewDefaultEditor()
		edited, file, err := edit.LaunchTempFile("osc-edit-", ext, buf)
		if err != nil {
			return preservedFile(err, results.file, cmd.Out())
		}

		// cleanup any file from the previous pass
		if len(results.file) > 0 {
			os.Remove(results.file)
		}

		glog.V(4).Infof("User edited:\n%s", string(edited))
		lines, err := hasLines(bytes.NewBuffer(edited))
		if err != nil {
			return preservedFile(err, file, cmd.Out())
		}
		if bytes.Equal(original, edited) {
			if len(results.edit) > 0 {
				preservedFile(nil, file, cmd.Out())
			} else {
				os.Remove(file)
			}
			fmt.Fprintln(cmd.Out(), "Edit cancelled, no changes made.")
			return nil
		}
		if !lines {
			if len(results.edit) > 0 {
				preservedFile(nil, file, cmd.Out())
			} else {
				os.Remove(file)
			}
			fmt.Fprintln(cmd.Out(), "Edit cancelled, saved file was empty.")
			return nil
		}

		results = editResults{
			file: file,
		}

		// parse the edited file
		updates, err := rmap.InfoForData(edited, "edited-file")
		if err != nil {
			results.header.reasons = append(results.header.reasons, editReason{
				head: fmt.Sprintf("The edited file had a syntax error: %v", err),
			})
			continue
		}

		err = resource.NewFlattenListVisitor(updates, rmap).Visit(func(info *resource.Info) error {
			data, err := info.Mapping.Codec.Encode(info.Object)
			if err != nil {
				return err
			}
			updated, err := resource.NewHelper(info.Client, info.Mapping).Update(info.Namespace, info.Name, false, data)
			if err != nil {
				fmt.Fprintln(cmd.Out(), results.AddError(err, info))
				return nil
			}
			info.Refresh(updated, true)
			fmt.Fprintf(out, "%s/%s\n", info.Mapping.Resource, info.Name)
			return nil
		})
		if err != nil {
			return preservedFile(err, file, cmd.Out())
		}

		if results.retryable > 0 {
			fmt.Fprintf(cmd.Out(), "You can run `%s update -f %s` to try this update again.\n", fullName, file)
			return errExit
		}
		if results.conflict > 0 {
			fmt.Fprintf(cmd.Out(), "You must update your resource version and run `%s update -f %s` to overwrite the remote changes.\n", fullName, file)
			return errExit
		}
		if len(results.edit) == 0 {
			if results.notfound == 0 {
				os.Remove(file)
			} else {
				fmt.Fprintf(cmd.Out(), "The edits you made on deleted resources have been saved to %q\n", file)
			}
			return nil
		}

		// loop again and edit the remaining items
		infos = results.edit
	}
	return nil
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
func (h *editHeader) WriteTo(w io.Writer) error {
	fmt.Fprint(w, `# Please edit the object below. Lines beginning with a '#' will be ignored,
# and an empty file will abort the edit. If an error occurs while saving this file will be
# reopened with the relevant failures.
#
`)
	for _, r := range h.reasons {
		if len(r.other) > 0 {
			fmt.Fprintf(w, "# %s:\n", r.head)
		} else {
			fmt.Fprintf(w, "# %s\n", r.head)
		}
		for _, o := range r.other {
			fmt.Fprintf(w, "# * %s\n", o)
		}
		fmt.Fprintln(w, "#")
	}
	return nil
}

// editResults capture the result of an update
type editResults struct {
	header    editHeader
	retryable int
	notfound  int
	conflict  int
	edit      []*resource.Info
	file      string
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
		r.conflict++
		// TODO: make this better by extracting the resource version of the new version and allowing the user
		// to know what command they need to run to update again (by rewriting the version?)
		return fmt.Sprintf("Error: %v", err)
	default:
		r.retryable++
		return fmt.Sprintf("Error: the %s %s could not be updated: %v", info.Mapping.Kind, info.Name, err)
	}
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

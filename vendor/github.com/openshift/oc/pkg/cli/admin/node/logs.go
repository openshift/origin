package node

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

var (
	logsLong = templates.LongDesc(`
		Display and filter node logs

		This command retrieves logs for the node. The default mode is to query the
		systemd journal on supported operating systems, which allows searching, time
		based filtering, and unit based filtering. You may also use the --path argument
		to see a list of log files available under /var/logs/ and view those contents
		directly.

		Node logs may contain sensitive output and so are limited to privileged node
		administrators. The system:node-admins role grants this permission by default.
		You check who has that permission via:

		    $ oc adm policy who-can --all-namespaces get nodes/log
		`)

	logsExample = templates.Examples(`
		# Show kubelet logs from all masters
		%[1]s node-logs --role master -u kubelet

		# See what logs are available in masters in /var/logs
		%[1]s node-logs --role master --path=/

		# Display cron log file from all masters
		%[1]s node-logs --role master --path=cron
	`)
)

// LogsOptions holds all the necessary options for running oc adm node-logs.
type LogsOptions struct {
	Resources []string
	Selector  string
	Role      string

	// the log path to fetch
	Path string

	// --path=journal specific arguments
	Grep              string
	GrepCaseSensitive bool
	Boot              int
	BootChanaged      bool
	Units             []string
	SinceTime         string
	UntilTime         string
	Tail              int
	Output            string

	// output format arguments
	Raw   bool
	Unify bool

	RESTClientGetter func(mapping *meta.RESTMapping) (resource.RESTClient, error)
	Builder          *resource.Builder

	genericclioptions.IOStreams
}

func NewLogsOptions(streams genericclioptions.IOStreams) *LogsOptions {
	return &LogsOptions{
		Path:              "journal",
		IOStreams:         streams,
		GrepCaseSensitive: true,
	}
}

// NewCmdLogs creates a new logs command that supports OpenShift resources.
func NewCmdLogs(baseName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewLogsOptions(streams)
	cmd := &cobra.Command{
		Use:                   "node-logs [-l LABELS] [NODE...]",
		DisableFlagsInUseLine: true,
		Short:                 "Display and filter node logs",
		Long:                  logsLong,
		Example:               fmt.Sprintf(logsExample, baseName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.RunLogs())
		},
	}

	cmd.Flags().StringVar(&o.Path, "path", o.Path, "Retrieve the specified path within the node's /var/logs/ folder. The 'journal' value will allow querying the journal on supported operating systems.")

	cmd.Flags().StringSliceVarP(&o.Units, "unit", "u", o.Units, "Return log entries from the specified unit(s). Only applies to node journal logs.")
	cmd.Flags().StringVarP(&o.Grep, "grep", "g", o.Grep, "Filter log entries by the provided regex pattern. Only applies to node journal logs.")
	cmd.Flags().BoolVar(&o.GrepCaseSensitive, "case-sensitive", o.GrepCaseSensitive, "Filters are case sensitive by default. Pass --case-sensitive=false to do a case insensitive filter.")
	cmd.Flags().StringVar(&o.SinceTime, "since", o.SinceTime, "Return logs after a specific ISO timestamp or relative date. Only applies to node journal logs.")
	cmd.Flags().StringVar(&o.UntilTime, "until", o.UntilTime, "Return logs before a specific ISO timestamp or relative date. Only applies to node journal logs.")
	cmd.Flags().IntVar(&o.Boot, "boot", o.Boot, " Show messages from a specific boot. Use negative numbers, allowed [-100, 0], passing invalid boot offset will fail retrieving logs. Only applies to node journal logs.")
	cmd.Flags().StringVarP(&o.Output, "output", "o", o.Output, "Display journal logs in an alternate format (short, cat, json, short-unix). Only applies to node journal logs.")
	cmd.Flags().IntVar(&o.Tail, "tail", o.Tail, "Return up to this many lines from the end of the log. Only applies to node journal logs.")

	cmd.Flags().StringVar(&o.Role, "role", o.Role, "Set a label selector by node role.")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on.")
	cmd.Flags().BoolVar(&o.Raw, "raw", o.Raw, "Perform no transformation of the returned data.")
	cmd.Flags().BoolVar(&o.Unify, "unify", o.Unify, "Interleave logs by sorting the output. Defaults on when viewing node journal logs")

	return cmd
}

func (o *LogsOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if !cmd.Flags().Lookup("unify").Changed {
		o.Unify = o.Path == "journal"
	}

	o.Resources = args

	o.RESTClientGetter = f.UnstructuredClientForMapping

	builder := f.NewBuilder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		SingleResourceType()

	if len(o.Resources) > 0 {
		builder.ResourceNames("nodes", o.Resources...)
	}
	if len(o.Role) > 0 {
		req, err := labels.NewRequirement(fmt.Sprintf("node-role.kubernetes.io/%s", o.Role), selection.Exists, nil)
		if err != nil {
			return fmt.Errorf("invalid --role: %v", err)
		}
		o.Selector = req.String()
	}
	if len(o.Selector) > 0 {
		builder.ResourceTypes("nodes").LabelSelectorParam(o.Selector)
	}
	o.Builder = builder
	o.BootChanaged = cmd.Flag("boot").Changed

	return nil
}

func (o LogsOptions) Validate() error {
	if len(o.Resources) == 0 && len(o.Selector) == 0 {
		return fmt.Errorf("at least one node name or a selector (-l) must be specified")
	}
	if len(o.Resources) > 0 && len(o.Selector) > 0 {
		return fmt.Errorf("node names and selector may not both be specified")
	}
	if o.BootChanaged && (o.Boot < -100 || o.Boot > 0) {
		return fmt.Errorf("--boot accepts values [-100, 0]")
	}
	return nil
}

// logRequest abstracts retrieving the content of the node logs endpoint which is normally
// either directory content or a file. It supports raw retrieval for use with the journal
// endpoint, and formats the HTML returned by a directory listing into a more user friendly
// output.
type logRequest struct {
	node string
	req  *rest.Request
	err  error

	// raw is set to true when we are viewing the journal and wish to skip prefixing
	raw bool
	// skipPrefix bypasses prefixing if the user knows that a unique identifier is already
	// in the file
	skipPrefix bool
}

// WriteTo prefixes the error message with the current node if necessary
func (req *logRequest) WriteRequest(out io.Writer) error {
	if req.err != nil {
		return req.err
	}
	err := req.writeTo(out)
	if err != nil {
		err = fmt.Errorf("%s %v", req.node, err)
		req.err = err
	}
	return err
}

func (req *logRequest) writeTo(out io.Writer) error {
	in, err := req.req.Stream()
	if err != nil {
		return err
	}
	defer in.Close()

	// raw output implies we may be getting binary content directly
	// from the remote and so we want to perform no translation
	if req.raw {
		// TODO: optionallyDecompress should be implemented by checking
		// the content-encoding of the response, but we perform optional
		// decompression here in case the content of the logs on the server
		// is also gzipped.
		return optionallyDecompress(out, in)
	}

	var prefix []byte
	if !req.skipPrefix {
		prefix = []byte(fmt.Sprintf("%s ", req.node))
	}

	return outputDirectoryEntriesOrContent(out, in, prefix)
}

// RunLogs retrieves node logs
func (o LogsOptions) RunLogs() error {
	builder := o.Builder

	var requests []*logRequest

	var errs []error
	result := builder.ContinueOnError().Flatten().Do()
	err := result.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			requests = append(requests, &logRequest{node: info.Name, err: err})
			return nil
		}
		mapping := info.ResourceMapping()
		client, err := o.RESTClientGetter(mapping)
		if err != nil {
			requests = append(requests, &logRequest{node: info.Name, err: err})
			return nil
		}
		path := client.Get().
			Namespace(info.Namespace).Name(info.Name).
			Resource(mapping.Resource.Resource).SubResource("proxy", "logs").Suffix(o.Path).URL().Path
		if strings.HasSuffix(o.Path, "/") {
			path += "/"
		}

		req := client.Get().RequestURI(path).
			SetHeader("Accept", "text/plain, */*").
			SetHeader("Accept-Encoding", "gzip")
		if o.Path == "journal" {
			if len(o.UntilTime) > 0 {
				req.Param("until", o.UntilTime)
			}
			if len(o.SinceTime) > 0 {
				req.Param("since", o.SinceTime)
			}
			if len(o.Output) > 0 {
				req.Param("output", o.Output)
			}
			if o.BootChanaged {
				req.Param("boot", fmt.Sprintf("%d", o.Boot))
			}
			if len(o.Units) > 0 {
				for _, unit := range o.Units {
					req.Param("unit", unit)
				}
			}
			if len(o.Grep) > 0 {
				req.Param("grep", o.Grep)
				req.Param("case-sensitive", fmt.Sprintf("%t", o.GrepCaseSensitive))
			}
			if o.Tail > 0 {
				req.Param("tail", strconv.Itoa(o.Tail))
			}
		}

		requests = append(requests, &logRequest{
			node: info.Name,
			req:  req,
			raw:  o.Raw || o.Path == "journal",
		})
		return nil
	})
	if err != nil {
		if agg, ok := err.(errors.Aggregate); ok {
			errs = append(errs, agg.Errors()...)
		} else {
			errs = append(errs, err)
		}
	}

	found := len(errs) + len(requests)
	// only hide prefix if the user specified a single item
	skipPrefix := found == 1 && result.TargetsSingleItems()

	// buffer output for slightly better streaming performance
	out := bufio.NewWriterSize(o.Out, 1024*16)
	defer out.Flush()

	if o.Unify {
		// unified output is each source, interleaved in lexographic order (assumes
		// the source input is sorted by time)
		var readers []Reader
		for i := range requests {
			req := requests[i]
			req.skipPrefix = true
			pr, pw := io.Pipe()
			readers = append(readers, Reader{
				R: pr,
			})
			go func() {
				err := req.WriteRequest(pw)
				pw.CloseWithError(err)
			}()
		}
		_, err := NewMergeReader(readers...).WriteTo(out)
		if agg := errors.Flatten(errors.NewAggregate([]error{err})); agg != nil {
			errs = append(errs, agg.Errors()...)
		}

	} else {
		// display files sequentially
		for _, req := range requests {
			req.skipPrefix = skipPrefix
			if err := req.WriteRequest(out); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(o.ErrOut, "error: %v\n", err)
		}
		return kcmdutil.ErrExit
	}

	return nil
}

func optionallyDecompress(out io.Writer, in io.Reader) error {
	bufferSize := 4096
	buf := bufio.NewReaderSize(in, bufferSize)
	head, err := buf.Peek(1024)
	if err != nil && err != io.EOF {
		return err
	}
	if _, err := gzip.NewReader(bytes.NewBuffer(head)); err != nil {
		// not a gzipped stream
		_, err = io.Copy(out, buf)
		return err
	}
	r, err := gzip.NewReader(buf)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	return err
}

func outputDirectoryEntriesOrContent(out io.Writer, in io.Reader, prefix []byte) error {
	bufferSize := 4096
	buf := bufio.NewReaderSize(in, bufferSize)

	// turn href links into lines of output
	content, _ := buf.Peek(bufferSize)
	if bytes.HasPrefix(content, []byte("<pre>")) {
		reLink := regexp.MustCompile(`href="([^"]+)"`)
		s := bufio.NewScanner(buf)
		s.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
			matches := reLink.FindSubmatchIndex(data)
			if matches == nil {
				advance = bytes.LastIndex(data, []byte("\n"))
				if advance == -1 {
					advance = 0
				}
				return advance, nil, nil
			}
			advance = matches[1]
			token = data[matches[2]:matches[3]]
			return advance, token, nil
		})
		for s.Scan() {
			if _, err := out.Write(prefix); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out, s.Text()); err != nil {
				return err
			}
		}
		return s.Err()
	}

	// without a prefix we can copy directly
	if len(prefix) == 0 {
		_, err := io.Copy(out, buf)
		return err
	}

	r := NewMergeReader(Reader{R: buf, Prefix: prefix})
	_, err := r.WriteTo(out)
	return err
}

// Reader wraps an io.Reader and inserts the provided prefix at the
// beginning of the output and before each newline character found
// in the stream.
type Reader struct {
	R      io.Reader
	Prefix []byte
}

type mergeReader []Reader

// NewMergeReader attempts to display the provided readers as line
// oriented output in lexographic order by always reading the next
// available line from the reader with the "smallest" line.
//
// For example, given the readers with the following lines:
//   1: A
//      B
//      D
//   2: C
//      D
//      E
//
//  the reader would contain:
//      A
//      B
//      C
//      D
//      D
//      E
//
// The merge reader uses bufio.NewReader() for each input and the
// ReadLine() method to find the next shortest input. If a given
// line is longer than the buffer size of 4096, and all readers
// have the same initial 4096 characters, the order is undefined.
func NewMergeReader(r ...Reader) io.WriterTo {
	return mergeReader(r)
}

// WriteTo copies the provided readers into the provided output.
func (r mergeReader) WriteTo(out io.Writer) (int64, error) {
	// shortcut common cases
	switch len(r) {
	case 0:
		return 0, nil
	case 1:
		if len(r[0].Prefix) == 0 {
			return io.Copy(out, r[0].R)
		}
	}

	// initialize the buffered readers
	bufSize := 4096
	var buffers sortedBuffers
	var errs []error
	for _, in := range r {
		buf := &buffer{
			r:      bufio.NewReaderSize(in.R, bufSize),
			prefix: in.Prefix,
		}
		if err := buf.next(); err != nil {
			errs = append(errs, err)
			continue
		}
		buffers = append(buffers, buf)
	}

	var n int64
	for len(buffers) > 0 {
		// find the lowest buffer
		sort.Sort(buffers)

		// write out the line from the smallest buffer
		buf := buffers[0]

		if len(buf.prefix) > 0 {
			b, err := out.Write(buf.prefix)
			n += int64(b)
			if err != nil {
				return n, err
			}
		}

		for {
			done := !buf.linePrefix
			b, err := out.Write(buf.line)
			n += int64(b)
			if err != nil {
				return n, err
			}

			// try to fill the buffer, and if we get an error reading drop this source
			if err := buf.next(); err != nil {
				errs = append(errs, err)
				buffers = buffers[1:]
				break
			}

			// we reached the end of our line
			if done {
				break
			}
		}
		b, err := fmt.Fprintln(out)
		n += int64(b)
		if err != nil {
			return n, err
		}
	}

	return n, errors.FilterOut(errors.NewAggregate(errs), func(err error) bool { return err == io.EOF })
}

type buffer struct {
	r          *bufio.Reader
	prefix     []byte
	line       []byte
	linePrefix bool
}

func (b *buffer) next() error {
	var err error
	b.line, b.linePrefix, err = b.r.ReadLine()
	return err
}

type sortedBuffers []*buffer

func (buffers sortedBuffers) Less(i, j int) bool {
	return bytes.Compare(buffers[i].line, buffers[j].line) < 0
}
func (buffers sortedBuffers) Swap(i, j int) {
	buffers[i], buffers[j] = buffers[j], buffers[i]
}
func (buffers sortedBuffers) Len() int {
	return len(buffers)
}

package observe

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/origin/pkg/util/proc"
)

var (
	observeCounts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "observe_counts",
			Help: "Number of changes observed to the underlying resource.",
		},
		[]string{"type"},
	)
	execDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "observe_exec_durations_milliseconds",
			Help: "Item execution latency distributions.",
		},
		[]string{"type", "exit_code"},
	)
	nameExecDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "observe_name_exec_durations_milliseconds",
			Help: "Name list execution latency distributions.",
		},
		[]string{"exit_code"},
	)
)

var (
	observeLong = templates.LongDesc(`
		Observe changes to resources and take action on them

		This command assists in building scripted reactions to changes that occur in
		Kubernetes or OpenShift resources. This is frequently referred to as a
		'controller' in Kubernetes and acts to ensure particular conditions are
		maintained. On startup, observe will list all of the resources of a
		particular type and execute the provided script on each one. Observe watches
		the server for changes, and will reexecute the script for each update.

		Observe works best for problems of the form "for every resource X, make sure
		Y is true". Some examples of ways observe can be used include:

		* Ensure every namespace has a quota or limit range object
		* Ensure every service is registered in DNS by making calls to a DNS API
		* Send an email alert whenever a node reports 'NotReady'
		* Watch for the 'FailedScheduling' event and write an IRC message
		* Dynamically provision persistent volumes when a new PVC is created
		* Delete pods that have reached successful completion after a period of time.

		The simplest pattern is maintaining an invariant on an object - for instance,
		"every namespace should have an annotation that indicates its owner". If the
		object is deleted no reaction is necessary. A variation on that pattern is
		creating another object: "every namespace should have a quota object based
		on the resources allowed for an owner".

		    $ cat set_owner.sh
		    #!/bin/sh
		    if [[ "$(%[1]s get namespace "$1" --template='{{ .metadata.annotations.owner }}')" == "" ]]; then
		      %[1]s annotate namespace "$1" owner=bob
		    fi

		    $ %[1]s observe namespaces -- ./set_owner.sh

		The set_owner.sh script is invoked with a single argument (the namespace name)
		for each namespace. This simple script ensures that any user without the
		"owner" annotation gets one set, but preserves any existing value.

		The next common of controller pattern is provisioning - making changes in an
		external system to match the state of a Kubernetes resource. These scripts need
		to account for deletions that may take place while the observe command is not
		running. You can provide the list of known objects via the --names command,
		which should return a newline-delimited list of names or namespace/name pairs.
		Your command will be invoked whenever observe checks the latest state on the
		server - any resources returned by --names that are not found on the server
		will be passed to your --delete command.

		For example, you may wish to ensure that every node that is added to Kubernetes
		is added to your cluster inventory along with its IP:

		    $ cat add_to_inventory.sh
		    #!/bin/sh
		    echo "$1 $2" >> inventory
		    sort -u inventory -o inventory

		    $ cat remove_from_inventory.sh
		    #!/bin/sh
		    grep -vE "^$1 " inventory > /tmp/newinventory
		    mv -f /tmp/newinventory inventory

		    $ cat known_nodes.sh
		    #!/bin/sh
		    touch inventory
		    cut -f 1-1 -d ' ' inventory

		    $ %[1]s observe nodes -a '{ .status.addresses[0].address }' \
		      --names ./known_nodes.sh \
		      --delete ./remove_from_inventory.sh \
		      -- ./add_to_inventory.sh

		If you stop the observe command and then delete a node, when you launch observe
		again the contents of inventory will be compared to the list of nodes from the
		server, and any node in the inventory file that no longer exists will trigger
		a call to remove_from_inventory.sh with the name of the node.

		Important: when handling deletes, the previous state of the object may not be
		available and only the name/namespace of the object will be passed to	your
		--delete command as arguments (all custom arguments are omitted).

		More complicated interactions build on the two examples above - your inventory
		script could make a call to allocate storage on your infrastructure as a
		service, or register node names in DNS, or set complex firewalls. The more
		complex your integration, the more important it is to record enough data in the
		remote system that you can identify when resources on either side are deleted.`)

	observeExample = templates.Examples(`
		# Observe changes to services
	  %[1]s observe services

	  # Observe changes to services, including the clusterIP and invoke a script for each
	  %[1]s observe services -a '{ .spec.clusterIP }' -- register_dns.sh`)
)

type ObserveOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	debugOut io.Writer
	quiet    bool

	client           resource.RESTClient
	mapping          *meta.RESTMapping
	includeNamespace bool

	// which resources to select
	namespace     string
	allNamespaces bool

	// additional debugging information
	listenAddr string

	// actions to take on each object
	eachCommand   []string
	objectEnvVar  string
	typeEnvVar    string
	deleteCommand stringSliceFlag

	// controls whether deletes are included
	nameSyncCommand stringSliceFlag

	// error handling logic
	observedErrors  int
	maximumErrors   int
	retryCount      int
	retryExitStatus int

	// when to exit or reprocess the list of items
	once               bool
	exitAfterPeriod    time.Duration
	resyncPeriod       time.Duration
	printMetricsOnExit bool

	// control the output of the command
	templates       stringSliceFlag
	printer         VersionedPrinterList
	strictTemplates bool

	argumentStore *objectArgumentsStore
	// knownObjects is nil if we do not need to track deletions
	knownObjects knownObjects

	genericclioptions.IOStreams
}

func NewObserveOptions(streams genericclioptions.IOStreams) *ObserveOptions {
	return &ObserveOptions{
		PrintFlags: (&genericclioptions.PrintFlags{
			TemplatePrinterFlags: genericclioptions.NewKubeTemplatePrintFlags(),
		}).WithDefaultOutput("jsonpath").WithTypeSetter(scheme.Scheme),
		IOStreams: streams,

		retryCount:    2,
		maximumErrors: 20,
		listenAddr:    ":11251",
	}
}

// NewCmdObserve creates the observe command.
func NewCmdObserve(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewObserveOptions(streams)

	cmd := &cobra.Command{
		Use:     "observe RESOURCE [-- COMMAND ...]",
		Short:   "Observe changes to resources and react to them (experimental)",
		Long:    fmt.Sprintf(observeLong, fullName),
		Example: fmt.Sprintf(observeExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := o.Complete(f, cmd, args); err != nil {
				cmdutil.CheckErr(err)
			}

			if err := o.Validate(args); err != nil {
				cmdutil.CheckErr(cmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := o.Run(); err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	// flags controlling what to select
	cmd.Flags().BoolVar(&o.allNamespaces, "all-namespaces", o.allNamespaces, "If true, list the requested object(s) across all projects. Project in current context is ignored.")

	// to perform deletion synchronization
	cmd.Flags().VarP(&o.deleteCommand, "delete", "d", "A command to run when resources are deleted. Specify multiple times to add arguments.")
	cmd.Flags().Var(&o.nameSyncCommand, "names", "A command that will list all of the currently known names, optional. Specify multiple times to add arguments. Use to get notifications when objects are deleted.")

	// add additional arguments / info to the server
	cmd.Flags().BoolVar(&o.strictTemplates, "strict-templates", o.strictTemplates, "If true return an error on any field or map key that is not missing in a template.")
	cmd.Flags().VarP(&o.templates, "argument", "a", "Template for the arguments to be passed to each command in the format defined by --output.")
	cmd.Flags().StringVar(&o.typeEnvVar, "type-env-var", o.typeEnvVar, "The name of an env var to set with the type of event received ('Sync', 'Updated', 'Deleted', 'Added') to the reaction command or --delete.")
	cmd.Flags().StringVar(&o.objectEnvVar, "object-env-var", o.objectEnvVar, "The name of an env var to serialize the object to when calling the command, optional.")

	// control retries of individual commands
	cmd.Flags().IntVar(&o.maximumErrors, "maximum-errors", o.maximumErrors, "Exit after this many errors have been detected with. May be set to -1 for no maximum.")
	cmd.Flags().IntVar(&o.retryExitStatus, "retry-on-exit-code", o.retryExitStatus, "If any command returns this exit code, retry up to --retry-count times.")
	cmd.Flags().IntVar(&o.retryCount, "retry-count", o.retryCount, "The number of times to retry a failing command before continuing.")

	// control observe program behavior
	cmd.Flags().BoolVar(&o.once, "once", o.once, "If true, exit with a status code 0 after all current objects have been processed.")
	cmd.Flags().DurationVar(&o.exitAfterPeriod, "exit-after", o.exitAfterPeriod, "Exit with status code 0 after the provided duration, optional.")
	cmd.Flags().DurationVar(&o.resyncPeriod, "resync-period", o.resyncPeriod, "When non-zero, periodically reprocess every item from the server as a Sync event. Use to ensure external systems are kept up to date.")
	cmd.Flags().BoolVar(&o.printMetricsOnExit, "print-metrics-on-exit", o.printMetricsOnExit, "If true, on exit write all metrics to stdout.")
	cmd.Flags().StringVar(&o.listenAddr, "listen-addr", o.listenAddr, "The name of an interface to listen on to expose metrics and health checking.")

	// additional debug output
	cmd.Flags().BoolVar(&o.quiet, "no-headers", o.quiet, "If true, skip printing information about each event prior to executing the command.")
	cmd.Flags().MarkDeprecated("no-headers", "This flag will be removed in a future release. Use --quiet instead.")
	cmd.Flags().BoolVarP(&o.quiet, "quiet", "q", o.quiet, "If true, skip printing information about each event prior to executing the command.")

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *ObserveOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error

	var command []string
	if i := cmd.ArgsLenAtDash(); i != -1 {
		command = args[i:]
		args = args[:i]
	}

	o.eachCommand = command

	switch len(args) {
	case 0:
		return fmt.Errorf("you must specify at least one argument containing the resource to observe")
	case 1:
	default:
		return fmt.Errorf("you may only specify one argument containing the resource to observe (use '--' to separate your resource and your command)")
	}

	gr := schema.ParseGroupResource(args[0])
	if gr.Empty() {
		return fmt.Errorf("unknown resource argument")
	}

	mapper, err := f.ToRESTMapper()
	if err != nil {
		return err
	}

	version, err := mapper.KindFor(gr.WithVersion(""))
	if err != nil {
		return err
	}
	mapping, err := mapper.RESTMapping(version.GroupKind())
	if err != nil {
		return err
	}
	o.mapping = mapping
	o.includeNamespace = mapping.Scope.Name() == meta.RESTScopeNamespace.Name()

	o.client, err = f.ClientForMapping(mapping)
	if err != nil {
		return err
	}

	o.namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// TODO: Remove in the next release
	// support backwards compatibility with misspelling of "go-template" output format
	if o.PrintFlags.OutputFormat != nil && *o.PrintFlags.OutputFormat == "gotemplate" {
		fmt.Fprintf(o.ErrOut, "DEPRECATED: The %q output format has been replaced by %q, please use the new spelling instead\n", "gotemplate", "go-template")
		*o.PrintFlags.OutputFormat = "go-template"
	}

	// TODO: Remove in the next release
	// support backwards compatibility with incorrect flag --strict-templates
	if o.strictTemplates {
		fmt.Fprintf(o.ErrOut, "DEPRECATED: %q has been replaced by %q, please use the new flag instead\n", "--strict-templates", "--allow-missing-template-keys=false")
		*o.PrintFlags.TemplatePrinterFlags.AllowMissingKeys = !o.strictTemplates
	}

	// prevent output-format from defaulting to go-template if no --output format
	// is explicitly provided by the user
	o.PrintFlags.OutputFlagSpecified = func() bool { return true }
	printerList := []printers.ResourcePrinter{}

	if p, err := o.PrintFlags.ToPrinter(); err == nil {
		// here we only want to see if a valid output format containing a template arg was specified
		// and there is a matching printer. If so, add that printer to our list
		printerList = append(printerList, p)
	}

	// normalize --output value, keep the specified go-template or jsonpath output format,
	// but remove any template arg specified as part of it
	if o.PrintFlags.OutputFormat != nil {
		*o.PrintFlags.OutputFormat = strings.Split(*o.PrintFlags.OutputFormat, "=")[0]
	}

	for _, arg := range o.templates {
		if len(arg) == 0 {
			continue
		}

		// take the value of our existing PrintFlags, and re-wrap them
		// in a new TypeSetter printer to end up with new value.
		// we do this in order to avoid dealing with the same TypeSetter object
		// on every iteration, and instead obtain a new printer altogether.
		printFlags := *o.PrintFlags.WithTypeSetter(scheme.Scheme)

		// we want to set the template argument value at the address
		// that TemplateArgument already points to, since that address
		// is shared with both the JSONPath and GoTemplate PrintFlags.
		*printFlags.TemplatePrinterFlags.TemplateArgument = arg
		templatePrinter, err := printFlags.ToPrinter()
		if err != nil {
			return err
		}

		printerList = append(printerList, templatePrinter)
	}

	o.printer = VersionedPrinterList(printerList)
	if o.quiet {
		o.debugOut = ioutil.Discard
	} else {
		o.debugOut = o.Out
	}

	o.argumentStore = &objectArgumentsStore{}
	switch {
	case len(o.nameSyncCommand) > 0:
		o.argumentStore.keyFn = func() ([]string, error) {
			var out []byte
			err := retryCommandError(o.retryExitStatus, o.retryCount, func() error {
				c := exec.Command(o.nameSyncCommand[0], o.nameSyncCommand[1:]...)
				var err error
				return measureCommandDuration(nameExecDurations, func() error {
					out, err = c.Output()
					return err
				})
			})
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					if len(exit.Stderr) > 0 {
						err = fmt.Errorf("%v\n%s", err, string(exit.Stderr))
					}
				}
				return nil, err
			}
			names := strings.Split(string(out), "\n")
			sort.Sort(sort.StringSlice(names))
			var outputNames []string
			for i, s := range names {
				if len(s) != 0 {
					outputNames = names[i:]
					break
				}
			}
			glog.V(4).Infof("Found existing keys: %v", outputNames)
			return outputNames, nil
		}
		o.knownObjects = o.argumentStore
	case len(o.deleteCommand) > 0, o.resyncPeriod > 0:
		o.knownObjects = o.argumentStore
	}

	return nil
}

func (o *ObserveOptions) Validate(args []string) error {
	if len(o.nameSyncCommand) > 0 && len(o.deleteCommand) == 0 {
		return fmt.Errorf("--delete and --names must both be specified")
	}
	return nil
}

func (o *ObserveOptions) Run() error {
	if len(o.deleteCommand) > 0 && len(o.nameSyncCommand) == 0 {
		fmt.Fprintf(o.ErrOut, "warning: If you are modifying resources outside of %q, you should use the --names command to ensure you don't miss deletions that occur while the command is not running.\n", o.mapping.Resource)
	}

	// watch the given resource for changes
	store := cache.NewDeltaFIFO(objectArgumentsKeyFunc, o.knownObjects)
	lw := restListWatcher{Helper: resource.NewHelper(o.client, o.mapping)}
	if !o.allNamespaces {
		lw.namespace = o.namespace
	}

	// ensure any child processes are reaped if we are running as PID 1
	proc.StartReaper()

	// listen on the provided address for metrics
	if len(o.listenAddr) > 0 {
		prometheus.MustRegister(observeCounts)
		prometheus.MustRegister(execDurations)
		prometheus.MustRegister(nameExecDurations)
		errWaitingForSync := fmt.Errorf("waiting for initial sync")
		healthz.InstallHandler(http.DefaultServeMux, healthz.NamedCheck("ready", func(r *http.Request) error {
			if !store.HasSynced() {
				return errWaitingForSync
			}
			return nil
		}))
		http.Handle("/metrics", prometheus.Handler())
		go func() {
			glog.Fatalf("Unable to listen on %q: %v", o.listenAddr, http.ListenAndServe(o.listenAddr, nil))
		}()
		glog.V(2).Infof("Listening on %s at /metrics and /healthz", o.listenAddr)
	}

	// exit cleanly after a certain period
	// lock guards the loop to ensure no child processes are running
	var lock sync.Mutex
	if o.exitAfterPeriod > 0 {
		go func() {
			<-time.After(o.exitAfterPeriod)
			lock.Lock()
			defer lock.Unlock()
			o.dumpMetrics()
			fmt.Fprintf(o.ErrOut, "Shutting down after %s ...\n", o.exitAfterPeriod)
			os.Exit(0)
		}()
	}

	defer o.dumpMetrics()
	stopCh := make(chan struct{})
	defer close(stopCh)

	// start the reflector
	reflector := cache.NewNamedReflector("observer", lw, nil, store, o.resyncPeriod)
	go func() {
		observedListErrors := 0
		wait.Until(func() {
			if err := reflector.ListAndWatch(stopCh); err != nil {
				utilruntime.HandleError(err)
				observedListErrors++
				if o.maximumErrors != -1 && observedListErrors > o.maximumErrors {
					glog.Fatalf("Maximum list errors of %d reached, exiting", o.maximumErrors)
				}
			}
		}, time.Second, stopCh)
	}()

	if o.once {
		// wait until the reflector reports it has completed the initial list and the
		// fifo has been populated
		for len(reflector.LastSyncResourceVersion()) == 0 {
			time.Sleep(50 * time.Millisecond)
		}
		// if the store is empty, there is nothing to sync
		if store.HasSynced() && len(store.ListKeys()) == 0 {
			fmt.Fprintf(o.ErrOut, "Nothing to sync, exiting immediately\n")
			return nil
		}
	}

	// process all changes that occur in the resource
	syncing := false
	for {
		_, err := store.Pop(func(obj interface{}) error {
			// if we failed to retrieve the list of keys, exit
			if err := o.argumentStore.ListKeysError(); err != nil {
				return fmt.Errorf("unable to list known keys: %v", err)
			}

			deltas := obj.(cache.Deltas)
			for _, delta := range deltas {
				if err := func() error {
					lock.Lock()
					defer lock.Unlock()

					// handle before and after observe notification
					switch {
					case !syncing && delta.Type == cache.Sync:
						if err := o.startSync(); err != nil {
							return err
						}
						syncing = true
					case syncing && delta.Type != cache.Sync:
						if err := o.finishSync(); err != nil {
							return err
						}
						syncing = false
					}

					// require the user to provide a name function in order to get events beyond added / updated
					if !syncing && o.knownObjects == nil && !(delta.Type == cache.Added || delta.Type == cache.Updated) {
						return nil
					}

					observeCounts.WithLabelValues(string(delta.Type)).Inc()

					// calculate the arguments for the delta and then invoke any command
					object, arguments, output, err := o.calculateArguments(delta)
					if err != nil {
						return err
					}
					if err := o.next(delta.Type, object, output, arguments); err != nil {
						return err
					}

					return nil
				}(); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		// if we only want to run once, exit here
		if o.once && store.HasSynced() {
			if syncing {
				if err := o.finishSync(); err != nil {
					return err
				}
			}
			return nil
		}
	}
}

// calculateArguments determines the arguments for a give delta and updates the argument store, or returns
// an error. If the object can be transformed into a full JSON object, that is also returned.
func (o *ObserveOptions) calculateArguments(delta cache.Delta) (runtime.Object, []string, []byte, error) {
	var arguments []string
	var object runtime.Object
	var key string
	var output []byte

	switch t := delta.Object.(type) {
	case cache.DeletedFinalStateUnknown:
		key = t.Key
		if obj, ok := t.Obj.(runtime.Object); ok {
			object = obj

			args := &ArgsWriter{}
			data, err := o.printer.PrintObj(obj, args)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("unable to write arguments: %v", err)
			}
			arguments = args.Args()
			output = data

		} else {
			value, _, err := o.argumentStore.GetByKey(key)
			if err != nil {
				return nil, nil, nil, err
			}
			if value != nil {
				args, ok := value.(objectArguments)
				if !ok {
					return nil, nil, nil, fmt.Errorf("unexpected cache value %T", value)
				}
				arguments = args.arguments
				output = args.output
			}
		}

		o.argumentStore.Remove(key)

	case runtime.Object:
		object = t

		args := &ArgsWriter{}
		data, err := o.printer.PrintObj(t, args)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("unable to write arguments: %v", err)
		}
		arguments = args.Args()
		output = data

		key, _ = cache.MetaNamespaceKeyFunc(t)
		if delta.Type == cache.Deleted {
			o.argumentStore.Remove(key)
		} else {
			saved := objectArguments{key: key, arguments: arguments}
			// only cache the object data if the commands will be using it.
			if len(o.objectEnvVar) > 0 {
				saved.output = output
			}
			o.argumentStore.Put(key, saved)
		}

	case objectArguments:
		key = t.key
		arguments = t.arguments
		output = t.output

	default:
		return nil, nil, nil, fmt.Errorf("unrecognized object %T from cache store", delta.Object)
	}

	if object == nil {
		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return nil, nil, nil, err
		}
		unstructured := &unstructured.Unstructured{}
		unstructured.SetNamespace(namespace)
		unstructured.SetName(name)
		object = unstructured
	}

	return object, arguments, output, nil
}

func (o *ObserveOptions) startSync() error {
	fmt.Fprintf(o.debugOut, "# %s Sync started\n", time.Now().Format(time.RFC3339))
	return nil
}
func (o *ObserveOptions) finishSync() error {
	fmt.Fprintf(o.debugOut, "# %s Sync ended\n", time.Now().Format(time.RFC3339))
	return nil
}

func (o *ObserveOptions) next(deltaType cache.DeltaType, obj runtime.Object, output []byte, arguments []string) error {
	glog.V(4).Infof("Processing %s %v: %#v", deltaType, arguments, obj)
	m, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("unable to handle %T: %v", obj, err)
	}
	resourceVersion := m.GetResourceVersion()

	outType := string(deltaType)

	var args []string
	if o.includeNamespace {
		args = append(args, m.GetNamespace())
	}
	args = append(args, m.GetName())

	var command string
	switch {
	case deltaType == cache.Deleted:
		if len(o.deleteCommand) > 0 {
			command = o.deleteCommand[0]
			args = append(append([]string{}, o.deleteCommand[1:]...), args...)
		}
	case len(o.eachCommand) > 0:
		command = o.eachCommand[0]
		args = append(append([]string{}, o.eachCommand[1:]...), args...)
	}

	args = append(args, arguments...)

	if len(command) == 0 {
		fmt.Fprintf(o.debugOut, "# %s %s %s\t%s\n", time.Now().Format(time.RFC3339), outType, resourceVersion, printCommandLine(command, args...))
		return nil
	}

	fmt.Fprintf(o.debugOut, "# %s %s %s\t%s\n", time.Now().Format(time.RFC3339), outType, resourceVersion, printCommandLine(command, args...))

	out, errOut := &newlineTrailingWriter{w: o.Out}, &newlineTrailingWriter{w: o.ErrOut}

	err = retryCommandError(o.retryExitStatus, o.retryCount, func() error {
		cmd := exec.Command(command, args...)
		cmd.Stdout = out
		cmd.Stderr = errOut
		cmd.Env = os.Environ()
		if len(o.objectEnvVar) > 0 {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", o.objectEnvVar, string(output)))
		}
		if len(o.typeEnvVar) > 0 {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", o.typeEnvVar, string(outType)))
		}
		err := measureCommandDuration(execDurations, cmd.Run, outType)
		out.Flush()
		errOut.Flush()
		return err
	})
	if err != nil {
		if code, ok := exitCodeForCommandError(err); ok && code != 0 {
			err = fmt.Errorf("command %q exited with status code %d", command, code)
		}
		return o.handleCommandError(err)
	}
	return nil
}

func (o *ObserveOptions) handleCommandError(err error) error {
	if err == nil {
		return nil
	}
	o.observedErrors++
	fmt.Fprintf(o.ErrOut, "error: %v\n", err)
	if o.maximumErrors == -1 || o.observedErrors < o.maximumErrors {
		return nil
	}
	return fmt.Errorf("reached maximum error limit of %d, exiting", o.maximumErrors)
}

func (o *ObserveOptions) dumpMetrics() {
	if !o.printMetricsOnExit {
		return
	}
	w := httptest.NewRecorder()
	prometheus.UninstrumentedHandler().ServeHTTP(w, &http.Request{})
	if w.Code == http.StatusOK {
		fmt.Fprintf(o.Out, w.Body.String())
	}
}

func measureCommandDuration(m *prometheus.SummaryVec, fn func() error, labels ...string) error {
	n := time.Now()
	err := fn()
	duration := time.Now().Sub(n)
	statusCode, ok := exitCodeForCommandError(err)
	if !ok {
		statusCode = -1
	}
	m.WithLabelValues(append(labels, strconv.Itoa(statusCode))...).Observe(float64(duration / time.Millisecond))

	if errnoError(err) == syscall.ECHILD {
		// ignore wait4 syscall errno as it means
		// that the subprocess has started and ended
		// before the wait call was made.
		return nil
	}

	return err
}

func errnoError(err error) syscall.Errno {
	if se, ok := err.(*os.SyscallError); ok {
		if errno, ok := se.Err.(syscall.Errno); ok {
			return errno
		}
	}

	return 0
}

func exitCodeForCommandError(err error) (int, bool) {
	if err == nil {
		return 0, true
	}
	if exit, ok := err.(*exec.ExitError); ok {
		if ws, ok := exit.ProcessState.Sys().(syscall.WaitStatus); ok {
			return ws.ExitStatus(), true
		}
	}
	return 0, false
}

func retryCommandError(onExitStatus, times int, fn func() error) error {
	err := fn()
	if err != nil && onExitStatus != 0 && times > 0 {
		if status, ok := exitCodeForCommandError(err); ok {
			if status == onExitStatus {
				glog.V(4).Infof("retrying command: %v", err)
				return retryCommandError(onExitStatus, times-1, fn)
			}
		}
	}
	return err
}

func printCommandLine(cmd string, args ...string) string {
	outCmd := cmd
	if strings.ContainsAny(outCmd, "\"\\ ") {
		outCmd = strconv.Quote(outCmd)
	}
	if len(outCmd) == 0 {
		outCmd = "\"\""
	}
	outArgs := make([]string, 0, len(args))
	for _, s := range args {
		if strings.ContainsAny(s, "\"\\ ") {
			s = strconv.Quote(s)
		}
		if len(s) == 0 {
			s = "\"\""
		}
		outArgs = append(outArgs, s)
	}
	if len(outArgs) == 0 {
		return outCmd
	}
	return fmt.Sprintf("%s %s", outCmd, strings.Join(outArgs, " "))
}

type restListWatcher struct {
	*resource.Helper
	namespace string
}

func (lw restListWatcher) List(opt metav1.ListOptions) (runtime.Object, error) {
	return lw.Helper.List(lw.namespace, "", false, &opt)
}

func (lw restListWatcher) Watch(opt metav1.ListOptions) (watch.Interface, error) {
	return lw.Helper.Watch(lw.namespace, opt.ResourceVersion, &opt)
}

type flushableWriter interface {
	Flush()
}

type ArgsWriter struct {
	buffer []byte
	args   []string
}

func (w *ArgsWriter) Args() []string {
	return w.args
}

func (w *ArgsWriter) Flush() {
	w.args = append(w.args, string(w.buffer))
	w.buffer = []byte{}
}

func (w *ArgsWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)
	return len(w.buffer), nil
}

// VersionedPrinterList wraps a list of template printers for a given object
// and converts the object to a groupified external API version before printing.
type VersionedPrinterList []printers.ResourcePrinter

func (p VersionedPrinterList) PrintObj(obj runtime.Object, out io.Writer) ([]byte, error) {
	// TODO(juanvallejo): This conversion to externals needs to be removed
	// once we switch to using the dynamic client. This is being tracked
	// in https://github.com/openshift/origin/issues/20992.
	data, err := runtime.Encode(scheme.DefaultJSONEncoder(), obj)
	if err != nil {
		return nil, err
	}

	// decode object, preferring prioritized versions
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	converted, err := runtime.Decode(decoder, data)
	if err != nil {
		if !runtime.IsNotRegisteredError(err) {
			return nil, err
		}

		panic(fmt.Errorf("unable to decode obj: %#v\n%v", obj, err))
	}

	for _, printer := range p {
		if err := printer.PrintObj(converted, out); err != nil {
			return nil, err
		}
		if w, ok := out.(flushableWriter); ok {
			w.Flush()
		}
	}

	return data, nil
}

type knownObjects interface {
	cache.KeyListerGetter

	ListKeysError() error
	Put(key string, value interface{})
	Remove(key string)
}

type objectArguments struct {
	key       string
	arguments []string
	output    []byte
}

func objectArgumentsKeyFunc(obj interface{}) (string, error) {
	if args, ok := obj.(objectArguments); ok {
		return args.key, nil
	}
	return cache.MetaNamespaceKeyFunc(obj)
}

type objectArgumentsStore struct {
	keyFn func() ([]string, error)

	lock      sync.Mutex
	arguments map[string]interface{}
	err       error
}

func (r *objectArgumentsStore) ListKeysError() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.err
}

func (r *objectArgumentsStore) ListKeys() []string {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.keyFn != nil {
		var keys []string
		keys, r.err = r.keyFn()
		return keys
	}

	keys := make([]string, 0, len(r.arguments))
	for k := range r.arguments {
		keys = append(keys, k)
	}
	return keys
}

func (r *objectArgumentsStore) GetByKey(key string) (interface{}, bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	args := r.arguments[key]
	return args, true, nil
}

func (r *objectArgumentsStore) Put(key string, arguments interface{}) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.arguments == nil {
		r.arguments = make(map[string]interface{})
	}
	r.arguments[key] = arguments
}

func (r *objectArgumentsStore) Remove(key string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	delete(r.arguments, key)
}

type newlineTrailingWriter struct {
	w        io.Writer
	openLine bool
}

func (w *newlineTrailingWriter) Write(data []byte) (int, error) {
	if len(data) > 0 && data[len(data)-1] != '\n' {
		w.openLine = true
	}
	return w.w.Write(data)
}

func (w *newlineTrailingWriter) Flush() error {
	if w.openLine {
		w.openLine = false
		_, err := fmt.Fprintln(w.w)
		return err
	}
	return nil
}

type stringSliceFlag []string

func (f *stringSliceFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *stringSliceFlag) String() string { return strings.Join(*f, " ") }
func (f *stringSliceFlag) Type() string   { return "string" }

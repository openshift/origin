package storage

import (
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/oc/admin/migrate"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	internalMigrateStorageLong = templates.LongDesc(`
		Migrate internal object storage via update

		This command invokes an update operation on every API object reachable by the caller. This forces
		the server to write to the underlying storage if the object representation has changed. Use this
		command to ensure that the most recent storage changes have been applied to all objects (storage
		version, storage encoding, any newer object defaults).

		To operate on a subset of resources, use the --include flag. If you encounter errors during a run
		the command will output a list of resources that received errors, which you can then re-run the
		command on. You may also specify --from-key and --to-key to restrict the set of resource names
		to operate on (key is NAMESPACE/NAME for resources in namespaces or NAME for cluster scoped
		resources). --from-key is inclusive if specified, while --to-key is exclusive.

		By default, events are not migrated since they expire within a very short period of time. If you
		have significantly increased the expiration time of events, run a migration with --include=events

		WARNING: This is a slow command and will put significant load on an API server. It may also
		result in significant intra-cluster traffic.`)

	internalMigrateStorageExample = templates.Examples(`
	  # Perform an update of all objects
	  %[1]s

	  # Only migrate pods
	  %[1]s --include=pods

	  # Only pods that are in namespaces starting with "bar"
	  %[1]s --include=pods --from-key=bar/ --to-key=bar/\xFF`)
)

const (
	// longThrottleLatency defines threshold for logging requests. All requests being
	// throttle for more than longThrottleLatency will be logged.
	longThrottleLatency = 50 * time.Millisecond

	// 1 MB == 1000 KB
	mbToKB = 1000
	// 1 KB == 1000 bytes
	kbToBytes = 1000
	// 1 byte == 8 bits
	// we use a float to avoid truncating on division
	byteToBits = 8.0
)

type MigrateAPIStorageOptions struct {
	migrate.ResourceOptions

	// Total network IO in megabits per second across all workers.
	// Zero means "no rate limit."
	bandwidth int
	// used to enforce bandwidth value
	limiter *tokenLimiter
}

// NewCmdMigrateAPIStorage implements a MigrateStorage command
func NewCmdMigrateAPIStorage(name, fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &MigrateAPIStorageOptions{
		ResourceOptions: migrate.ResourceOptions{
			Out:    out,
			ErrOut: errout,

			Unstructured: true,
			Include:      []string{"*"},
			DefaultExcludes: []schema.GroupResource{
				// openshift resources:
				{Resource: "appliedclusterresourcequotas"},
				{Resource: "imagestreamimages"}, {Resource: "imagestreamtags"}, {Resource: "imagestreammappings"}, {Resource: "imagestreamimports"},
				{Resource: "projectrequests"}, {Resource: "projects"},
				{Resource: "clusterrolebindings"}, {Resource: "rolebindings"},
				{Resource: "clusterroles"}, {Resource: "roles"},
				{Resource: "resourceaccessreviews"}, {Resource: "localresourceaccessreviews"}, {Resource: "subjectaccessreviews"},
				{Resource: "selfsubjectrulesreviews"}, {Resource: "localsubjectaccessreviews"},
				{Resource: "useridentitymappings"},
				{Resource: "podsecuritypolicyreviews"}, {Resource: "podsecuritypolicyselfsubjectreviews"}, {Resource: "podsecuritypolicysubjectreviews"},

				// kubernetes resources:
				{Resource: "bindings"},
				{Resource: "deploymentconfigrollbacks"},
				{Resource: "events"},
				{Resource: "componentstatuses"},
				{Resource: "replicationcontrollerdummies.extensions"},
				{Resource: "podtemplates"},
				{Resource: "selfsubjectaccessreviews", Group: "authorization.k8s.io"}, {Resource: "localsubjectaccessreviews", Group: "authorization.k8s.io"},
			},
			// Resources known to share the same storage
			OverlappingResources: []sets.String{
				// openshift resources:
				sets.NewString("deploymentconfigs.apps.openshift.io", "deploymentconfigs"),

				sets.NewString("clusterpolicies.authorization.openshift.io", "clusterpolicies"),
				sets.NewString("clusterpolicybindings.authorization.openshift.io", "clusterpolicybindings"),
				sets.NewString("clusterrolebindings.authorization.openshift.io", "clusterrolebindings"),
				sets.NewString("clusterroles.authorization.openshift.io", "clusterroles"),
				sets.NewString("localresourceaccessreviews.authorization.openshift.io", "localresourceaccessreviews"),
				sets.NewString("localsubjectaccessreviews.authorization.openshift.io", "localsubjectaccessreviews"),
				sets.NewString("policies.authorization.openshift.io", "policies"),
				sets.NewString("policybindings.authorization.openshift.io", "policybindings"),
				sets.NewString("resourceaccessreviews.authorization.openshift.io", "resourceaccessreviews"),
				sets.NewString("rolebindingrestrictions.authorization.openshift.io", "rolebindingrestrictions"),
				sets.NewString("rolebindings.authorization.openshift.io", "rolebindings"),
				sets.NewString("roles.authorization.openshift.io", "roles"),
				sets.NewString("selfsubjectrulesreviews.authorization.openshift.io", "selfsubjectrulesreviews"),
				sets.NewString("subjectaccessreviews.authorization.openshift.io", "subjectaccessreviews"),
				sets.NewString("subjectrulesreviews.authorization.openshift.io", "subjectrulesreviews"),

				sets.NewString("builds.build.openshift.io", "builds"),
				sets.NewString("buildconfigs.build.openshift.io", "buildconfigs"),

				sets.NewString("images.image.openshift.io", "images"),
				sets.NewString("imagesignatures.image.openshift.io", "imagesignatures"),
				sets.NewString("imagestreamimages.image.openshift.io", "imagestreamimages"),
				sets.NewString("imagestreamimports.image.openshift.io", "imagestreamimports"),
				sets.NewString("imagestreammappings.image.openshift.io", "imagestreammappings"),
				sets.NewString("imagestreams.image.openshift.io", "imagestreams"),
				sets.NewString("imagestreamtags.image.openshift.io", "imagestreamtags"),

				sets.NewString("clusternetworks.network.openshift.io", "clusternetworks"),
				sets.NewString("egressnetworkpolicies.network.openshift.io", "egressnetworkpolicies"),
				sets.NewString("hostsubnets.network.openshift.io", "hostsubnets"),
				sets.NewString("netnamespaces.network.openshift.io", "netnamespaces"),

				sets.NewString("oauthaccesstokens.oauth.openshift.io", "oauthaccesstokens"),
				sets.NewString("oauthauthorizetokens.oauth.openshift.io", "oauthauthorizetokens"),
				sets.NewString("oauthclientauthorizations.oauth.openshift.io", "oauthclientauthorizations"),
				sets.NewString("oauthclients.oauth.openshift.io", "oauthclients"),

				sets.NewString("projectrequests.project.openshift.io", "projectrequests"),
				sets.NewString("projects.project.openshift.io", "projects"),

				sets.NewString("appliedclusterresourcequotas.quota.openshift.io", "appliedclusterresourcequotas"),
				sets.NewString("clusterresourcequotas.quota.openshift.io", "clusterresourcequotas"),

				sets.NewString("routes.route.openshift.io", "routes"),

				sets.NewString("podsecuritypolicyreviews.security.openshift.io", "podsecuritypolicyreviews"),
				sets.NewString("podsecuritypolicyselfsubjectreviews.security.openshift.io", "podsecuritypolicyselfsubjectreviews"),
				sets.NewString("podsecuritypolicysubjectreviews.security.openshift.io", "podsecuritypolicysubjectreviews"),

				sets.NewString("processedtemplates.template.openshift.io", "processedtemplates"),
				sets.NewString("templates.template.openshift.io", "templates"),

				sets.NewString("groups.user.openshift.io", "groups"),
				sets.NewString("identities.user.openshift.io", "identities"),
				sets.NewString("useridentitymappings.user.openshift.io", "useridentitymappings"),
				sets.NewString("users.user.openshift.io", "users"),

				// kubernetes resources:
				sets.NewString("horizontalpodautoscalers.autoscaling", "horizontalpodautoscalers.extensions"),
				sets.NewString("jobs.batch", "jobs.extensions"),
			},
		},
	}
	cmd := &cobra.Command{
		Use:     name, // TODO do something useful here
		Short:   "Update the stored version of API objects",
		Long:    internalMigrateStorageLong,
		Example: fmt.Sprintf(internalMigrateStorageExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}
	options.ResourceOptions.Bind(cmd)
	flags := cmd.Flags()

	// opt-in to allow parallel execution since we know this command is goroutine safe
	// storage migration is IO bound so we make sure that we have enough workers to saturate the rate limiter
	options.Workers = 32 * runtime.NumCPU()
	// expose a flag to allow rate limiting the workers based on network bandwidth
	flags.IntVar(&options.bandwidth, "bandwidth", 10,
		"Average network bandwidth measured in megabits per second (Mbps) to use during storage migration.  Zero means no limit.  This flag is alpha and may change in the future.")

	// remove flags that do not make sense
	flags.MarkDeprecated("confirm", "storage migration does not support dry run, this flag is ignored")
	flags.MarkHidden("confirm")
	flags.MarkDeprecated("output", "storage migration does not support dry run, this flag is ignored")
	flags.MarkHidden("output")

	return cmd
}

func (o *MigrateAPIStorageOptions) Complete(f *clientcmd.Factory, c *cobra.Command, args []string) error {
	// force unset output, it does not make sense for this command
	if err := c.Flags().Set("output", ""); err != nil {
		return err
	}
	// force confirm, dry run does not make sense for this command
	o.Confirm = true

	o.ResourceOptions.SaveFn = o.save
	if err := o.ResourceOptions.Complete(f, c); err != nil {
		return err
	}

	// do not limit the builder as we handle throttling via our own limiter
	// thus the list calls that the builder makes are never rate limited
	// we estimate their IO usage in our call to o.limiter.take
	always := flowcontrol.NewFakeAlwaysRateLimiter()
	o.Builder.TransformRequests(
		func(req *rest.Request) {
			req.Throttle(always)
		},
	).
		// we use info.Client to directly fetch objects
		// this is just documentation, it does not really change anything because we fetch and decode lists of all objects regardless
		RequireObject(false)

	// bandwidth < 0 means error
	// bandwidth == 0 means "no limit", we use a nil check to minimize overhead
	// bandwidth > 0 means limit accordingly
	if o.bandwidth > 0 {
		o.limiter = newTokenLimiter(o.bandwidth, o.Workers)
	}

	return nil
}

func (o MigrateAPIStorageOptions) Validate() error {
	if o.bandwidth < 0 {
		return fmt.Errorf("invalid value %d for --bandwidth, must be at least 0", o.bandwidth)
	}
	return o.ResourceOptions.Validate()
}

func (o MigrateAPIStorageOptions) Run() error {
	return o.ResourceOptions.Visitor().Visit(migrate.AlwaysRequiresMigration)
}

// save invokes the API to alter an object. The reporter passed to this method is the same returned by
// the migration visitor method (for this type, transformAPIStorage). It should return an error
// if the input type cannot be saved. It returns migrate.ErrRecalculate if migration should be re-run
// on the provided object.
func (o *MigrateAPIStorageOptions) save(info *resource.Info, reporter migrate.Reporter) error {
	switch info.Object.(type) {
	// TODO: add any custom mutations necessary
	default:
		// load the body and save it back, without transformation to avoid losing fields
		r := info.Client.Get().
			Resource(info.Mapping.Resource).
			NamespaceIfScoped(info.Namespace, info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace).
			Name(info.Name)
		get := r.Do()
		data, err := get.Raw()
		if err != nil {
			// since we have an error, processing the body is safe because we are not going
			// to send it back to the server.  Thus we can safely call Result.Error().
			// This is required because we want to make sure we pass an errors.APIStatus so
			// that DefaultRetriable can correctly determine if the error is safe to retry.
			return migrate.DefaultRetriable(info, get.Error())
		}

		// a nil limiter means "no limit"
		if o.limiter != nil {
			// we have to wait until after the GET to determine how much data we will PUT
			// thus we need to double the amount to account for both operations
			// we also need to account for the initial list operation which is roughly another GET per object
			// thus we can amortize the cost of the list by adding another GET to our calculations
			// so we have 2 GETs + 1 PUT == 3 * size of data
			latency := o.limiter.take(3 * len(data))
			// mimic rest.Request.tryThrottle logging logic
			if latency > longThrottleLatency {
				glog.V(4).Infof("Throttling request took %v, request: %s:%s", latency, "GET", r.URL().String())
			}
		}

		update := info.Client.Put().
			Resource(info.Mapping.Resource).
			NamespaceIfScoped(info.Namespace, info.Mapping.Scope.Name() == meta.RESTScopeNameNamespace).
			Name(info.Name).Body(data).
			Do()
		if err := update.Error(); err != nil {
			return migrate.DefaultRetriable(info, err)
		}

		if oldObject, err := get.Get(); err == nil {
			info.Refresh(oldObject, true)
			oldVersion := info.ResourceVersion
			if object, err := update.Get(); err == nil {
				info.Refresh(object, true)
				if info.ResourceVersion == oldVersion {
					return migrate.ErrUnchanged
				}
			} else {
				glog.V(4).Infof("unable to calculate resource version: %v", err)
			}
		} else {
			glog.V(4).Infof("unable to calculate resource version: %v", err)
		}
	}
	return nil
}

type tokenLimiter struct {
	burst       int
	rateLimiter *rate.Limiter
	nowFunc     func() time.Time // for unit testing
}

// take n bytes from the rateLimiter, and sleep if needed
// return the length of the sleep
// is goroutine safe
func (t *tokenLimiter) take(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	// if n > burst, we need to split the reservation otherwise ReserveN will fail
	var extra time.Duration
	for ; n > t.burst; n -= t.burst {
		extra += t.getDuration(t.burst)
	}
	// calculate the remaining sleep time
	total := t.getDuration(n) + extra
	time.Sleep(total)
	return total
}

func (t *tokenLimiter) getDuration(n int) time.Duration {
	now := t.nowFunc()
	reservation := t.rateLimiter.ReserveN(now, n)
	if !reservation.OK() {
		// this should never happen but we do not want to hang a worker forever
		glog.Errorf("unable to get rate limited reservation, burst=%d n=%d limiter=%#v", t.burst, n, *t.rateLimiter)
		return time.Minute
	}
	return reservation.DelayFrom(now)
}

// rate limit based on bandwidth after conversion to bytes
// we use a burst value that scales linearly with the number of workers
func newTokenLimiter(bandwidth, workers int) *tokenLimiter {
	burst := 100 * kbToBytes * workers // 100 KB of burst per worker
	return &tokenLimiter{burst: burst, rateLimiter: rate.NewLimiter(rate.Limit(bandwidth*mbToKB*kbToBytes/byteToBits), burst), nowFunc: time.Now}
}

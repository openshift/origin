package storage

import (
	"fmt"
	"runtime"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"

	"github.com/openshift/origin/pkg/oc/cli/admin/migrate"
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

	// consider any network IO limit less than 30 Mbps to be "slow"
	// we use this as a heuristic to prevent ResourceExpired errors caused by paging
	slowBandwidth = 30
)

type MigrateAPIStorageOptions struct {
	migrate.ResourceOptions

	// Total network IO in megabits per second across all workers.
	// Zero means "no rate limit."
	bandwidth int
	// used to enforce bandwidth value
	limiter *tokenLimiter

	// unstructured client used to make no-op PUTs
	client dynamic.Interface
}

func NewMigrateAPIStorageOptions(streams genericclioptions.IOStreams) *MigrateAPIStorageOptions {
	return &MigrateAPIStorageOptions{

		bandwidth: 10,

		ResourceOptions: migrate.ResourceOptions{
			IOStreams: streams,

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
}

// NewCmdMigrateAPIStorage implements a MigrateStorage command
func NewCmdMigrateAPIStorage(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMigrateAPIStorageOptions(streams)
	cmd := &cobra.Command{
		Use:     name, // TODO do something useful here
		Short:   "Update the stored version of API objects",
		Long:    internalMigrateStorageLong,
		Example: fmt.Sprintf(internalMigrateStorageExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	o.ResourceOptions.Bind(cmd)

	// opt-in to allow parallel execution since we know this command is goroutine safe
	// storage migration is IO bound so we make sure that we have enough workers to saturate the rate limiter
	o.Workers = 32 * runtime.NumCPU()
	// expose a flag to allow rate limiting the workers based on network bandwidth
	cmd.Flags().IntVar(&o.bandwidth, "bandwidth", o.bandwidth,
		"Average network bandwidth measured in megabits per second (Mbps) to use during storage migration.  Zero means no limit.  This flag is alpha and may change in the future.")

	// remove flags that do not make sense
	cmd.Flags().MarkDeprecated("confirm", "storage migration does not support dry run, this flag is ignored")
	cmd.Flags().MarkHidden("confirm")
	cmd.Flags().MarkDeprecated("output", "storage migration does not support dry run, this flag is ignored")
	cmd.Flags().MarkHidden("output")

	return cmd
}

func (o *MigrateAPIStorageOptions) Complete(f kcmdutil.Factory, c *cobra.Command, args []string) error {
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
	)

	// bandwidth < 0 means error
	// bandwidth == 0 means "no limit", we use a nil check to minimize overhead
	// bandwidth > 0 means limit accordingly
	if o.bandwidth > 0 {
		o.limiter = newTokenLimiter(o.bandwidth, o.Workers)

		// disable paging when using a low rate limit to prevent ResourceExpired errors
		if o.bandwidth < slowBandwidth {
			o.Builder.RequestChunksOf(0)
		}
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	// We do not have a way to access the REST client that dynamic.NewForConfig uses
	// Thus we cannot use resource.NewClientWithOptions with our flowcontrol.NewFakeAlwaysRateLimiter
	// To avoid any possibility of rate limiting, use an absurdly high burst and QPS
	// We handle throttling via our own limiter
	clientConfigCopy := rest.CopyConfig(clientConfig)
	clientConfigCopy.Burst = 99999
	clientConfigCopy.QPS = 99999

	client, err := dynamic.NewForConfig(clientConfigCopy)
	if err != nil {
		return err
	}

	o.client = client

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
	switch oldObject := info.Object.(type) {
	case *unstructured.Unstructured:
		// a nil limiter means "no limit"
		if o.limiter != nil {
			// we rate limit after performing all operations to make us less sensitive to conflicts
			// use a defer to make sure we always rate limit even if the PUT fails
			defer o.rateLimit(oldObject)
		}

		// we are relying on unstructured types being lossless and unchanging
		// across a decode and encode round trip (otherwise this command will mutate data)
		newObject, err := o.client.
			Resource(info.Mapping.Resource).
			Namespace(info.Namespace).
			Update(oldObject)
		// storage migration is special in that all it needs to do is a no-op update to cause
		// the api server to migrate the object to the preferred version.  thus if we encounter
		// a conflict, we know that something updated the object and we no longer need to do
		// anything - if the object needed migration, the api server has already migrated it.
		if errors.IsConflict(err) {
			return migrate.ErrUnchanged
		}
		if err != nil {
			return migrate.DefaultRetriable(info, err)
		}
		if newObject.GetResourceVersion() == oldObject.GetResourceVersion() {
			return migrate.ErrUnchanged
		}
	default:
		return fmt.Errorf("invalid type %T passed to storage migration: %v", oldObject, oldObject)
	}
	return nil
}

func (o *MigrateAPIStorageOptions) rateLimit(oldObject *unstructured.Unstructured) {
	// we need to approximate how many bytes this object was on the wire
	// the simplest way to do that is to encode it back into bytes
	// this is wasteful but we are trying to artificially slow down the worker anyway
	var dataLen int
	if data, err := oldObject.MarshalJSON(); err != nil {
		// this should never happen
		glog.Errorf("failed to marshall %#v: %v", oldObject, err)
		// but in case it somehow does happen, assume the object was
		// larger than most objects so we still rate limit "enough"
		dataLen = 8192
	} else {
		dataLen = len(data)
	}

	// we need to account for the initial list operation which is roughly another PUT per object
	// thus we amortize the cost of the list by:
	// (1 LIST) / (N items) + 1 PUT == 2 PUTs == 2 * size of data

	// this is a slight overestimate since every retry attempt will still try to account for
	// the initial list operation.  this should not be an issue since retries are not that common
	// and the rate limiting is best effort anyway.  going slightly slower is acceptable.
	latency := o.limiter.take(2 * dataLen)
	// mimic rest.Request.tryThrottle logging logic
	if latency > longThrottleLatency {
		glog.V(4).Infof("Throttling request took %v, request: %s:%s", latency, "PUT", oldObject.GetSelfLink())
	}
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
		glog.Errorf("unable to get rate limited reservation, burst=%d n=%d", t.burst, n)
		return time.Minute
	}
	return reservation.DelayFrom(now)
}

// rate limit based on bandwidth after conversion to bytes
// we use a burst value that scales linearly with the number of workers
func newTokenLimiter(bandwidth, workers int) *tokenLimiter {
	burst := 100 * kbToBytes * workers // 100 KB of burst per worker
	return &tokenLimiter{burst: burst, rateLimiter: rate.NewLimiter(rate.Limit(bandwidth*mbToKB*kbToBytes)/byteToBits, burst), nowFunc: time.Now}
}

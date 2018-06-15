package etcd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/net/context"
)

var (
	internalMigrateTTLLong = templates.LongDesc(`
		Attach etcd keys to v3 leases to assist in migration from etcd v2

		This command updates keys to associate them with an etcd v3 lease. In etcd v2, keys have an
		innate TTL field which has been altered in the new schema. This can be used to set a timeout
		on keys migrated from the etcd v2 schema to etcd v3 is intended to be used after that upgrade
		is complete on events and access tokens. Keys that are already attached to a lease will be
		ignored. If another user modifies a key while this command is running you will need to re-run.
		
		Any resource impacted by this command will be removed from etcd after the lease-duration
		expires. Be VERY CAREFUL in which values you place to --ttl-keys-prefix, and ensure you
		have an up to date backup of your etcd database.`)

	internalMigrateTTLExample = templates.Examples(`
	  # Migrate TTLs for keys under /kubernetes.io/events to a 2 hour lease
	  %[1]s --etcd-address=localhost:2379 --ttl-keys-prefix=/kubernetes.io/events/ --lease-duration=2h`)
)

type MigrateTTLReferenceOptions struct {
	Out, ErrOut io.Writer

	etcdAddress   string
	ttlKeysPrefix string
	leaseDuration time.Duration
	certFile      string
	keyFile       string
	caFile        string
}

// NewCmdMigrateTTLs helps move etcd v2 TTL keys to etcd v3 lease keys.
func NewCmdMigrateTTLs(name, fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &MigrateTTLReferenceOptions{
		Out:    out,
		ErrOut: errout,
	}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s --etcd-address=HOST --ttl-keys-prefix=PATH", name),
		Short:   "Attach keys to etcd v3 leases to assist in etcd v2 migrations",
		Long:    internalMigrateTTLLong,
		Example: fmt.Sprintf(internalMigrateTTLExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Run())
		},
	}

	options.Bind(cmd.Flags())

	return cmd
}

func (o *MigrateTTLReferenceOptions) Bind(flag *pflag.FlagSet) {
	flag.StringVar(&o.etcdAddress, "etcd-address", "", "Etcd address")
	flag.StringVar(&o.ttlKeysPrefix, "ttl-keys-prefix", "", "Prefix for TTL keys")
	flag.DurationVar(&o.leaseDuration, "lease-duration", 0, "Lease duration (format: '2h', '120m', etc)")
	flag.StringVar(&o.certFile, "cert", "", "identify secure client using this TLS certificate file")
	flag.StringVar(&o.keyFile, "key", "", "identify secure client using this TLS key file")
	flag.StringVar(&o.caFile, "cacert", "", "verify certificates of TLS-enabled secure servers using this CA bundle")
}

func generateClientConfig(o *MigrateTTLReferenceOptions) (*clientv3.Config, error) {
	if o.etcdAddress == "" {
		return nil, fmt.Errorf("--etcd-address flag is required")
	}
	if o.ttlKeysPrefix == "" {
		return nil, fmt.Errorf("--ttl-keys-prefix flag is required")
	}
	if o.leaseDuration < time.Second {
		return nil, fmt.Errorf("--lease-duration must be at least one second")
	}

	c := &clientv3.Config{
		Endpoints:   []string{o.etcdAddress},
		DialTimeout: 5 * time.Second,
	}

	var cfgtls *transport.TLSInfo
	tlsinfo := transport.TLSInfo{}
	if o.certFile != "" {
		tlsinfo.CertFile = o.certFile
		cfgtls = &tlsinfo
	}

	if o.keyFile != "" {
		tlsinfo.KeyFile = o.keyFile
		cfgtls = &tlsinfo
	}

	if o.caFile != "" {
		tlsinfo.CAFile = o.caFile
		cfgtls = &tlsinfo
	}

	if cfgtls != nil {
		glog.V(4).Infof("TLS configuration: %#v", cfgtls)
		clientTLS, err := cfgtls.ClientConfig()
		if err != nil {
			return nil, err
		}
		c.TLS = clientTLS
	}
	return c, nil
}

func (o *MigrateTTLReferenceOptions) Run() error {
	c, err := generateClientConfig(o)
	if err != nil {
		return err
	}
	glog.V(4).Infof("Using client config: %#v", c)

	client, err := clientv3.New(*c)
	if err != nil {
		return fmt.Errorf("unable to create etcd client: %v", err)
	}

	// Make sure that ttlKeysPrefix is ended with "/" so that we only get children "directories".
	if !strings.HasSuffix(o.ttlKeysPrefix, "/") {
		o.ttlKeysPrefix += "/"
	}
	ctx := context.Background()

	objectsResp, err := client.KV.Get(ctx, o.ttlKeysPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("unable to get objects to attach to the lease: %v", err)
	}

	lease, err := client.Lease.Grant(ctx, int64(o.leaseDuration/time.Second))
	if err != nil {
		return fmt.Errorf("unable to create lease: %v", err)
	}
	fmt.Fprintf(o.Out, "info: Lease #%d with TTL %d created\n", lease.ID, lease.TTL)

	fmt.Fprintf(o.Out, "info: Attaching lease to %d entries\n", len(objectsResp.Kvs))
	errors := 0
	alreadyAttached := 0
	for _, kv := range objectsResp.Kvs {
		if kv.Lease != 0 {
			alreadyAttached++
		}
		txnResp, err := client.KV.Txn(ctx).If(
			clientv3.Compare(clientv3.ModRevision(string(kv.Key)), "=", kv.ModRevision),
		).Then(
			clientv3.OpPut(string(kv.Key), string(kv.Value), clientv3.WithLease(lease.ID)),
		).Commit()
		if err != nil {
			fmt.Fprintf(o.ErrOut, "error: Unable to attach lease to %s: %v\n", string(kv.Key), err)
			errors++
			continue
		}
		if !txnResp.Succeeded {
			fmt.Fprintf(o.ErrOut, "error: Unable to attach lease to %s: another client is writing to etcd. You must re-run this script.\n", string(kv.Key))
			errors++
		}
	}
	if alreadyAttached > 0 {
		fmt.Fprintf(o.Out, "info: Lease already attached to %d entries, no change made\n", alreadyAttached)
	}
	if errors != 0 {
		return fmt.Errorf("unable to complete migration, encountered %d errors", errors)
	}
	return nil
}

package sync

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	kerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/ldap"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/oc/lib/groupsync"
)

const PruneRecommendedName = "prune"

var (
	pruneLong = templates.LongDesc(`
    Prune OpenShift Groups referencing missing records on from an external provider.

    In order to prune OpenShift Group records using those from an external provider, determine which Groups you wish
    to prune. For instance, all or some groups may be selected from the current Groups stored in OpenShift that have
    been synced previously. Any combination of a literal whitelist, a whitelist file and a blacklist file is supported.
    The path to a sync configuration file that was used for syncing the groups in question is required in order to
    describe how data is requested from the external record store. Default behavior is to indicate all OpenShift groups
    for which the external record does not exist, to run the pruning process and commit the results, use the --confirm
    flag.`)

	pruneExamples = templates.Examples(`
    # Prune all orphaned groups
    %[1]s --sync-config=/path/to/ldap-sync-config.yaml --confirm

    # Prune all orphaned groups except the ones from the blacklist file
    %[1]s --blacklist=/path/to/blacklist.txt --sync-config=/path/to/ldap-sync-config.yaml --confirm

    # Prune all orphaned groups from a list of specific groups specified in a whitelist file
    %[1]s --whitelist=/path/to/whitelist.txt --sync-config=/path/to/ldap-sync-config.yaml --confirm

    # Prune all orphaned groups from a list of specific groups specified in a whitelist
    %[1]s groups/group_name groups/other_name --sync-config=/path/to/ldap-sync-config.yaml --confirm`)
)

type PruneOptions struct {
	// Config is the LDAP sync config read from file
	Config     *config.LDAPSyncConfig
	ConfigFile string

	// Whitelist are the names of OpenShift group or LDAP group UIDs to use for syncing
	Whitelist     []string
	WhitelistFile string

	// Blacklist are the names of OpenShift group or LDAP group UIDs to exclude
	Blacklist     []string
	BlacklistFile string

	// Confirm determines whether or not to write to OpenShift
	Confirm bool

	// GroupInterface is the interface used to interact with OpenShift Group objects
	GroupInterface userv1client.GroupInterface

	genericclioptions.IOStreams
}

func NewPruneOptions(streams genericclioptions.IOStreams) *PruneOptions {
	return &PruneOptions{
		Whitelist: []string{},
		IOStreams: streams,
	}
}

func NewCmdPrune(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewPruneOptions(streams)
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [WHITELIST] [--whitelist=WHITELIST-FILE] [--blacklist=BLACKLIST-FILE] --sync-config=CONFIG-SOURCE", name),
		Short:   "Remove old OpenShift groups referencing missing records on an external provider",
		Long:    pruneLong,
		Example: fmt.Sprintf(pruneExamples, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.WhitelistFile, "whitelist", o.WhitelistFile, "path to the group whitelist file")
	cmd.MarkFlagFilename("whitelist", "txt")
	cmd.Flags().StringVar(&o.BlacklistFile, "blacklist", o.BlacklistFile, "path to the group blacklist file")
	cmd.MarkFlagFilename("blacklist", "txt")
	// TODO(deads): enable this once we're able to support string slice elements that have commas
	// cmd.Flags().StringSliceVar(&o.Blacklist, "blacklist-group", o.Blacklist, "group to blacklist")
	cmd.Flags().StringVar(&o.ConfigFile, "sync-config", o.ConfigFile, "path to the sync config")
	cmd.MarkFlagFilename("sync-config", "yaml", "yml")
	cmd.Flags().BoolVar(&o.Confirm, "confirm", o.Confirm, "if true, modify OpenShift groups; if false, display groups")

	return cmd
}

func (o *PruneOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Config, err = decodeSyncConfigFromFile(o.ConfigFile)
	if err != nil {
		return err
	}

	o.Whitelist, err = buildOpenShiftGroupNameList(args, o.WhitelistFile, o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)
	if err != nil {
		return err
	}

	o.Blacklist, err = buildOpenShiftGroupNameList([]string{}, o.BlacklistFile, o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	userClient, err := userv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.GroupInterface = userClient.Groups()

	return nil
}

func (o *PruneOptions) Validate() error {
	results := ldap.ValidateLDAPSyncConfig(o.Config)
	if o.GroupInterface == nil {
		results.Errors = append(results.Errors, field.Required(field.NewPath("groupInterface"), ""))
	}
	// TODO(skuznets): pretty-print validation results
	if len(results.Errors) > 0 {
		return fmt.Errorf("validation of LDAP sync config failed: %v", results.Errors.ToAggregate())
	}
	return nil
}

// Run creates the GroupSyncer specified and runs it to sync groups
// the arguments are only here because its the only way to get the printer we need
func (o *PruneOptions) Run() error {
	bindPassword, err := config.ResolveStringValue(o.Config.BindPassword)
	if err != nil {
		return err
	}
	clientConfig, err := ldaputil.NewLDAPClientConfig(o.Config.URL, o.Config.BindDN, bindPassword, o.Config.CA, o.Config.Insecure)
	if err != nil {
		return fmt.Errorf("could not determine LDAP client configuration: %v", err)
	}

	pruneBuilder, err := buildPruneBuilder(clientConfig, o.Config)
	if err != nil {
		return err
	}

	// populate schema-independent pruner fields
	pruner := &syncgroups.LDAPGroupPruner{
		Host:        clientConfig.Host(),
		GroupClient: o.GroupInterface,
		DryRun:      !o.Confirm,

		Out: o.Out,
		Err: os.Stderr,
	}

	listerMapper, err := getOpenShiftGroupListerMapper(clientConfig.Host(), o)
	if err != nil {
		return err
	}
	pruner.GroupLister = listerMapper
	pruner.GroupNameMapper = listerMapper

	pruner.GroupDetector, err = pruneBuilder.GetGroupDetector()
	if err != nil {
		return err
	}

	// Now we run the pruner and report any errors
	pruneErrors := pruner.Prune()
	return kerrs.NewAggregate(pruneErrors)

}

func buildPruneBuilder(clientConfig ldapclient.Config, pruneConfig *config.LDAPSyncConfig) (PruneBuilder, error) {
	switch {
	case pruneConfig.RFC2307Config != nil:
		return &RFC2307Builder{ClientConfig: clientConfig, Config: pruneConfig.RFC2307Config}, nil
	case pruneConfig.ActiveDirectoryConfig != nil:
		return &ADBuilder{ClientConfig: clientConfig, Config: pruneConfig.ActiveDirectoryConfig}, nil
	case pruneConfig.AugmentedActiveDirectoryConfig != nil:
		return &AugmentedADBuilder{ClientConfig: clientConfig, Config: pruneConfig.AugmentedActiveDirectoryConfig}, nil
	default:
		return nil, errors.New("invalid sync config type")
	}
}

// The following getters ensure that PruneOptions satisfies the name restriction interfaces

func (o *PruneOptions) GetWhitelist() []string {
	return o.Whitelist
}

func (o *PruneOptions) GetBlacklist() []string {
	return o.Blacklist
}

func (o *PruneOptions) GetClient() userv1client.GroupInterface {
	return o.GroupInterface
}

func (o *PruneOptions) GetGroupNameMappings() map[string]string {
	return o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping
}

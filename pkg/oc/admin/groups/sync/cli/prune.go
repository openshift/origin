package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/ldap"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	userclientinternal "github.com/openshift/origin/pkg/user/generated/internalclientset"
	usertypedclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
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
	Config *config.LDAPSyncConfig

	// Whitelist are the names of OpenShift group or LDAP group UIDs to use for syncing
	Whitelist []string

	// Blacklist are the names of OpenShift group or LDAP group UIDs to exclude
	Blacklist []string

	// Confirm determines whether or not to write to OpenShift
	Confirm bool

	// GroupInterface is the interface used to interact with OpenShift Group objects
	GroupInterface usertypedclient.GroupInterface

	// Stderr is the writer to write warnings and errors to
	Stderr io.Writer

	// Out is the writer to write output to
	Out io.Writer
}

func NewPruneOptions() *PruneOptions {
	return &PruneOptions{
		Stderr:    os.Stderr,
		Whitelist: []string{},
	}
}

func NewCmdPrune(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewPruneOptions()
	options.Out = out

	whitelistFile := ""
	blacklistFile := ""
	configFile := ""

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [WHITELIST] [--whitelist=WHITELIST-FILE] [--blacklist=BLACKLIST-FILE] --sync-config=CONFIG-SOURCE", name),
		Short:   "Prune OpenShift groups referencing missing records on an external provider.",
		Long:    pruneLong,
		Example: fmt.Sprintf(pruneExamples, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(whitelistFile, blacklistFile, configFile, args, f); err != nil {
				cmdutil.CheckErr(cmdutil.UsageErrorf(c, err.Error()))
			}

			if err := options.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageErrorf(c, err.Error()))
			}

			err := options.Run(c, f)
			if err != nil {
				if aggregate, ok := err.(kerrs.Aggregate); ok {
					for _, err := range aggregate.Errors() {
						fmt.Printf("%s\n", err)
					}
					os.Exit(1)
				}
			}
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringVar(&whitelistFile, "whitelist", whitelistFile, "path to the group whitelist file")
	cmd.MarkFlagFilename("whitelist", "txt")
	cmd.Flags().StringVar(&blacklistFile, "blacklist", whitelistFile, "path to the group blacklist file")
	cmd.MarkFlagFilename("blacklist", "txt")
	// TODO(deads): enable this once we're able to support string slice elements that have commas
	// cmd.Flags().StringSliceVar(&options.Blacklist, "blacklist-group", options.Blacklist, "group to blacklist")

	cmd.Flags().StringVar(&configFile, "sync-config", configFile, "path to the sync config")
	cmd.MarkFlagFilename("sync-config", "yaml", "yml")

	cmd.Flags().BoolVar(&options.Confirm, "confirm", false, "if true, modify OpenShift groups; if false, display groups")

	return cmd
}

func (o *PruneOptions) Complete(whitelistFile, blacklistFile, configFile string, args []string, f *clientcmd.Factory) error {
	var err error

	o.Config, err = decodeSyncConfigFromFile(configFile)
	if err != nil {
		return err
	}

	o.Whitelist, err = buildOpenShiftGroupNameList(args, whitelistFile, o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)
	if err != nil {
		return err
	}

	o.Blacklist, err = buildOpenShiftGroupNameList([]string{}, blacklistFile, o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)
	if err != nil {
		return err
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	userClient, err := userclientinternal.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.GroupInterface = userClient.User().Groups()

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
func (o *PruneOptions) Run(cmd *cobra.Command, f *clientcmd.Factory) error {
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

func (o *PruneOptions) GetClient() usertypedclient.GroupInterface {
	return o.GroupInterface
}

func (o *PruneOptions) GetGroupNameMappings() map[string]string {
	return o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping
}

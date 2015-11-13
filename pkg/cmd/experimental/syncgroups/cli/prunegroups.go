package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kerrs "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	PruneGroupsRecommendedName = "prune-groups"

	pruneGroupsLong = `
Prune OpenShift Groups referencing missing records on from an external provider.

In order to prune OpenShift Group records using those from an external provider, determine which Groups you wish
to prune. For instance, all or some groups may be selected from the current Groups stored in OpenShift that have
been synced previously. Any combination of a literal whitelist, a whitelist file and a blacklist file is supported.
The path to a sync configuration file that was used for syncing the groups in question is required in order to 
describe how data is requested from the external record store. Default behavior is to prune all OpenShift groups 
for which the external record does not exist.
`
	pruneGroupsExamples = `  # Prune all orphaned groups 
  $ %[1]s --sync-config=/path/to/ldap-sync-config.yaml

  # Prune all orphaned groups except the ones from the blacklist file
  $ %[1]s --blacklist=/path/to/blacklist.txt --sync-config=/path/to/ldap-sync-config.yaml

  # Prune all orphaned groups from a list of specific groups specified in a whitelist file
  $ %[1]s --whitelist=/path/to/whitelist.txt --sync-config=/path/to/ldap-sync-config.yaml

  # Prune all orphaned groups from a list of specific groups specified in a whitelist
  $ %[1]s groups/group_name groups/other_name --sync-config=/path/to/ldap-sync-config.yaml
`
)

type PruneGroupsOptions struct {
	// Config is the LDAP sync config read from file
	Config *api.LDAPSyncConfig

	// Whitelist are the names of OpenShift group or LDAP group UIDs to use for syncing
	Whitelist []string

	// Blacklist are the names of OpenShift group or LDAP group UIDs to exclude
	Blacklist []string

	// Confirm determines whether or not to write to OpenShift
	Confirm bool

	// GroupsInterface is the interface used to interact with OpenShift Group objects
	GroupInterface osclient.GroupInterface

	// Stderr is the writer to write warnings and errors to
	Stderr io.Writer

	// Out is the writer to write output to
	Out io.Writer
}

func NewPruneGroupsOptions() *PruneGroupsOptions {
	return &PruneGroupsOptions{
		Stderr:    os.Stderr,
		Whitelist: []string{},
	}
}

func NewCmdPruneGroups(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewPruneGroupsOptions()
	options.Out = out

	whitelistFile := ""
	blacklistFile := ""
	configFile := ""

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [WHITELIST] [--whitelist=WHITELIST-FILE] [--blacklist=BLACKLIST-FILE] --sync-config=CONFIG-SOURCE", name),
		Short:   "Prune OpenShift groups referencing missing records on an external provider.",
		Long:    pruneGroupsLong,
		Example: fmt.Sprintf(pruneGroupsExamples, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(whitelistFile, blacklistFile, configFile, args, f); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(c, err.Error()))
			}

			if err := options.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(c, err.Error()))
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
	cmd.Flags().StringVar(&blacklistFile, "blacklist", whitelistFile, "path to the group blacklist file")
	// TODO(deads): enable this once we're able to support string slice elements that have commas
	// cmd.Flags().StringSliceVar(&options.Blacklist, "blacklist-group", options.Blacklist, "group to blacklist")
	cmd.Flags().StringVar(&configFile, "sync-config", configFile, "path to the sync config")
	cmd.Flags().BoolVar(&options.Confirm, "confirm", false, "if true, modify OpenShift groups; if false, display groups")

	return cmd
}

func (o *PruneGroupsOptions) Complete(whitelistFile, blacklistFile, configFile string, args []string, f *clientcmd.Factory) error {
	var err error
	o.Whitelist, err = buildNameList(GroupSyncSourceOpenShift, args, whitelistFile)
	if err != nil {
		return err
	}

	o.Blacklist, err = buildNameList(GroupSyncSourceOpenShift, []string{}, blacklistFile)
	if err != nil {
		return err
	}

	o.Config, err = decodeSyncConfigFromFile(configFile)
	if err != nil {
		return err
	}

	osClient, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.GroupInterface = osClient.Groups()

	return nil
}

func (o *PruneGroupsOptions) Validate() error {
	results := validation.ValidateLDAPSyncConfig(o.Config)
	if o.GroupInterface == nil {
		results.Errors = append(results.Errors, fmt.Errorf("an OpenShift group client is required"))
	}
	// TODO(skuznets): pretty-print validation results
	if len(results.Errors) > 0 {
		return fmt.Errorf("validation of LDAP sync config failed: %v", kerrs.NewAggregate([]error(results.Errors)))
	}
	return nil
}

// Run creates the GroupSyncer specified and runs it to sync groups
// the arguments are only here because its the only way to get the printer we need
func (o *PruneGroupsOptions) Run(cmd *cobra.Command, f *clientcmd.Factory) error {
	clientConfig, err := ldaputil.NewLDAPClientConfig(o.Config.URL, o.Config.BindDN, o.Config.BindPassword, o.Config.CA, o.Config.Insecure)
	if err != nil {
		return fmt.Errorf("could not determine LDAP client configuration: %v", err)
	}

	pruneBuilder, err := buildPruneBuilder(clientConfig, o.Config.RFC2307Config, o.Config.ActiveDirectoryConfig, o.Config.AugmentedActiveDirectoryConfig)
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

func buildPruneBuilder(clientConfig ldapclient.Config, rfc2307 *api.RFC2307Config, ad *api.ActiveDirectoryConfig, augmentedAD *api.AugmentedActiveDirectoryConfig) (PruneBuilder, error) {
	switch {
	case rfc2307 != nil:
		return &RFC2307Builder{ClientConfig: clientConfig, Config: rfc2307}, nil
	case ad != nil:
		return &ADBuilder{ClientConfig: clientConfig, Config: ad}, nil
	case augmentedAD != nil:
		return &AugmentedADBuilder{ClientConfig: clientConfig, Config: augmentedAD}, nil
	default:
		return nil, errors.New("invalid sync config type")
	}
}

// The following getters ensure that PruneGroupsOptions satisfies the name restriction interfaces

func (o *PruneGroupsOptions) GetWhitelist() []string {
	return o.Whitelist
}

func (o *PruneGroupsOptions) GetBlacklist() []string {
	return o.Blacklist
}

func (o *PruneGroupsOptions) GetClient() osclient.GroupInterface {
	return o.GroupInterface
}

func (o *PruneGroupsOptions) GetGroupNameMappings() map[string]string {
	return o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping
}

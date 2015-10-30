package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	kapi "k8s.io/kubernetes/pkg/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kerrs "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	kyaml "k8s.io/kubernetes/pkg/util/yaml"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	"github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	SyncGroupsRecommendedName = "sync-groups"

	syncGroupsLong = `
Sync OpenShift Groups with records from an external provider.

In order to sync OpenShift Group records with those from an external provider, determine which Groups you wish
to sync and where their records live. For instance, all or some groups may be selected from the current Groups
stored in OpenShift that have been synced previously, or similarly all or some groups may be selected from those 
stored on an LDAP server. The path to a sync configuration file is required in order to describe how data is 
requested from the external record store and migrated to OpenShift records. Default behavior is to sync all 
groups from the LDAP server returned by the LDAP query templates.
`
	syncGroupsExamples = `  # Sync all groups from an LDAP server
  $ %[1]s --sync-config=/path/to/ldap-sync-config.yaml

  # Sync all groups except the ones from the blacklist file from an LDAP server
  $ %[1]s --blacklist=/path/to/blacklist.txt --sync-config=/path/to/ldap-sync-config.yaml

  # Sync specific groups specified in a whitelist file with an LDAP server 
  $ %[1]s --whitelist=/path/to/whitelist.txt --sync-config=/path/to/sync-config.yaml

  # Sync all OpenShift Groups that have been synced previously with an LDAP server
  $ %[1]s --existing --sync-config=/path/to/ldap-sync-config.yaml

  # Sync specific OpenShift Groups if they have been synced previously with an LDAP server
  $ %[1]s groups/group1 groups/group2 groups/group3 --sync-config=/path/to/sync-config.yaml
`
)

// GroupSyncSource determines the source of the groups to be synced
type GroupSyncSource string

const (
	// GroupSyncSourceLDAP determines that the groups to be synced are determined from an LDAP record
	GroupSyncSourceLDAP GroupSyncSource = "ldap"
	// GroupSyncSourceOpenShift determines that the groups to be synced are determined from OpenShift records
	GroupSyncSourceOpenShift GroupSyncSource = "openshift"
)

var AllowedSourceTypes = []string{string(GroupSyncSourceLDAP), string(GroupSyncSourceOpenShift)}

func ValidateSource(source GroupSyncSource) bool {
	knownSources := sets.NewString(string(GroupSyncSourceLDAP), string(GroupSyncSourceOpenShift))
	return knownSources.Has(string(source))
}

type SyncGroupsOptions struct {
	// Source determines the source of the list of groups to sync
	Source GroupSyncSource

	// Config is the LDAP sync config read from file
	Config api.LDAPSyncConfig

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

func NewSyncGroupsOptions() *SyncGroupsOptions {
	return &SyncGroupsOptions{
		Stderr:    os.Stderr,
		Whitelist: []string{},
	}
}

func NewCmdSyncGroups(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewSyncGroupsOptions()
	options.Out = out

	typeArg := string(GroupSyncSourceLDAP)
	whitelistFile := ""
	blacklistFile := ""
	configFile := ""

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [SOURCE SCOPE WHITELIST --whitelist=WHITELIST-FILE] --sync-config=CONFIG-SOURCE", name),
		Short:   "Sync OpenShift groups with records from an external provider.",
		Long:    syncGroupsLong,
		Example: fmt.Sprintf(syncGroupsExamples, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(typeArg, whitelistFile, blacklistFile, configFile, args, f); err != nil {
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
	cmd.Flags().StringVar(&typeArg, "type", typeArg, "type of group used to locate LDAP group UIDs: "+strings.Join(AllowedSourceTypes, ","))
	cmd.Flags().BoolVar(&options.Confirm, "confirm", false, "if true, modify OpenShift groups; if false, display groups")
	cmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")

	return cmd
}

type SyncBuilder interface {
	GetGroupLister() (interfaces.LDAPGroupLister, error)
	GetGroupNameMapper() (interfaces.LDAPGroupNameMapper, error)
	GetUserNameMapper() (interfaces.LDAPUserNameMapper, error)
	GetGroupMemberExtractor() (interfaces.LDAPMemberExtractor, error)
}

func (o *SyncGroupsOptions) Complete(typeArg, whitelistFile, blacklistFile, configFile string, args []string, f *clientcmd.Factory) error {
	switch typeArg {
	case string(GroupSyncSourceLDAP):
		o.Source = GroupSyncSourceLDAP
	case string(GroupSyncSourceOpenShift):
		o.Source = GroupSyncSourceOpenShift

	default:
		return fmt.Errorf("unrecognized --type %q; allowed types %v", typeArg, strings.Join(AllowedSourceTypes, ","))
	}

	// if args are given, they are OpenShift Group names forming a whitelist
	if len(args) > 0 {
		o.Whitelist = append(o.Whitelist, args...)
	}

	// unpack whitelist file from source
	if len(whitelistFile) != 0 {
		whitelistData, err := readLines(whitelistFile)
		if err != nil {
			return err
		}
		o.Whitelist = append(o.Whitelist, whitelistData...)

		if o.Source == GroupSyncSourceOpenShift {
			o.Whitelist, err = openshiftGroupNamesOnlyList(o.Whitelist)
			if err != nil {
				return err
			}
		}
	}

	// unpack blacklist file from source
	if len(blacklistFile) != 0 {
		blacklistData, err := readLines(blacklistFile)
		if err != nil {
			return err
		}
		o.Blacklist = append(o.Blacklist, blacklistData...)

		if o.Source == GroupSyncSourceOpenShift {
			o.Blacklist, err = openshiftGroupNamesOnlyList(o.Blacklist)
			if err != nil {
				return err
			}
		}
	}

	yamlConfig, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("could not read file %s: %v", configFile, err)
	}
	jsonConfig, err := kyaml.ToJSON(yamlConfig)
	if err != nil {
		return fmt.Errorf("could not parse file %s: %v", configFile, err)
	}
	if err := configapilatest.Codec.DecodeInto(jsonConfig, &o.Config); err != nil {
		return err
	}

	if f != nil {
		osClient, _, err := f.Clients()
		if err != nil {
			return err
		}
		o.GroupInterface = osClient.Groups()
	}

	return nil
}

// openshiftGroupNamesOnlyBlacklist returns back a list that contains only the names of the groups.
// Since Group.Name cannot contain '/', the split is safe.  Any resource ref that is not a group
// is skipped.
func openshiftGroupNamesOnlyList(list []string) ([]string, error) {
	ret := []string{}
	errs := []error{}

	for _, curr := range list {
		tokens := strings.SplitN(curr, "/", 2)
		if len(tokens) == 1 {
			ret = append(ret, tokens[0])
			continue
		}

		if tokens[0] != "group" && tokens[0] != "groups" {
			errs = append(errs, fmt.Errorf("%q is not a valid OpenShift group", curr))
			continue
		}

		ret = append(ret, tokens[1])
	}

	if len(errs) > 0 {
		return nil, kerrs.NewAggregate(errs)
	}

	return ret, nil
}

// readLines interprets a file as plaintext and returns a string array of the lines of text in the file
func readLines(path string) ([]string, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not open file %s: %v", path, err)
	}
	rawLines := strings.Split(string(bytes), "\n")
	var trimmedLines []string
	for _, line := range rawLines {
		if len(strings.TrimSpace(line)) > 0 {
			trimmedLines = append(trimmedLines, strings.TrimSpace(line))
		}
	}
	return trimmedLines, nil
}

func (o *SyncGroupsOptions) Validate() error {
	if !ValidateSource(o.Source) {
		return fmt.Errorf("sync source must be one of the following: %v", strings.Join(AllowedSourceTypes, ","))
	}

	results := validation.ValidateLDAPSyncConfig(o.Config)
	// TODO(skuznets): pretty-print validation results
	if len(results.Errors) > 0 {
		return fmt.Errorf("validation of LDAP sync config failed: %v", kerrs.NewAggregate([]error(results.Errors)))
	}
	return nil
}

// Run creates the GroupSyncer specified and runs it to sync groups
// the arguments are only here because its the only way to get the printer we need
func (o *SyncGroupsOptions) Run(cmd *cobra.Command, f *clientcmd.Factory) error {
	clientConfig, err := ldaputil.NewLDAPClientConfig(o.Config.URL, o.Config.BindDN, o.Config.BindPassword, o.Config.CA, o.Config.Insecure)
	if err != nil {
		return fmt.Errorf("could not determine LDAP client configuration: %v", err)
	}

	var syncBuilder SyncBuilder
	switch {
	case o.Config.RFC2307Config != nil:
		syncBuilder = &RFC2307SyncBuilder{ClientConfig: clientConfig, Config: o.Config.RFC2307Config}

	case o.Config.ActiveDirectoryConfig != nil:
		syncBuilder = &ADSyncBuilder{ClientConfig: clientConfig, Config: o.Config.ActiveDirectoryConfig}

	case o.Config.AugmentedActiveDirectoryConfig != nil:
		syncBuilder = &AugmentedADSyncBuilder{ClientConfig: clientConfig, Config: o.Config.AugmentedActiveDirectoryConfig}

	default:
		return fmt.Errorf("invalid sync config type: %v", o.Config)
	}

	// populate schema-independent syncer fields
	syncer := &syncgroups.LDAPGroupSyncer{
		Host:        clientConfig.Host(),
		GroupClient: o.GroupInterface,
		DryRun:      !o.Confirm,

		Out: o.Out,
		Err: os.Stderr,
	}

	switch o.Source {
	case GroupSyncSourceOpenShift:
		// when your source of ldapGroupUIDs is from an openshift group, the mapping of ldapGroupUID to openshift group name is logically
		// pinned by the existing mapping.
		listerMapper, err := o.GetOpenShiftGroupListerMapper(clientConfig.Host())
		if err != nil {
			return err
		}
		syncer.GroupLister = listerMapper
		syncer.GroupNameMapper = listerMapper

	case GroupSyncSourceLDAP:
		syncer.GroupLister, err = o.GetLDAPGroupLister(syncBuilder)
		if err != nil {
			return err
		}
		syncer.GroupNameMapper, err = o.GetGroupNameMapper(syncBuilder)
		if err != nil {
			return err
		}

	default:
		return fmt.Errorf("invalid group source: %v", o.Source)
	}

	syncer.GroupMemberExtractor, err = syncBuilder.GetGroupMemberExtractor()
	if err != nil {
		return err
	}

	syncer.UserNameMapper, err = syncBuilder.GetUserNameMapper()
	if err != nil {
		return err
	}

	// Now we run the Syncer and report any errors
	openshiftGroups, syncErrors := syncer.Sync()
	if o.Confirm {
		return kerrs.NewAggregate(syncErrors)
	}

	list := &kapi.List{}
	for _, item := range openshiftGroups {
		list.Items = append(list.Items, item)
	}
	if err := f.Factory.PrintObject(cmd, list, o.Out); err != nil {
		return err
	}

	return kerrs.NewAggregate(syncErrors)

}

func (o *SyncGroupsOptions) GetOpenShiftGroupListerMapper(host string) (interfaces.LDAPGroupListerNameMapper, error) {
	if o.Source != GroupSyncSourceOpenShift {
		return nil, errors.New("openshift is not a valid group source for this config")
	}

	if len(o.Whitelist) != 0 {
		return syncgroups.NewOpenShiftGroupLister(o.Whitelist, o.Blacklist, host, o.GroupInterface), nil
	}

	return syncgroups.NewAllOpenShiftGroupLister(o.Blacklist, host, o.GroupInterface), nil
}

func (o *SyncGroupsOptions) GetLDAPGroupLister(syncBuilder SyncBuilder) (interfaces.LDAPGroupLister, error) {
	if o.Source != GroupSyncSourceLDAP {
		return nil, errors.New("ldap is not a valid group source for this config")
	}

	if len(o.Whitelist) != 0 {
		ldapWhitelist := syncgroups.NewLDAPWhitelistGroupLister(o.Whitelist)
		if len(o.Blacklist) == 0 {
			return ldapWhitelist, nil
		}
		return syncgroups.NewLDAPBlacklistGroupLister(o.Blacklist, ldapWhitelist), nil
	}

	syncLister, err := syncBuilder.GetGroupLister()
	if err != nil {
		return nil, err
	}
	if len(o.Blacklist) == 0 {
		return syncLister, nil
	}

	return syncgroups.NewLDAPBlacklistGroupLister(o.Blacklist, syncLister), nil
}

func (o *SyncGroupsOptions) GetGroupNameMapper(syncBuilder SyncBuilder) (interfaces.LDAPGroupNameMapper, error) {
	syncNameMapper, err := syncBuilder.GetGroupNameMapper()
	if err != nil {
		return nil, err
	}

	// if the mapping is specified, union the specified mapping with the default mapping.  The specified mapping is checked first
	if len(o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping) > 0 {
		userDefinedMapper := syncgroups.NewUserDefinedGroupNameMapper(o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)

		if syncNameMapper == nil {
			return userDefinedMapper, nil
		}

		return &syncgroups.UnionGroupNameMapper{GroupNameMappers: []interfaces.LDAPGroupNameMapper{userDefinedMapper, syncNameMapper}}, nil
	}

	return syncNameMapper, nil
}

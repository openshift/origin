package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	kerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/ldap"
	"github.com/openshift/origin/pkg/cmd/util/print"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync/interfaces"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync/syncerror"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	userclientinternal "github.com/openshift/origin/pkg/user/generated/internalclientset"
	usertypedclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

const SyncRecommendedName = "sync"

var (
	syncLong = templates.LongDesc(`
    Sync OpenShift Groups with records from an external provider.

    In order to sync OpenShift Group records with those from an external provider, determine which Groups you wish
    to sync and where their records live. For instance, all or some groups may be selected from the current Groups
    stored in OpenShift that have been synced previously, or similarly all or some groups may be selected from those
    stored on an LDAP server. The path to a sync configuration file is required in order to describe how data is
    requested from the external record store and migrated to OpenShift records. Default behavior is to do a dry-run
    without changing OpenShift records. Passing '--confirm' will sync all groups from the LDAP server returned by the
    LDAP query templates.`)

	syncExamples = templates.Examples(`
    # Sync all groups from an LDAP server
    %[1]s --sync-config=/path/to/ldap-sync-config.yaml --confirm

    # Sync all groups except the ones from the blacklist file from an LDAP server
    %[1]s --blacklist=/path/to/blacklist.txt --sync-config=/path/to/ldap-sync-config.yaml --confirm

    # Sync specific groups specified in a whitelist file with an LDAP server
    %[1]s --whitelist=/path/to/whitelist.txt --sync-config=/path/to/sync-config.yaml --confirm

    # Sync all OpenShift Groups that have been synced previously with an LDAP server
    %[1]s --type=openshift --sync-config=/path/to/ldap-sync-config.yaml --confirm

    # Sync specific OpenShift Groups if they have been synced previously with an LDAP server
    %[1]s groups/group1 groups/group2 groups/group3 --sync-config=/path/to/sync-config.yaml --confirm`)
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
	return sets.NewString(AllowedSourceTypes...).Has(string(source))
}

type SyncOptions struct {
	// Source determines the source of the list of groups to sync
	Source GroupSyncSource

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

// CreateErrorHandler creates an error handler for the LDAP sync job
func (o *SyncOptions) CreateErrorHandler() syncerror.Handler {
	components := []syncerror.Handler{}
	if o.Config.RFC2307Config != nil {
		if o.Config.RFC2307Config.TolerateMemberOutOfScopeErrors {
			components = append(components, syncerror.NewMemberLookupOutOfBoundsSuppressor(o.Stderr))
		}
		if o.Config.RFC2307Config.TolerateMemberNotFoundErrors {
			components = append(components, syncerror.NewMemberLookupMemberNotFoundSuppressor(o.Stderr))
		}
	}

	return syncerror.NewCompoundHandler(components...)
}

func NewSyncOptions() *SyncOptions {
	return &SyncOptions{
		Stderr:    os.Stderr,
		Whitelist: []string{},
	}
}

func NewCmdSync(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewSyncOptions()
	options.Out = out

	typeArg := string(GroupSyncSourceLDAP)
	whitelistFile := ""
	blacklistFile := ""
	configFile := ""

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [--type=TYPE] [WHITELIST] [--whitelist=WHITELIST-FILE] --sync-config=CONFIG-FILE [--confirm]", name),
		Short:   "Sync OpenShift groups with records from an external provider.",
		Long:    syncLong,
		Example: fmt.Sprintf(syncExamples, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := options.Complete(typeArg, whitelistFile, blacklistFile, configFile, args, f); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
			}

			if err := options.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
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
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringVar(&whitelistFile, "whitelist", whitelistFile, "path to the group whitelist file")
	cmd.MarkFlagFilename("whitelist", "txt")
	cmd.Flags().StringVar(&blacklistFile, "blacklist", whitelistFile, "path to the group blacklist file")
	cmd.MarkFlagFilename("blacklist", "txt")

	// TODO enable this we're able to support string slice elements that have commas
	// cmd.Flags().StringSliceVar(&options.Blacklist, "blacklist-group", options.Blacklist, "group to blacklist")

	cmd.Flags().StringVar(&configFile, "sync-config", configFile, "path to the sync config")
	cmd.MarkFlagFilename("sync-config", "yaml", "yml")

	cmd.Flags().StringVar(&typeArg, "type", typeArg, "which groups white- and blacklist entries refer to: "+strings.Join(AllowedSourceTypes, ","))
	cmd.Flags().BoolVar(&options.Confirm, "confirm", false, "if true, modify OpenShift groups; if false, display results of a dry-run")

	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().Lookup("output").DefValue = "yaml"
	cmd.Flags().Lookup("output").Value.Set("yaml")

	return cmd
}

func (o *SyncOptions) Complete(typeArg, whitelistFile, blacklistFile, configFile string, args []string, f *clientcmd.Factory) error {
	switch typeArg {
	case string(GroupSyncSourceLDAP):
		o.Source = GroupSyncSourceLDAP
	case string(GroupSyncSourceOpenShift):
		o.Source = GroupSyncSourceOpenShift
	default:
		return fmt.Errorf("unrecognized --type %q; allowed types %v", typeArg, strings.Join(AllowedSourceTypes, ","))
	}

	var err error

	o.Config, err = decodeSyncConfigFromFile(configFile)
	if err != nil {
		return err
	}

	if o.Source == GroupSyncSourceOpenShift {
		o.Whitelist, err = buildOpenShiftGroupNameList(args, whitelistFile, o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)
		if err != nil {
			return err
		}
		o.Blacklist, err = buildOpenShiftGroupNameList([]string{}, blacklistFile, o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping)
		if err != nil {
			return err
		}
	} else {
		o.Whitelist, err = buildNameList(args, whitelistFile)
		if err != nil {
			return err
		}
		o.Blacklist, err = buildNameList([]string{}, blacklistFile)
		if err != nil {
			return err
		}
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

// buildOpenShiftGroupNameList builds a list of OpenShift names from file and args
// nameMapping is used to override the OpenShift names built from file and args
func buildOpenShiftGroupNameList(args []string, file string, nameMapping map[string]string) ([]string, error) {
	rawList, err := buildNameList(args, file)
	if err != nil {
		return nil, err
	}

	namesList, err := openshiftGroupNamesOnlyList(rawList)
	if err != nil {
		return nil, err
	}

	// override items in namesList if present in mapping
	if len(nameMapping) > 0 {
		for i, name := range namesList {
			if nameOverride, ok := nameMapping[name]; ok {
				namesList[i] = nameOverride
			}
		}
	}

	return namesList, nil
}

// buildNameLists builds a list from file and args
func buildNameList(args []string, file string) ([]string, error) {
	var list []string
	if len(args) > 0 {
		list = append(list, args...)
	}

	// unpack file from source
	if len(file) != 0 {
		listData, err := readLines(file)
		if err != nil {
			return nil, err
		}
		list = append(list, listData...)
	}

	return list, nil
}

func decodeSyncConfigFromFile(configFile string) (*config.LDAPSyncConfig, error) {
	var config config.LDAPSyncConfig
	yamlConfig, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %v", configFile, err)
	}
	jsonConfig, err := kyaml.ToJSON(yamlConfig)
	if err != nil {
		return nil, fmt.Errorf("could not parse file %s: %v", configFile, err)
	}
	if err := runtime.DecodeInto(configapilatest.Codec, jsonConfig, &config); err != nil {
		return nil, fmt.Errorf("could not decode file into config: %v", err)
	}
	return &config, nil
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

func (o *SyncOptions) Validate() error {
	if !ValidateSource(o.Source) {
		return fmt.Errorf("sync source must be one of the following: %v", strings.Join(AllowedSourceTypes, ","))
	}

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
func (o *SyncOptions) Run(cmd *cobra.Command, f *clientcmd.Factory) error {
	bindPassword, err := config.ResolveStringValue(o.Config.BindPassword)
	if err != nil {
		return err
	}
	clientConfig, err := ldaputil.NewLDAPClientConfig(o.Config.URL, o.Config.BindDN, bindPassword, o.Config.CA, o.Config.Insecure)
	if err != nil {
		return fmt.Errorf("could not determine LDAP client configuration: %v", err)
	}

	errorHandler := o.CreateErrorHandler()

	syncBuilder, err := buildSyncBuilder(clientConfig, o.Config, errorHandler)
	if err != nil {
		return err
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
		listerMapper, err := getOpenShiftGroupListerMapper(clientConfig.Host(), o)
		if err != nil {
			return err
		}
		syncer.GroupLister = listerMapper
		syncer.GroupNameMapper = listerMapper

	case GroupSyncSourceLDAP:
		syncer.GroupLister, err = getLDAPGroupLister(syncBuilder, o)
		if err != nil {
			return err
		}
		syncer.GroupNameMapper, err = getGroupNameMapper(syncBuilder, o)
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
	fn := print.VersionedPrintObject(legacyscheme.Scheme, legacyscheme.Registry, kcmdutil.PrintObject, cmd, o.Out)
	if err := fn(list); err != nil {
		return err
	}

	return kerrs.NewAggregate(syncErrors)
}

func buildSyncBuilder(clientConfig ldapclient.Config, syncConfig *config.LDAPSyncConfig, errorHandler syncerror.Handler) (SyncBuilder, error) {
	switch {
	case syncConfig.RFC2307Config != nil:
		return &RFC2307Builder{ClientConfig: clientConfig, Config: syncConfig.RFC2307Config, ErrorHandler: errorHandler}, nil
	case syncConfig.ActiveDirectoryConfig != nil:
		return &ADBuilder{ClientConfig: clientConfig, Config: syncConfig.ActiveDirectoryConfig}, nil
	case syncConfig.AugmentedActiveDirectoryConfig != nil:
		return &AugmentedADBuilder{ClientConfig: clientConfig, Config: syncConfig.AugmentedActiveDirectoryConfig}, nil
	default:
		return nil, errors.New("invalid sync config type")
	}
}

func getOpenShiftGroupListerMapper(host string, info OpenShiftGroupNameRestrictions) (interfaces.LDAPGroupListerNameMapper, error) {
	if len(info.GetWhitelist()) != 0 {
		return syncgroups.NewOpenShiftGroupLister(info.GetWhitelist(), info.GetBlacklist(), host, info.GetClient()), nil
	} else {
		return syncgroups.NewAllOpenShiftGroupLister(info.GetBlacklist(), host, info.GetClient()), nil
	}
}

func getLDAPGroupLister(syncBuilder SyncBuilder, info GroupNameRestrictions) (interfaces.LDAPGroupLister, error) {
	if len(info.GetWhitelist()) != 0 {
		ldapWhitelist := syncgroups.NewLDAPWhitelistGroupLister(info.GetWhitelist())
		if len(info.GetBlacklist()) == 0 {
			return ldapWhitelist, nil
		}
		return syncgroups.NewLDAPBlacklistGroupLister(info.GetBlacklist(), ldapWhitelist), nil
	}

	syncLister, err := syncBuilder.GetGroupLister()
	if err != nil {
		return nil, err
	}
	if len(info.GetBlacklist()) == 0 {
		return syncLister, nil
	}

	return syncgroups.NewLDAPBlacklistGroupLister(info.GetBlacklist(), syncLister), nil
}

func getGroupNameMapper(syncBuilder SyncBuilder, info MappedNameRestrictions) (interfaces.LDAPGroupNameMapper, error) {
	syncNameMapper, err := syncBuilder.GetGroupNameMapper()
	if err != nil {
		return nil, err
	}

	// if the mapping is specified, union the specified mapping with the default mapping.  The specified mapping is checked first
	if len(info.GetGroupNameMappings()) > 0 {
		userDefinedMapper := syncgroups.NewUserDefinedGroupNameMapper(info.GetGroupNameMappings())
		if syncNameMapper == nil {
			return userDefinedMapper, nil
		}
		return &syncgroups.UnionGroupNameMapper{GroupNameMappers: []interfaces.LDAPGroupNameMapper{userDefinedMapper, syncNameMapper}}, nil
	}
	return syncNameMapper, nil
}

// The following getters ensure that SyncOptions satisfies the name restriction interfaces

func (o *SyncOptions) GetWhitelist() []string {
	return o.Whitelist
}

func (o *SyncOptions) GetBlacklist() []string {
	return o.Blacklist
}

func (o *SyncOptions) GetClient() usertypedclient.GroupInterface {
	return o.GroupInterface
}

func (o *SyncOptions) GetGroupNameMappings() map[string]string {
	return o.Config.LDAPGroupUIDToOpenShiftGroupNameMapping
}

package common

import (
	"fmt"
	"reflect"

	ign3types "github.com/coreos/ignition/v2/config/v3_4/types"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	"github.com/openshift/machine-config-operator/pkg/daemon/constants"
	"k8s.io/klog/v2"
)

// IsRenderedConfigReconcilable checks the new rendered config against the old
// rendered config to make sure that the only changes requested are ones we
// know how to do in-place. If we can reconcile, (nil) is returned. Otherwise,
// if we can't do it in place, the returned string value includes the
// rationale.
//
// We can only update machine configs that have changes to the files,
// directories, links, and systemd units sections of the included ignition
// config currently.
func IsRenderedConfigReconcilable(oldConfig, newConfig *mcfgv1.MachineConfig) error {
	return IsComponentConfigsReconcilable(oldConfig, []*mcfgv1.MachineConfig{newConfig})
}

// IsComponentConfigsReconcilable checks each individual component
// MachineConfig against the old rendered config. This is slightly more
// efficient because we only have to process the old config once. If an error
// occurs, the name of the component config will be included in the error
// message.
func IsComponentConfigsReconcilable(oldConfig *mcfgv1.MachineConfig, newConfigs []*mcfgv1.MachineConfig) error {
	// The parser will try to translate versions less than maxVersion to maxVersion, or output an err.
	// The ignition output in case of success will always have maxVersion
	oldIgn, err := ParseAndConvertConfig(oldConfig.Spec.Config.Raw)
	if err != nil {
		return fmt.Errorf("parsing old Ignition config from machineconfig %q failed: %w", oldConfig.Name, err)
	}

	// Go through each component config and determine if it is reconcilable.
	for _, newConfig := range newConfigs {
		if err := isReconcilable(oldIgn, oldConfig, newConfig); err != nil {
			return err
		}
	}

	return nil
}

// Parses a new config and determines if it is reconcilable.
func isReconcilable(oldIgn ign3types.Config, oldConfig, newConfig *mcfgv1.MachineConfig) error {
	newIgn, err := ParseAndConvertConfig(newConfig.Spec.Config.Raw)
	if err != nil {
		return fmt.Errorf("parsing new Ignition config from machineconfig %q failed: %w", newConfig.Name, err)
	}

	// Check if this is a generally valid Ignition Config
	if err := ValidateIgnition(newIgn); err != nil {
		return fmt.Errorf("validating new Ignition config from machineconfig %q failed: %w", newConfig.Name, err)
	}

	if err := isConfigReconcilable(oldIgn, newIgn, oldConfig, newConfig); err != nil {
		return fmt.Errorf("new machineconfig %q is not reconcilable against %q: %s", oldConfig.Name, newConfig.Name, err)
	}

	return nil
}

// Determines if a given config is reconcilable.
func isConfigReconcilable(oldIgn, newIgn ign3types.Config, oldConfig, newConfig *mcfgv1.MachineConfig) error {
	// Passwd section

	// we don't currently configure Groups in place. we don't configure Users except
	// for setting/updating SSHAuthorizedKeys for the only allowed user "core".
	// otherwise we can't fix it if something changed here.
	if !reflect.DeepEqual(oldIgn.Passwd, newIgn.Passwd) {
		if err := validatePasswdChanges(oldIgn, newIgn); err != nil {
			return fmt.Errorf("invalid passwd change(s): %w", err)
		}
	}

	// Kernel args

	// ignition now supports kernel args, but the MCO doesn't implement them yet
	if !reflect.DeepEqual(oldIgn.KernelArguments, newIgn.KernelArguments) {
		return fmt.Errorf("ignition kargs section contains changes")
	}

	// Storage section

	// we can only reconcile files right now. make sure the sections we can't
	// fix aren't changed.
	if !reflect.DeepEqual(oldIgn.Storage.Disks, newIgn.Storage.Disks) {
		return fmt.Errorf("ignition disks section contains changes")
	}
	if !reflect.DeepEqual(oldIgn.Storage.Filesystems, newIgn.Storage.Filesystems) {
		return fmt.Errorf("ignition filesystems section contains changes")
	}
	if !reflect.DeepEqual(oldIgn.Storage.Raid, newIgn.Storage.Raid) {
		return fmt.Errorf("ignition raid section contains changes")
	}
	if !reflect.DeepEqual(oldIgn.Storage.Directories, newIgn.Storage.Directories) {
		return fmt.Errorf("ignition directories section contains changes")
	}
	if !reflect.DeepEqual(oldIgn.Storage.Links, newIgn.Storage.Links) {
		// This means links have been added, as opposed as being removed as it happened with
		// https://bugzilla.redhat.com/show_bug.cgi?id=1677198. This doesn't really change behavior
		// since we still don't support links but we allow old MC to remove links when upgrading.
		if len(newIgn.Storage.Links) != 0 {
			return fmt.Errorf("ignition links section contains changes")
		}
	}

	// Special case files append: if the new config wants us to append, then we
	// have to force a reprovision since it's not idempotent
	for _, f := range newIgn.Storage.Files {
		if len(f.Append) > 0 {
			return fmt.Errorf("ignition file %v includes append", f.Path)
		}
		// We also disallow writing some special files
		if f.Path == constants.MachineConfigDaemonForceFile {
			return fmt.Errorf("cannot create %s via Ignition", f.Path)
		}
	}

	// FIPS section
	// We do not allow update to FIPS for a running cluster, so any changes here will be an error.
	// We do a naive check here for reusability. The MCD will continue to do a
	// more in-depth check, taking the nodes filesystem into consideration.
	if oldConfig.Spec.FIPS != newConfig.Spec.FIPS {
		return fmt.Errorf("detected change to FIPS flag; refusing to modify FIPS on a running cluster")
	}

	return nil
}

// verifyUserFields returns nil for the user Name = "core" if 1 or more SSHKeys exist for
// this user or if a password exists for this user and if all other fields in User are empty.
// Otherwise, an error will be returned and the proposed config will not be reconcilable.
// At this time we do not support non-"core" users or any changes to the "core" user
// outside of SSHAuthorizedKeys and passwordHash.
func verifyUserFields(pwdUser ign3types.PasswdUser) error {
	emptyUser := ign3types.PasswdUser{}
	tempUser := pwdUser
	if tempUser.Name == constants.CoreUserName && ((tempUser.PasswordHash) != nil || len(tempUser.SSHAuthorizedKeys) >= 1) {
		tempUser.Name = ""
		tempUser.SSHAuthorizedKeys = nil
		tempUser.PasswordHash = nil
		if !reflect.DeepEqual(emptyUser, tempUser) {
			return fmt.Errorf("SSH keys and password hash are not reconcilable")
		}
		klog.Info("SSH Keys reconcilable")
	} else {
		return fmt.Errorf("ignition passwd user section contains unsupported changes: user must be core and have 1 or more sshKeys")
	}
	return nil
}

// Validates that changes to the Passwd section of the Ignition config are
// reconcilable.
func validatePasswdChanges(oldIgn, newIgn ign3types.Config) error {
	if !reflect.DeepEqual(oldIgn.Passwd.Groups, newIgn.Passwd.Groups) {
		return fmt.Errorf("ignition Passwd Groups section contains changes")
	}

	if !reflect.DeepEqual(oldIgn.Passwd.Users, newIgn.Passwd.Users) {
		// there is an update to Users, we must verify that it is ONLY making an acceptable
		// change to the SSHAuthorizedKeys for the user "core"
		for _, user := range newIgn.Passwd.Users {
			if user.Name != constants.CoreUserName {
				return fmt.Errorf("ignition passwd user section contains unsupported changes: non-core user")
			}
		}
		// We don't want to panic if the "new" users is empty, and it's still reconcilable because the absence of a user here does not mean "remove the user from the system"
		if len(newIgn.Passwd.Users) != 0 {
			klog.Infof("user data to be verified before ssh update: %v", newIgn.Passwd.Users[len(newIgn.Passwd.Users)-1])
			if err := verifyUserFields(newIgn.Passwd.Users[len(newIgn.Passwd.Users)-1]); err != nil {
				return err
			}
		}
	}

	return nil
}

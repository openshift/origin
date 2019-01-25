package calls_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
)

func TestRole(t *testing.T) {
	var (
		rolesNone []string
		roleX     = []string{"x"}
	)
	for ti, tc := range []struct {
		call  *scheduler.Call
		roles []string
	}{
		{calls.Revive(), rolesNone},
		{calls.Suppress(), rolesNone},

		{calls.ReviveWith(nil), rolesNone},
		{calls.SuppressWith(nil), rolesNone},

		{calls.ReviveWith(roleX), roleX},
		{calls.SuppressWith(roleX), roleX},
	} {
		roles, hasRole := func() ([]string, bool) {
			switch tc.call.Type {
			case scheduler.Call_SUPPRESS:
				return tc.call.GetSuppress().GetRoles(), len(tc.call.GetSuppress().GetRoles()) > 0
			case scheduler.Call_REVIVE:
				return tc.call.GetRevive().GetRoles(), len(tc.call.GetRevive().GetRoles()) > 0
			default:
				panic(fmt.Sprintf("test case %d failed: unsupported call type: %v", ti, tc.call.Type))
			}
		}()
		if hasRole != (len(tc.roles) > 0) {
			if hasRole {
				t.Errorf("test case %d failed: expected no role instead of %q", ti, roles)
			} else {
				t.Errorf("test case %d failed: expected role %q instead of no role", ti, tc.roles)
			}
		}
		if hasRole && !reflect.DeepEqual(tc.roles, roles) {
			t.Errorf("test case %d failed: expected role %q instead of %q", ti, tc.roles, roles)
		}
	}
}

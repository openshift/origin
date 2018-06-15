package syncgroups

import (
	"errors"
	"fmt"
	"io/ioutil"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/oc/admin/groups/sync/interfaces"
	userfakeclient "github.com/openshift/origin/pkg/user/generated/internalclientset/fake"
)

func TestGoodPrune(t *testing.T) {
	testGroupPruner, tc := newTestPruner()
	errs := testGroupPruner.Prune()
	for _, err := range errs {
		t.Errorf("unexpected prune error: %v", err)
	}

	checkClientForDeletedGroups(tc, []string{"os" + Group2UID}, t)
}

func TestListFailsForPrune(t *testing.T) {
	testGroupPruner, tc := newTestPruner()
	listErr := errors.New("error during listing")
	testGroupPruner.GroupLister.(*TestGroupLister).err = listErr

	errs := testGroupPruner.Prune()
	if len(errs) != 1 {
		t.Errorf("unexpected prune errors: %v", errs)

	} else if errs[0] != listErr {
		t.Errorf("incorrect prune error:\n\twanted:\n\t%v\n\tgot:\n\t%v\n", listErr, errs[0])
	}

	deletedGroups := extractDeletedGroups(tc)
	if len(deletedGroups) != 0 {
		t.Errorf("expected no groups to be deleted, got: %v", deletedGroups)
	}
}

// TestLocateFails tests that a failure locating a group does not fail the entire prune job, or cause it to prune that group
func TestLocateFails(t *testing.T) {
	testGroupPruner, tc := newTestPruner()
	locateErr := fmt.Errorf("error during location for group: %s", Group1UID)
	testGroupPruner.GroupDetector.(*TestGroupDetector).SourceOfErrors[Group1UID] = locateErr

	errs := testGroupPruner.Prune()
	if len(errs) != 1 {
		t.Errorf("unexpected prune errors: %v", errs)

	} else if errs[0] != locateErr {
		t.Errorf("incorrect prune error:\n\twanted:\n\t%v\n\tgot:\n\t%v\n", locateErr, errs[0])
	}

	checkClientForDeletedGroups(tc, []string{"os" + Group2UID}, t)
}

// TestDeleteFails tests that a prune failure doesn't fail the entire job
func TestDeleteFails(t *testing.T) {
	testGroupPruner, tc := newTestPruner()
	deleteErr := fmt.Errorf("failed to delete group: %s", "os"+Group1UID)
	tc.PrependReactor("delete", "groups", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		deleteAction := action.(clientgotesting.DeleteAction)
		if deleteAction.GetName() == "os"+Group1UID {
			return true, nil, deleteErr
		}
		return false, nil, nil
	})
	testGroupPruner.GroupDetector.(*TestGroupDetector).SourceOfTruth[Group1UID] = false

	errs := testGroupPruner.Prune()
	if len(errs) != 1 {
		t.Errorf("unexpected prune error: %v", errs)

	} else if errs[0] != deleteErr {
		t.Errorf("incorrect prune error:\n\twanted:\n\t%v\n\tgot:\n\t%v\n", deleteErr, errs[0])
	}

	// although the first delete will fail, the event is still registered
	// we are interested in seeing that both delete actions happen
	checkClientForDeletedGroups(tc, []string{"os" + Group1UID, "os" + Group2UID}, t)
}

func checkClientForDeletedGroups(tc *userfakeclient.Clientset, expectedGroups []string, t *testing.T) {
	actualGroups := sets.NewString(extractDeletedGroups(tc)...)
	wantedGroups := sets.NewString(expectedGroups...)

	if !actualGroups.Equal(wantedGroups) {
		t.Errorf("did not delete correct groups:\n\twanted:\n\t%v\n\tgot:\n\t%v\n", wantedGroups, actualGroups)
	}
}

func extractDeletedGroups(tc *userfakeclient.Clientset) []string {
	ret := []string{}
	for _, genericAction := range tc.Actions() {
		switch action := genericAction.(type) {
		case clientgotesting.DeleteAction:
			ret = append(ret, action.GetName())
		}
	}

	return ret
}

func newTestPruner() (*LDAPGroupPruner, *userfakeclient.Clientset) {
	tc := userfakeclient.NewSimpleClientset()
	tc.PrependReactor("delete", "groups", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
	})

	return &LDAPGroupPruner{
		GroupLister:     newTestLister(),
		GroupDetector:   newTestGroupDetector(),
		GroupNameMapper: newTestGroupNameMapper(),
		GroupClient:     tc.User().Groups(),
		Host:            newTestHost(),
		Out:             ioutil.Discard,
		Err:             ioutil.Discard,
	}, tc

}

func newTestGroupDetector() interfaces.LDAPGroupDetector {
	return &TestGroupDetector{
		SourceOfTruth: map[string]bool{
			Group1UID: true,
			Group2UID: false,
			Group3UID: true,
		},
		SourceOfErrors: map[string]error{
			Group1UID: nil,
			Group2UID: nil,
			Group3UID: nil,
		},
	}
}

var _ interfaces.LDAPGroupDetector = &TestGroupDetector{}

type TestGroupDetector struct {
	SourceOfTruth  map[string]bool
	SourceOfErrors map[string]error
}

func (l *TestGroupDetector) Exists(ldapGroupUID string) (bool, error) {
	status, exists := l.SourceOfTruth[ldapGroupUID]
	return status && exists, l.SourceOfErrors[ldapGroupUID]
}

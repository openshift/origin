package bootstrappolicy

import (
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
)

const osClusterRoleAggregationPrefix = "system:openshift:"

// this map must be manually kept up to date as we make changes to aggregation
// we hard code this data with no constants because we cannot change the underlying values
var expectedAggregationMap = map[string]sets.String{
	"admin": sets.NewString("system:openshift:aggregate-to-admin", "system:aggregate-to-admin"),
	"edit":  sets.NewString("system:openshift:aggregate-to-edit", "system:aggregate-to-edit"),
	"view":  sets.NewString("system:openshift:aggregate-to-view", "system:aggregate-to-view"),
}

func TestPolicyAggregation(t *testing.T) {
	policyData := Policy()
	clusterRoles := policyData.ClusterRoles
	clusterRolesToAggregate := policyData.ClusterRolesToAggregate

	// do some basic sanity checks

	if len(clusterRoles) == 0 || len(clusterRolesToAggregate) == 0 {
		t.Fatalf("invalid policy data:\n%#v\n%#v", clusterRoles, clusterRolesToAggregate)
	}

	// make sure we have no duplicate new names
	shouldHaveAggregationRuleSet := sets.NewString()
	newNameOfClusterRoleSet := sets.NewString()
	for oldName, newName := range clusterRolesToAggregate {
		if newNameOfClusterRoleSet.Has(newName) {
			t.Errorf("duplicate value %s for key %s", newName, oldName)
		}
		shouldHaveAggregationRuleSet.Insert(oldName)
		newNameOfClusterRoleSet.Insert(newName)
	}

	// now the actual test

	hasAggregationRuleSet := sets.NewString()
	aggregationMap := map[string]sets.String{} // map of cluster role name to all cluster roles that aggregate into it
	for i := range clusterRoles {
		cr := clusterRoles[i]
		if cr.AggregationRule == nil {
			continue
		}

		hasAggregationRuleSet.Insert(cr.Name)

		// insert this cluster role into aggregationMap with all of cluster roles that aggregate into it
		for j := range cr.AggregationRule.ClusterRoleSelectors {
			labelSelector := cr.AggregationRule.ClusterRoleSelectors[j]
			selector, err := v1.LabelSelectorAsSelector(&labelSelector)
			if err != nil {
				// should never happen
				t.Errorf("invalid label selector %#v   at index %d for cluster role %s: %v", labelSelector, j, cr.Name, err)
				continue
			}

			// iterate over all cluster roles again to see what matches the aggregation rule selector
			for k := range clusterRoles {
				cr2 := clusterRoles[k]
				if selector.Matches(labels.Set(cr2.Labels)) {
					if cr.Name == cr2.Name {
						// sanity check, should never happen
						t.Errorf("invalid self match %s", cr.Name)
						continue
					}
					if aggregationMap[cr.Name] == nil {
						aggregationMap[cr.Name] = sets.NewString()
					}
					if aggregationMap[cr.Name].Has(cr2.Name) {
						// sanity check, should never happen
						t.Errorf("invalid duplicate entry %s for %s -> %s", cr2.Name, cr.Name, aggregationMap[cr.Name].List())
						continue
					}
					aggregationMap[cr.Name].Insert(cr2.Name)
				}
			}
		}
	}

	// check that we are actually aggregating the cluster roles that we said we would
	if !shouldHaveAggregationRuleSet.Equal(hasAggregationRuleSet) {
		missingClusterRoles := shouldHaveAggregationRuleSet.Difference(hasAggregationRuleSet).List()
		extraClusterRoles := hasAggregationRuleSet.Difference(shouldHaveAggregationRuleSet).List()
		t.Errorf("missing aggregation cluster roles = %s\nextra aggregation cluster roles = %s", missingClusterRoles, extraClusterRoles)
	}

	// check that the new name of the aggregated cluster role is valid
	// aggregationMap is effectively old name -> all possible valid new names
	// clusterRolesToAggregate is old name -> new name
	for parentClusterRole, childClusterRoles := range aggregationMap {
		newNameOfClusterRole, ok := clusterRolesToAggregate[parentClusterRole]
		// we would have caught this earlier but does not hurt to check again
		if !ok {
			t.Errorf("cluster role %s in missing from %#v", parentClusterRole, clusterRolesToAggregate)
			continue
		}
		if !childClusterRoles.Has(newNameOfClusterRole) {
			t.Errorf("cluster role %s -> %s is missing the new name cluster role %s", parentClusterRole, childClusterRoles.List(), newNameOfClusterRole)
		}
		if !strings.HasPrefix(newNameOfClusterRole, osClusterRoleAggregationPrefix) {
			t.Errorf("invalid new name %s for old cluster role %s -> %s", newNameOfClusterRole, parentClusterRole, childClusterRoles.List())
		}
	}

	// at this point we have checked everything except to make sure that no cluster role has added a label
	// that causes it to start aggregating into an existing cluster role that already had an aggregation rule
	if !reflect.DeepEqual(expectedAggregationMap, aggregationMap) {
		t.Errorf("unexpected data in aggregationMap:\n%s", diff.ObjectDiff(expectedAggregationMap, aggregationMap))
	}
}

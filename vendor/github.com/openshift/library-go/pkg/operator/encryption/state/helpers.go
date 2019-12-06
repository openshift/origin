package state

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MigratedFor returns whether all given resources are marked as migrated in the given key.
// It returns missing GRs and a reason if that's not the case.
func MigratedFor(grs []schema.GroupResource, km KeyState) (ok bool, missing []schema.GroupResource, reason string) {
	var missingStrings []string
	for _, gr := range grs {
		found := false
		for _, mgr := range km.Migrated.Resources {
			if mgr == gr {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, gr)
			missingStrings = append(missingStrings, gr.String())
		}
	}

	if len(missing) > 0 {
		return false, missing, fmt.Sprintf("key ID %s misses resource %s among migrated resources", km.Key.Name, strings.Join(missingStrings, ","))
	}

	return true, nil, ""
}

// KeysWithPotentiallyPersistedDataAndNextReadKey returns the minimal, recent secrets which have migrated all given GRs.
func KeysWithPotentiallyPersistedDataAndNextReadKey(grs []schema.GroupResource, recentFirstSortedKeys []KeyState) []KeyState {
	for i, k := range recentFirstSortedKeys {
		if allMigrated, missing, _ := MigratedFor(grs, k); allMigrated {
			if i+1 < len(recentFirstSortedKeys) {
				return recentFirstSortedKeys[:i+2]
			} else {
				return recentFirstSortedKeys[:i+1]
			}
		} else {
			// continue with keys we haven't found a migration key for yet
			grs = missing
		}
	}
	return recentFirstSortedKeys
}

func SortRecentFirst(unsorted []KeyState) []KeyState {
	ret := make([]KeyState, len(unsorted))
	copy(ret, unsorted)
	sort.Slice(ret, func(i, j int) bool {
		// it is fine to ignore the validKeyID bool here because we filtered out invalid secrets in the loop above
		iKeyID, _ := NameToKeyID(ret[i].Key.Name)
		jKeyID, _ := NameToKeyID(ret[j].Key.Name)
		return iKeyID > jKeyID
	})
	return ret
}

func NameToKeyID(name string) (uint64, bool) {
	lastIdx := strings.LastIndex(name, "-")
	idString := name
	if lastIdx >= 0 {
		idString = name[lastIdx+1:] // this can never overflow since str[-1+1:] is
	}
	id, err := strconv.ParseUint(idString, 10, 0)
	return id, err == nil
}

func EqualKeyAndEqualID(s1, s2 *KeyState) bool {
	if s1.Mode != s2.Mode || s1.Key.Secret != s2.Key.Secret {
		return false
	}

	id1, valid1 := NameToKeyID(s1.Key.Name)
	id2, valid2 := NameToKeyID(s2.Key.Name)
	return valid1 && valid2 && id1 == id2
}

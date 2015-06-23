package assets

import (
	"fmt"
	"sort"
)

// JoinAssetFuncs returns an asset function that delegates to each of the provided asset functions in turn.
// The functions are assumed to provide non-overlapping assets
func JoinAssetFuncs(funcs ...AssetFunc) AssetFunc {
	return func(name string) ([]byte, error) {
		for _, f := range funcs {
			if data, err := f(name); err == nil {
				return data, nil
			}
		}
		return nil, fmt.Errorf("Asset %s not found", name)
	}
}

// JoinAssetDirFuncs returns an asset dir function that delegates to the provided asset dir functions.
// The functions are assumed to provide non-overlapping assets
func JoinAssetDirFuncs(funcs ...AssetDirFunc) AssetDirFunc {
	roots := []string{}
	for _, f := range funcs {
		if localRoots, err := f(""); err == nil {
			roots = append(roots, localRoots...)
		}
	}
	sort.Strings(roots)

	return func(name string) ([]string, error) {
		if name == "" {
			return roots, nil
		}
		for _, f := range funcs {
			if dirs, err := f(name); err == nil {
				return dirs, nil
			}
		}
		return nil, fmt.Errorf("Asset %s not found", name)
	}
}

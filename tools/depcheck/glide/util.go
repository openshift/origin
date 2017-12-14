package glide

import (
	"fmt"
)

// MissingImports receives a LockFile and a YamlFile and returns a slice of
// imports not contained in the YamlFile. Dependencies with a non-empty "repo" field
// are skipped, as they are assumed to contain forked dependencies.
func MissingImports(lockfile *LockFile, yamlfile *YamlFile) (YamlFileImportList, []string, error) {
	if lockfile == nil || yamlfile == nil {
		return nil, []string{}, fmt.Errorf("both a lockfile and a yamlfile are required")
	}

	warnings := []string{}
	newImports := []*YamlFileImport{}

	for _, lockDep := range lockfile.Imports {
		if len(lockDep.Repo) > 0 {
			warnings = append(warnings, fmt.Sprintf("info: skipping package with \"repo\" field: %s", lockDep.Name))
			continue
		}

		yamlDepExists := false
		for _, yamlDep := range yamlfile.Imports {
			if yamlDep.Package == lockDep.Name {
				yamlDepExists = true
				break
			}
		}
		if yamlDepExists {
			continue
		}

		newImports = append(newImports, &YamlFileImport{
			Package: lockDep.Name,
			Version: lockDep.Version,
		})
	}

	return newImports, warnings, nil
}

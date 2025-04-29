package check

// AllChecks returns a set of all types and checks to be performed on the catalog image.
func AllChecks() Checks {
	return Checks{
		ImageChecks:      AllImageChecks(),
		FilesystemChecks: AllFilesystemChecks(),
		// TODO: Enable those tests when community-operator-index and certified-operator-index
		// are able to pass on them
		// CatalogChecks:    AllCatalogChecks(),
	}
}

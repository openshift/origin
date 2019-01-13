package customresourcevalidation

// RequireNameCluster is a name validation function that requires the name to be cluster.  It's handy for config.openshift.io types.
func RequireNameCluster(name string, prefix bool) []string {
	if name != "cluster" {
		return []string{"must be cluster"}
	}
	return nil
}

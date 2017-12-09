package util

var (
	AdminKubeConfigPaths = []string{
		"/etc/openshift/master/admin.kubeconfig",           // enterprise
		"/openshift.local.config/master/admin.kubeconfig",  // origin systemd
		"./openshift.local.config/master/admin.kubeconfig", // origin binary
	}
)

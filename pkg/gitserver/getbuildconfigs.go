package gitserver

import (
	"fmt"
	"io"
	"os"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

const gitRepositoryAnnotationKey = "openshift.io/git-repository"

func GetRepositoryBuildConfigs(c client.Interface, name string, out io.Writer) error {

	ns := os.Getenv("POD_NAMESPACE")
	buildConfigList, err := c.BuildConfigs(ns).List(kapi.ListOptions{})
	if err != nil {
		return err
	}

	matchingBuildConfigs := []*buildapi.BuildConfig{}

	for i := range buildConfigList.Items {
		bc := &buildConfigList.Items[i]
		repoAnnotation, hasAnnotation := bc.Annotations[gitRepositoryAnnotationKey]
		if hasAnnotation {
			if repoAnnotation == name {
				matchingBuildConfigs = append(matchingBuildConfigs, bc)
			}
			continue
		}
		if bc.Name == name {
			matchingBuildConfigs = append(matchingBuildConfigs, bc)
		}
	}

	for _, bc := range matchingBuildConfigs {
		var ref string
		if bc.Spec.Source.Git != nil {
			ref = bc.Spec.Source.Git.Ref
		}
		if ref == "" {
			ref = "master"
		}
		fmt.Fprintf(out, "%s %s\n", bc.Name, ref)
	}

	return nil
}

func GetClient() (client.Interface, error) {
	clientConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %v", err)
	}
	osClient, err := client.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error obtaining OpenShift client: %v", err)
	}
	return osClient, nil
}

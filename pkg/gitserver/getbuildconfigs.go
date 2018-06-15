package gitserver

import (
	"fmt"
	"io"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
)

const gitRepositoryAnnotationKey = "openshift.io/git-repository"

func GetRepositoryBuildConfigs(c buildclient.Interface, name string, out io.Writer) error {

	ns := os.Getenv("POD_NAMESPACE")
	buildConfigList, err := c.Build().BuildConfigs(ns).List(metav1.ListOptions{})
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

// GetClient returns a build client.
func GetClient() (buildclient.Interface, error) {
	clientConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %v", err)
	}
	buildClient, err := buildclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error obtaining OpenShift client: %v", err)
	}
	return buildClient, nil
}

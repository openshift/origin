package gitserver

import (
	"fmt"
	"io"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	buildv1 "github.com/openshift/api/build/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
)

const gitRepositoryAnnotationKey = "openshift.io/git-repository"

func GetRepositoryBuildConfigs(c buildv1client.Interface, name string, out io.Writer) error {

	ns := os.Getenv("POD_NAMESPACE")
	buildConfigList, err := c.BuildV1().BuildConfigs(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	matchingBuildConfigs := []*buildv1.BuildConfig{}

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
func GetClient() (buildv1client.Interface, error) {
	clientConfig, err := restclient.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get client config: %v", err)
	}
	buildClient, err := buildv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error obtaining OpenShift client: %v", err)
	}
	return buildClient, nil
}

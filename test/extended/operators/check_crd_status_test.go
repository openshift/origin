package operators

import (
	"fmt"
	"os"
	"testing"

	exutil "github.com/openshift/origin/test/extended/util"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

func Test_checkSubresourceStatus(t *testing.T) {

	crdClient := setupLocalAPIClientset()
	t.Run("Test_checkSubresourceStatus test", func(t *testing.T) {
		crdList, err := getCRDItemList(*crdClient)
		if err != nil {
			t.Errorf("Fail: %s", err)
		}
		failures := checkSubresourceStatus(crdList)
		if len(failures) > 0 {
			t.Error("There should be no failures")
			for _, i := range failures {
				fmt.Println(i)
			}
		}
	})
}

func setupLocalAPIClientset() *apiextensionsclientset.Clientset {
	// Get the kubeconfig by creating an Openshift cluster with cluster-bot, downloading it,
	// and using the filename for KUBECONFIG.
	home_dir := os.Getenv("HOME")
	err := os.Setenv("KUBECONFIG", fmt.Sprintf("%s/Downloads/cluster-bot-2022-05-10-100029.kubeconfig.txt", home_dir))
	kube_dir := os.Getenv("KUBECONFIG")
	fmt.Println(kube_dir)
	if err != nil {
		fmt.Printf("Error setting KUBECONFIG: %s", err)
	}
	oc := exutil.NewCLI("default")
	local_client := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
	return local_client
}

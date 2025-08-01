package compat_otp

import (
	"context"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func DeletePVCsForDeployment(client clientset.Interface, oc *exutil.CLI, deploymentPrefix string) {
	pvclist, err := client.CoreV1().PersistentVolumeClaims(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("pvc list error %#v\n", err)
	}
	for _, pvc := range pvclist.Items {
		e2e.Logf("found pvc %s\n", pvc.Name)
		if strings.HasPrefix(pvc.Name, deploymentPrefix) {
			err = client.CoreV1().PersistentVolumeClaims(oc.Namespace()).Delete(context.Background(), pvc.Name, metav1.DeleteOptions{})
			if err != nil {
				e2e.Logf("pvc del error %#v\n", err)
			} else {
				e2e.Logf("deleted pvc %s\n", pvc.Name)
			}
		}
	}
}

func DumpPersistentVolumeInfo(oc *exutil.CLI) {
	e2e.Logf("Dumping persistent volume info for cluster")
	out, err := oc.AsAdmin().Run("get").Args("pv").Output()
	if err != nil {
		e2e.Logf("Error dumping persistent volume info: %v", err)
		return
	}
	e2e.Logf("\n%s", out)
	out, err = oc.AsAdmin().Run("get").Args("pv", "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping persistent volume info: %v", err)
		return
	}
	e2e.Logf(out)
	out, err = oc.AsAdmin().Run("get").Args("pvc", "-n", oc.Namespace()).Output()
	if err != nil {
		e2e.Logf("Error dumping persistent volume claim info: %v", err)
		return
	}
	e2e.Logf("\n%s", out)
	out, err = oc.AsAdmin().Run("get").Args("pvc", "-n", oc.Namespace(), "-o", "yaml").Output()
	if err != nil {
		e2e.Logf("Error dumping persistent volume claim info: %v", err)
		return
	}
	e2e.Logf(out)

}

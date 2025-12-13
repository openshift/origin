package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	defaultScAnnotationKey = "storageclass.kubernetes.io/is-default-class"
	inTreeScPrefix         = "kubernetes.io/"
	sleepInterval          = 10
	maxRetries             = 10
)

// This is [Serial] because it modifies the default ClusterCSIDriver and StorageClass
var _ = g.Describe("[sig-storage][Feature:DisableStorageClass][Serial][apigroup:operator.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc           = exutil.NewCLIWithPodSecurityLevel("disable-sc-test", admissionapi.LevelPrivileged)
		sctest       *DisableStorageClassTest
		savedSCState operatorv1.StorageClassStateName
	)

	g.BeforeEach(func() {
		exutil.PreTestDump()

		// find default storage class, skip if not found
		sc := FindDefaultStorageClass(oc)
		if sc == nil {
			g.Skip("No default StorageClass found")
		}

		sctest = NewDisableStorageClassTest(oc, sc.Name, sc.Provisioner)
		// save the original StorageClassState value
		savedSCState = sctest.GetSCState()
		// set StorageClassState to Managed to start with the default SC
		if savedSCState != operatorv1.ManagedStorageClass {
			g.By("setting StorageClassState to Managed")
			sctest.SetSCState(operatorv1.ManagedStorageClass)
			g.By("verifying the StorageClass exists")
			sctest.VerifySCExists(true)
		}
	})

	g.AfterEach(func() {
		// restore StorageClassState to the original value
		sctest.SetSCState(savedSCState)
	})

	g.It("should reconcile the StorageClass when StorageClassState is Managed", g.Label("Size:S"), func() {
		g.By("verifying StorageClassState is set to Managed")
		scState := sctest.GetSCState()
		o.Expect(scState).To(o.Equal(operatorv1.ManagedStorageClass))

		g.By("setting AllowVolumeExpansion to false on the StorageClass")
		sctest.SetAllowExpansion(false)

		g.By("verifying the AllowVolumeExpansion reverts to true")
		sctest.VerifyAllowExpansion(true, nil)
	})

	g.It("should not reconcile the StorageClass when StorageClassState is Unmanaged", g.Label("Size:S"), func() {
		g.By("setting StorageClassState to Unmanaged")
		sctest.SetSCState(operatorv1.UnmanagedStorageClass)

		g.By("setting AllowVolumeExpansion to false on the StorageClass")
		sctest.SetAllowExpansion(false)

		g.By("verifying the AllowVolumeExpansion stays set to false")
		// wait to see if the operator tries to reconcile before checking
		time.Sleep(sleepInterval * time.Second)
		sctest.VerifyAllowExpansion(false, func() {
			// try again if verification fails
			sctest.SetAllowExpansion(false)
		})
	})

	g.It("should remove the StorageClass when StorageClassState is Removed", g.Label("Size:S"), func() {
		g.By("setting StorageClassState to Removed")
		sctest.SetSCState(operatorv1.RemovedStorageClass)

		g.By("verifying the StorageClass is removed")
		sctest.VerifySCExists(false)

		g.By("setting StorageClassState to Managed")
		sctest.SetSCState(operatorv1.ManagedStorageClass)

		g.By("verifying the StorageClass is recreated")
		sctest.VerifySCExists(true)
	})
})

func FindDefaultStorageClass(oc *exutil.CLI) *storagev1.StorageClass {
	scList, err := oc.AdminKubeClient().StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			e2e.Logf("no storage classes found")
			return nil
		}
		e2e.Failf("could not list storage classes: %v", err)
	}
	for _, sc := range scList.Items {
		if strings.HasPrefix(sc.Provisioner, inTreeScPrefix) {
			e2e.Logf("ignoring in-tree storage class %s", sc.Name)
			continue
		}
		if sc.Annotations[defaultScAnnotationKey] == "true" {
			return &sc
		}
	}
	e2e.Logf("no default storage class found")
	return nil
}

type DisableStorageClassTest struct {
	oc              *exutil.CLI
	scName          string
	provisionerName string
}

func NewDisableStorageClassTest(oc *exutil.CLI, scName string, provisionerName string) *DisableStorageClassTest {
	return &DisableStorageClassTest{
		oc:              oc,
		scName:          scName,
		provisionerName: provisionerName,
	}
}

func (d *DisableStorageClassTest) GetSCState() operatorv1.StorageClassStateName {
	ccd, err := d.oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Get(context.Background(), d.provisionerName, metav1.GetOptions{})
	if err != nil {
		e2e.Failf("failed to get ClusterCSIDriver %s: %v", d.provisionerName, err)
	}
	return ccd.Spec.StorageClassState
}

func (d *DisableStorageClassTest) SetSCState(scState operatorv1.StorageClassStateName) {
	patch := []byte(fmt.Sprintf("{\"spec\":{\"storageClassState\":\"%s\"}}", scState))
	_, err := d.oc.AdminOperatorClient().OperatorV1().ClusterCSIDrivers().Patch(context.Background(), d.provisionerName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		e2e.Failf("failed to patch ClusterCSIDriver: %v", err)
	}
}

func (d *DisableStorageClassTest) VerifySCExists(expected bool) {
	for i := 0; i < maxRetries; i++ {
		found := true
		_, err := d.oc.AdminKubeClient().StorageV1().StorageClasses().Get(context.Background(), d.scName, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				e2e.Failf("failed to get StorageClass %s: %v", d.scName, err)
			}
			found = false
		}
		if found == expected {
			e2e.Logf("StorageClass %s matched expected (exists = %t) after %d attempts", d.scName, expected, i)
			return
		}
		time.Sleep(sleepInterval * time.Second)
	}
	e2e.Failf("StorageClass %s did not match expected (exists = %t) after %d attempts", d.scName, expected, maxRetries)
}

func (d *DisableStorageClassTest) GetAllowExpansion() bool {
	sc, err := d.oc.AdminKubeClient().StorageV1().StorageClasses().Get(context.Background(), d.scName, metav1.GetOptions{})
	if err != nil {
		e2e.Failf("failed to get StorageClass %s: %v", d.scName, err)
	}
	allowExpansion := false
	if sc.AllowVolumeExpansion != nil {
		allowExpansion = *sc.AllowVolumeExpansion
	}
	return allowExpansion
}

type SCPatch struct {
	AllowVolumeExpansion *bool `json:"allowVolumeExpansion,omitempty"`
}

func (d *DisableStorageClassTest) SetAllowExpansion(allowExpansion bool) {
	scPatch := SCPatch{AllowVolumeExpansion: &allowExpansion}
	patch, err := json.Marshal(scPatch)
	if err != nil {
		e2e.Failf("failed to marshal json: %v", err)
	}
	_, err = d.oc.AdminKubeClient().StorageV1().StorageClasses().Patch(context.Background(), d.scName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		e2e.Failf("failed to patch StorageClass: %v", err)
	}
}

func (d *DisableStorageClassTest) VerifyAllowExpansion(expected bool, retry func()) {
	for i := 0; i < maxRetries; i++ {
		sc, err := d.oc.AdminKubeClient().StorageV1().StorageClasses().Get(context.Background(), d.scName, metav1.GetOptions{})
		if err != nil {
			e2e.Failf("failed to get StorageClass %s: %v", d.scName, err)
		}
		allowExpansion := false
		if sc.AllowVolumeExpansion != nil {
			allowExpansion = *sc.AllowVolumeExpansion
		}
		if allowExpansion == expected {
			e2e.Logf("AllowVolumeExpansion matched %t after %d attempts", expected, maxRetries)
			return
		}
		if retry != nil {
			e2e.Logf("VerifyAllowExpansion calling retry after %d attempts", i)
			retry()
		}
		time.Sleep(sleepInterval * time.Second)
	}
	e2e.Failf("AllowVolumeExpansion did not match %t after %d attempts", expected, maxRetries)
}

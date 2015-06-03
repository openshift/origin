package admission

import (
	kadmission "github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"testing"

	allocator "github.com/openshift/origin/pkg/security"
)

func NewTestAdmission(store cache.Store, kclient client.Interface) kadmission.Interface {
	return &constraint{
		Handler: kadmission.NewHandler(kadmission.Create, kadmission.Update),
		client:  kclient,
		store:   store,
	}
}

func TestAdmission(t *testing.T) {
	// create the annotated namespace and add it to the fake client
	namespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
			Annotations: map[string]string{
				allocator.UIDRangeAnnotation: "1/3",
				allocator.MCSAnnotation:      "s0:c1,c0",
			},
		},
	}
	serviceAccount := &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name: "default",
		},
	}

	tc := testclient.NewSimpleFake(namespace, serviceAccount)

	// create scc that requires allocation retrieval
	saSCC := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-sa",
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAsRange,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyMustRunAs,
		},
		Groups: []string{"system:serviceaccounts"},
	}
	// create scc that has specific requirements that shouldn't match but is permissioned to
	// service accounts to test exact matches
	var exactUID int64 = 999
	saExactSCC := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-sa-exact",
		},
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyMustRunAs,
			UID:  &exactUID,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyMustRunAs,
			SELinuxOptions: &kapi.SELinuxOptions{
				Level: "s9:z0,z1",
			},
		},
		Groups: []string{"system:serviceaccounts"},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	store.Add(saSCC)
	store.Add(saExactSCC)

	// create the admission plugin
	p := NewTestAdmission(store, tc)

	// setup test data
	// goodPod is empty and should not be used directly for testing since we're providing
	// two different SCCs.  Since no values are specified it would be allowed to match either
	// SCC when defaults are filled in.
	goodPod := func() *kapi.Pod {
		return &kapi.Pod{
			Spec: kapi.PodSpec{
				ServiceAccount: "default",
				Containers: []kapi.Container{
					{
						SecurityContext: &kapi.SecurityContext{},
					},
				},
			},
		}
	}

	uidNotInRange := goodPod()
	var uid int64 = 1001
	uidNotInRange.Spec.Containers[0].SecurityContext.RunAsUser = &uid

	invalidMCSLabels := goodPod()
	invalidMCSLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s1:q0,q1",
	}

	disallowedPriv := goodPod()
	var priv bool = true
	disallowedPriv.Spec.Containers[0].SecurityContext.Privileged = &priv

	// specifies a UID in the range of the preallocated UID annotation
	specifyUIDInRange := goodPod()
	var goodUID int64 = 3
	specifyUIDInRange.Spec.Containers[0].SecurityContext.RunAsUser = &goodUID

	// specifieds an mcs label that matches the preallocated mcs annotation
	specifyLabels := goodPod()
	specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions = &kapi.SELinuxOptions{
		Level: "s0:c1,c0",
	}

	testCases := map[string]struct {
		pod           *kapi.Pod
		shouldAdmit   bool
		expectedUID   int64
		expectedLevel string
		expectedPriv  bool
	}{
		"uidNotInRange": {
			pod:         uidNotInRange,
			shouldAdmit: false,
		},
		"invalidMCSLabels": {
			pod:         invalidMCSLabels,
			shouldAdmit: false,
		},
		"disallowedPriv": {
			pod:         disallowedPriv,
			shouldAdmit: false,
		},
		"specifyUIDInRange": {
			pod:           specifyUIDInRange,
			shouldAdmit:   true,
			expectedUID:   *specifyUIDInRange.Spec.Containers[0].SecurityContext.RunAsUser,
			expectedLevel: "s0:c1,c0",
		},
		"specifyLabels": {
			pod:           specifyLabels,
			shouldAdmit:   true,
			expectedUID:   1,
			expectedLevel: specifyLabels.Spec.Containers[0].SecurityContext.SELinuxOptions.Level,
		},
	}

	for k, v := range testCases {
		attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "", string(kapi.ResourcePods), kadmission.Create, &user.DefaultInfo{})
		err := p.Admit(attrs)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected no errors but received %v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected errors but received none", k)
		}

		if v.shouldAdmit {
			validatedSCC, ok := v.pod.Annotations[createPodAnnotationKey(&v.pod.Spec.Containers[0])]
			if !ok {
				t.Errorf("%s expected to find the validated annotation on the pod for the scc but found none", k)
			}
			if validatedSCC != saSCC.Name {
				t.Errorf("%s should have validated against %s but found %s", k, saSCC.Name, validatedSCC)
			}
			if *v.pod.Spec.Containers[0].SecurityContext.RunAsUser != v.expectedUID {
				t.Errorf("%s expected UID %d but found %d", k, v.expectedUID, *v.pod.Spec.Containers[0].SecurityContext.RunAsUser)
			}
			if v.pod.Spec.Containers[0].SecurityContext.SELinuxOptions.Level != v.expectedLevel {
				t.Errorf("%s expected Level %s but found %s", k, v.expectedLevel, v.pod.Spec.Containers[0].SecurityContext.SELinuxOptions.Level)
			}
		}
	}

	// now add an escalated scc to the group and re-run the cases that expected failure, they should
	// now pass by validating against the escalated scc.
	adminSCC := &kapi.SecurityContextConstraints{
		ObjectMeta: kapi.ObjectMeta{
			Name: "scc-admin",
		},
		AllowPrivilegedContainer: true,
		RunAsUser: kapi.RunAsUserStrategyOptions{
			Type: kapi.RunAsUserStrategyRunAsAny,
		},
		SELinuxContext: kapi.SELinuxContextStrategyOptions{
			Type: kapi.SELinuxStrategyRunAsAny,
		},
		Groups: []string{"system:serviceaccounts"},
	}
	store.Add(adminSCC)

	for k, v := range testCases {
		if !v.shouldAdmit {
			attrs := kadmission.NewAttributesRecord(v.pod, "Pod", "", string(kapi.ResourcePods), kadmission.Create, &user.DefaultInfo{})
			err := p.Admit(attrs)
			if err != nil {
				t.Errorf("Expected %s to pass with escalated scc but got error %v", k, err)
			}
			validatedSCC, ok := v.pod.Annotations[createPodAnnotationKey(&v.pod.Spec.Containers[0])]
			if !ok {
				t.Errorf("%s expected to find the validated annotation on the pod for the scc but found none", k)
			}
			if validatedSCC != adminSCC.Name {
				t.Errorf("%s should have validated against %s but found %s", k, adminSCC.Name, validatedSCC)
			}
		}
	}
}

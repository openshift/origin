package validation

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	securityapi "github.com/openshift/origin/pkg/security/api"
)

func validPodSpec() kapi.PodSpec {
	activeDeadlineSeconds := int64(1)
	return kapi.PodSpec{
		Volumes: []kapi.Volume{
			{Name: "vol", VolumeSource: kapi.VolumeSource{EmptyDir: &kapi.EmptyDirVolumeSource{}}},
		},
		Containers:    []kapi.Container{{Name: "ctr", Image: "image", ImagePullPolicy: "IfNotPresent"}},
		RestartPolicy: kapi.RestartPolicyAlways,
		NodeSelector: map[string]string{
			"key": "value",
		},
		NodeName:              "foobar",
		DNSPolicy:             kapi.DNSClusterFirst,
		ActiveDeadlineSeconds: &activeDeadlineSeconds,
		ServiceAccountName:    "acct",
	}
}

func invalidPodSpec() kapi.PodSpec {
	return kapi.PodSpec{
		Containers:    []kapi.Container{{}},
		RestartPolicy: kapi.RestartPolicyAlways,
		DNSPolicy:     kapi.DNSClusterFirst,
	}
}

func TestValidatePodSecurityPolicySelfSubjectReview(t *testing.T) {
	okCases := map[string]securityapi.PodSecurityPolicySelfSubjectReview{
		"good case": {
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
			},
		},
	}
	for k, v := range okCases {
		errs := ValidatePodSecurityPolicySelfSubjectReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

	koCases := map[string]securityapi.PodSecurityPolicySelfSubjectReview{
		"[spec.template.spec.containers[0].name: Required value, spec.template.spec.containers[0].image: Required value, spec.template.spec.containers[0].imagePullPolicy: Required value]": {
			Spec: securityapi.PodSecurityPolicySelfSubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: invalidPodSpec(),
				},
			},
		},
	}
	for k, v := range koCases {
		errs := ValidatePodSecurityPolicySelfSubjectReview(&v)
		if len(errs) == 0 {
			t.Errorf("%s expected error %v", k, errs)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("Expected '%s' got '%s'", k, errs.ToAggregate().Error())
		}
	}
}

func TestValidatePodSecurityPolicySubjectReview(t *testing.T) {
	okCases := map[string]securityapi.PodSecurityPolicySubjectReview{
		"good case": {
			Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
			},
		},
	}
	for k, v := range okCases {
		errs := ValidatePodSecurityPolicySubjectReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

	koCases := map[string]securityapi.PodSecurityPolicySubjectReview{
		"[spec.template.spec.containers[0].name: Required value, spec.template.spec.containers[0].image: Required value, spec.template.spec.containers[0].imagePullPolicy: Required value]": {
			Spec: securityapi.PodSecurityPolicySubjectReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: invalidPodSpec(),
				},
			},
		},
	}
	for k, v := range koCases {
		errs := ValidatePodSecurityPolicySubjectReview(&v)
		if len(errs) == 0 {
			t.Errorf("%s expected error %v", k, errs)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("Expected '%s' got '%s'", k, errs.ToAggregate().Error())
		}
	}
}

func TestValidatePodSecurityPolicyReview(t *testing.T) {
	okCases := map[string]securityapi.PodSecurityPolicyReview{
		"good case 1": {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
			},
		},
		"good case 2": {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"good-service.account"},
			},
		},
	}
	for k, v := range okCases {
		errs := ValidatePodSecurityPolicyReview(&v)
		if len(errs) > 0 {
			t.Errorf("%s unexpected error %v", k, errs)
		}
	}

	koCases := map[string]securityapi.PodSecurityPolicyReview{
		"[spec.template.spec.containers[0].name: Required value, spec.template.spec.containers[0].image: Required value, spec.template.spec.containers[0].imagePullPolicy: Required value]": {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: invalidPodSpec(),
				},
			},
		},
		`spec.serviceAccountNames[0]: Invalid value: "my bad sa": must match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)* (e.g. 'example.com')`: {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"my bad sa"},
			},
		},
		`spec.serviceAccountNames[1]: Invalid value: "my bad sa": must match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)* (e.g. 'example.com')`: {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"good-service.account", "my bad sa"},
			},
		},
		`spec.serviceAccountNames[2]: Invalid value: ""`: {
			Spec: securityapi.PodSecurityPolicyReviewSpec{
				Template: kapi.PodTemplateSpec{
					Spec: validPodSpec(),
				},
				ServiceAccountNames: []string{"good-service.account1", "good-service.account2", ""},
			},
		},
	}
	for k, v := range koCases {
		errs := ValidatePodSecurityPolicyReview(&v)
		if len(errs) == 0 {
			t.Errorf("%s expected error %v", k, errs)
			continue
		}
		if errs.ToAggregate().Error() != k {
			t.Errorf("Expected '%s' got '%s'", k, errs.ToAggregate().Error())
		}
	}

}

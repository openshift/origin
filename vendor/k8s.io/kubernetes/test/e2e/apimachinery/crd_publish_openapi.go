/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apimachinery

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-openapi/spec"
	"github.com/onsi/ginkgo"
	"k8s.io/utils/pointer"

	"k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	k8sclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	openapiutil "k8s.io/kube-openapi/pkg/util"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/utils/crd"
	"sigs.k8s.io/yaml"
)

var (
	metaPattern = `"kind":"%s","apiVersion":"%s/%s","metadata":{"name":"%s"}`
)

var _ = SIGDescribe("CustomResourcePublishOpenAPI [Privileged:ClusterAdmin]", func() {
	f := framework.NewDefaultFramework("crd-publish-openapi")

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, with validation schema
		Description: Register a custom resource definition with a validating schema consisting of objects, arrays and
		primitives. Attempt to create and apply a change a custom resource using valid properties, via kubectl;
		client-side validation MUST pass. Attempt both operations with unknown properties and without required
		properties; client-side validation MUST reject the operations. Attempt kubectl explain; the output MUST
		explain the custom resource properties. Attempt kubectl explain on custom resource properties; the output MUST
		explain the nested custom resource properties.
	*/
	framework.ConformanceIt("works for CRD with validation schema", func() {
		crd, err := setupCRDAndVerifySchema(f, schemaFoo, "foo", "v1")
		if err != nil {
			framework.Failf("%v", err)
		}

		meta := fmt.Sprintf(metaPattern, crd.Crd.Spec.Names.Kind, crd.Crd.Spec.Group, crd.Crd.Spec.Versions[0].Name, "test-foo")
		ns := fmt.Sprintf("--namespace=%v", f.Namespace.Name)

		ginkgo.By("client-side validation (kubectl create and apply) allows request with known and required properties")
		validCR := fmt.Sprintf(`{%s,"spec":{"bars":[{"name":"test-bar"}]}}`, meta)
		if _, err := framework.RunKubectlInput(f.Namespace.Name, validCR, ns, "create", "-f", "-"); err != nil {
			framework.Failf("failed to create valid CR %s: %v", validCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-foo"); err != nil {
			framework.Failf("failed to delete valid CR: %v", err)
		}
		if _, err := framework.RunKubectlInput(f.Namespace.Name, validCR, ns, "apply", "-f", "-"); err != nil {
			framework.Failf("failed to apply valid CR %s: %v", validCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-foo"); err != nil {
			framework.Failf("failed to delete valid CR: %v", err)
		}

		// TODO(workload): re-enable client-side validation tests
		/*
			ginkgo.By("client-side validation (kubectl create and apply) rejects request with unknown properties when disallowed by the schema")
			unknownCR := fmt.Sprintf(`{%s,"spec":{"foo":true}}`, meta)
			if _, err := framework.RunKubectlInput(f.Namespace.Name, unknownCR, ns, "create", "-f", "-"); err == nil || !strings.Contains(err.Error(), `unknown field "foo"`) {
				framework.Failf("unexpected no error when creating CR with unknown field: %v", err)
			}
			if _, err := framework.RunKubectlInput(f.Namespace.Name, unknownCR, ns, "apply", "-f", "-"); err == nil || !strings.Contains(err.Error(), `unknown field "foo"`) {
				framework.Failf("unexpected no error when applying CR with unknown field: %v", err)
			}

			ginkgo.By("client-side validation (kubectl create and apply) rejects request without required properties")
			noRequireCR := fmt.Sprintf(`{%s,"spec":{"bars":[{"age":"10"}]}}`, meta)
			if _, err := framework.RunKubectlInput(f.Namespace.Name, noRequireCR, ns, "create", "-f", "-"); err == nil || !strings.Contains(err.Error(), `missing required field "name"`) {
				framework.Failf("unexpected no error when creating CR without required field: %v", err)
			}
			if _, err := framework.RunKubectlInput(f.Namespace.Name, noRequireCR, ns, "apply", "-f", "-"); err == nil || !strings.Contains(err.Error(), `missing required field "name"`) {
				framework.Failf("unexpected no error when applying CR without required field: %v", err)
			}
		*/

		ginkgo.By("kubectl explain works to explain CR properties")
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural, `(?s)DESCRIPTION:.*Foo CRD for Testing.*FIELDS:.*apiVersion.*<string>.*APIVersion defines.*spec.*<Object>.*Specification of Foo`); err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("kubectl explain works to explain CR properties recursively")
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural+".metadata", `(?s)DESCRIPTION:.*Standard object's metadata.*FIELDS:.*creationTimestamp.*<string>.*CreationTimestamp is a timestamp`); err != nil {
			framework.Failf("%v", err)
		}
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural+".spec", `(?s)DESCRIPTION:.*Specification of Foo.*FIELDS:.*bars.*<\[\]Object>.*List of Bars and their specs`); err != nil {
			framework.Failf("%v", err)
		}
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural+".spec.bars", `(?s)RESOURCE:.*bars.*<\[\]Object>.*DESCRIPTION:.*List of Bars and their specs.*FIELDS:.*bazs.*<\[\]string>.*List of Bazs.*name.*<string>.*Name of Bar`); err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("kubectl explain works to return error when explain is called on property that doesn't exist")
		if _, err := framework.RunKubectl(f.Namespace.Name, "explain", crd.Crd.Spec.Names.Plural+".spec.bars2"); err == nil || !strings.Contains(err.Error(), `field "bars2" does not exist`) {
			framework.Failf("unexpected no error when explaining property that doesn't exist: %v", err)
		}

		if err := cleanupCRD(f, crd); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, with x-preserve-unknown-fields in object
		Description: Register a custom resource definition with x-preserve-unknown-fields in the top level object.
		Attempt to create and apply a change a custom resource, via kubectl; client-side validation MUST accept unknown
		properties. Attempt kubectl explain; the output MUST contain a valid DESCRIPTION stanza.
	*/
	framework.ConformanceIt("works for CRD without validation schema", func() {
		crd, err := setupCRDAndVerifySchema(f, nil, "empty", "v1")
		if err != nil {
			framework.Failf("%v", err)
		}

		meta := fmt.Sprintf(metaPattern, crd.Crd.Spec.Names.Kind, crd.Crd.Spec.Group, crd.Crd.Spec.Versions[0].Name, "test-cr")
		ns := fmt.Sprintf("--namespace=%v", f.Namespace.Name)

		ginkgo.By("client-side validation (kubectl create and apply) allows request with any unknown properties")
		randomCR := fmt.Sprintf(`{%s,"a":{"b":[{"c":"d"}]}}`, meta)
		if _, err := framework.RunKubectlInput(f.Namespace.Name, randomCR, ns, "create", "-f", "-"); err != nil {
			framework.Failf("failed to create random CR %s for CRD without schema: %v", randomCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-cr"); err != nil {
			framework.Failf("failed to delete random CR: %v", err)
		}
		if _, err := framework.RunKubectlInput(f.Namespace.Name, randomCR, ns, "apply", "-f", "-"); err != nil {
			framework.Failf("failed to apply random CR %s for CRD without schema: %v", randomCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-cr"); err != nil {
			framework.Failf("failed to delete random CR: %v", err)
		}

		ginkgo.By("kubectl explain works to explain CR without validation schema")
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural, `(?s)DESCRIPTION:.*<empty>`); err != nil {
			framework.Failf("%v", err)
		}

		if err := cleanupCRD(f, crd); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, with x-preserve-unknown-fields at root
		Description: Register a custom resource definition with x-preserve-unknown-fields in the schema root.
		Attempt to create and apply a change a custom resource, via kubectl; client-side validation MUST accept unknown
		properties. Attempt kubectl explain; the output MUST show the custom resource KIND.
	*/
	framework.ConformanceIt("works for CRD preserving unknown fields at the schema root", func() {
		crd, err := setupCRD(f, schemaPreserveRoot, "unknown-at-root", "v1")
		if err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crd, "v1"), nil); err != nil {
			framework.Failf("%v", err)
		}

		meta := fmt.Sprintf(metaPattern, crd.Crd.Spec.Names.Kind, crd.Crd.Spec.Group, crd.Crd.Spec.Versions[0].Name, "test-cr")
		ns := fmt.Sprintf("--namespace=%v", f.Namespace.Name)

		ginkgo.By("client-side validation (kubectl create and apply) allows request with any unknown properties")
		randomCR := fmt.Sprintf(`{%s,"a":{"b":[{"c":"d"}]}}`, meta)
		if _, err := framework.RunKubectlInput(f.Namespace.Name, randomCR, ns, "create", "-f", "-"); err != nil {
			framework.Failf("failed to create random CR %s for CRD that allows unknown properties at the root: %v", randomCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-cr"); err != nil {
			framework.Failf("failed to delete random CR: %v", err)
		}
		if _, err := framework.RunKubectlInput(f.Namespace.Name, randomCR, ns, "apply", "-f", "-"); err != nil {
			framework.Failf("failed to apply random CR %s for CRD without schema: %v", randomCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-cr"); err != nil {
			framework.Failf("failed to delete random CR: %v", err)
		}

		ginkgo.By("kubectl explain works to explain CR")
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural, fmt.Sprintf(`(?s)KIND:.*%s`, crd.Crd.Spec.Names.Kind)); err != nil {
			framework.Failf("%v", err)
		}

		if err := cleanupCRD(f, crd); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, with x-preserve-unknown-fields in embedded object
		Description: Register a custom resource definition with x-preserve-unknown-fields in an embedded object.
		Attempt to create and apply a change a custom resource, via kubectl; client-side validation MUST accept unknown
		properties. Attempt kubectl explain; the output MUST show that x-preserve-unknown-properties is used on the
		nested field.
	*/
	framework.ConformanceIt("works for CRD preserving unknown fields in an embedded object", func() {
		crd, err := setupCRD(f, schemaPreserveNested, "unknown-in-nested", "v1")
		if err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crd, "v1"), nil); err != nil {
			framework.Failf("%v", err)
		}

		meta := fmt.Sprintf(metaPattern, crd.Crd.Spec.Names.Kind, crd.Crd.Spec.Group, crd.Crd.Spec.Versions[0].Name, "test-cr")
		ns := fmt.Sprintf("--namespace=%v", f.Namespace.Name)

		ginkgo.By("client-side validation (kubectl create and apply) allows request with any unknown properties")
		randomCR := fmt.Sprintf(`{%s,"spec":{"b":[{"c":"d"}]}}`, meta)
		if _, err := framework.RunKubectlInput(f.Namespace.Name, randomCR, ns, "create", "-f", "-"); err != nil {
			framework.Failf("failed to create random CR %s for CRD that allows unknown properties in a nested object: %v", randomCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-cr"); err != nil {
			framework.Failf("failed to delete random CR: %v", err)
		}
		if _, err := framework.RunKubectlInput(f.Namespace.Name, randomCR, ns, "apply", "-f", "-"); err != nil {
			framework.Failf("failed to apply random CR %s for CRD without schema: %v", randomCR, err)
		}
		if _, err := framework.RunKubectl(f.Namespace.Name, ns, "delete", crd.Crd.Spec.Names.Plural, "test-cr"); err != nil {
			framework.Failf("failed to delete random CR: %v", err)
		}

		ginkgo.By("kubectl explain works to explain CR")
		if err := verifyKubectlExplain(f.Namespace.Name, crd.Crd.Spec.Names.Plural, `(?s)DESCRIPTION:.*preserve-unknown-properties in nested field for Testing`); err != nil {
			framework.Failf("%v", err)
		}

		if err := cleanupCRD(f, crd); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, varying groups
		Description: Register multiple custom resource definitions spanning different groups and versions;
		OpenAPI definitions MUST be published for custom resource definitions.
	*/
	framework.ConformanceIt("works for multiple CRDs of different groups", func() {
		ginkgo.By("CRs in different groups (two CRDs) show up in OpenAPI documentation")
		crdFoo, err := setupCRD(f, schemaFoo, "foo", "v1")
		if err != nil {
			framework.Failf("%v", err)
		}
		crdWaldo, err := setupCRD(f, schemaWaldo, "waldo", "v1beta1")
		if err != nil {
			framework.Failf("%v", err)
		}
		if crdFoo.Crd.Spec.Group == crdWaldo.Crd.Spec.Group {
			framework.Failf("unexpected: CRDs should be of different group %v, %v", crdFoo.Crd.Spec.Group, crdWaldo.Crd.Spec.Group)
		}
		if err := waitForDefinition(f, definitionName(crdWaldo, "v1beta1"), schemaWaldo); err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdFoo, "v1"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdWaldo); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, varying versions
		Description: Register a custom resource definition with multiple versions; OpenAPI definitions MUST be published
		for custom resource definitions.
	*/
	framework.ConformanceIt("works for multiple CRDs of same group but different versions", func() {
		ginkgo.By("CRs in the same group but different versions (one multiversion CRD) show up in OpenAPI documentation")
		crdMultiVer, err := setupCRD(f, schemaFoo, "multi-ver", "v2", "v3")
		if err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdMultiVer, "v3"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdMultiVer, "v2"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdMultiVer); err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("CRs in the same group but different versions (two CRDs) show up in OpenAPI documentation")
		crdFoo, err := setupCRD(f, schemaFoo, "common-group", "v4")
		if err != nil {
			framework.Failf("%v", err)
		}
		crdWaldo, err := setupCRD(f, schemaWaldo, "common-group", "v5")
		if err != nil {
			framework.Failf("%v", err)
		}
		if crdFoo.Crd.Spec.Group != crdWaldo.Crd.Spec.Group {
			framework.Failf("unexpected: CRDs should be of the same group %v, %v", crdFoo.Crd.Spec.Group, crdWaldo.Crd.Spec.Group)
		}
		if err := waitForDefinition(f, definitionName(crdWaldo, "v5"), schemaWaldo); err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdFoo, "v4"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdWaldo); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, varying kinds
		Description: Register multiple custom resource definitions in the same group and version but spanning different kinds;
		OpenAPI definitions MUST be published for custom resource definitions.
	*/
	framework.ConformanceIt("works for multiple CRDs of same group and version but different kinds", func() {
		ginkgo.By("CRs in the same group and version but different kinds (two CRDs) show up in OpenAPI documentation")
		crdFoo, err := setupCRD(f, schemaFoo, "common-group", "v6")
		if err != nil {
			framework.Failf("%v", err)
		}
		crdWaldo, err := setupCRD(f, schemaWaldo, "common-group", "v6")
		if err != nil {
			framework.Failf("%v", err)
		}
		if crdFoo.Crd.Spec.Group != crdWaldo.Crd.Spec.Group {
			framework.Failf("unexpected: CRDs should be of the same group %v, %v", crdFoo.Crd.Spec.Group, crdWaldo.Crd.Spec.Group)
		}
		if err := waitForDefinition(f, definitionName(crdWaldo, "v6"), schemaWaldo); err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdFoo, "v6"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := cleanupCRD(f, crdWaldo); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, version rename
		Description: Register a custom resource definition with multiple versions; OpenAPI definitions MUST be published
		for custom resource definitions. Rename one of the versions of the custom resource definition via a patch;
		OpenAPI definitions MUST update to reflect the rename.
	*/
	framework.ConformanceIt("updates the published spec when one version gets renamed", func() {
		ginkgo.By("set up a multi version CRD")
		crdMultiVer, err := setupCRD(f, schemaFoo, "multi-ver", "v2", "v3")
		if err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdMultiVer, "v3"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crdMultiVer, "v2"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("rename a version")
		patch := []byte(`[
			{"op":"test","path":"/spec/versions/1/name","value":"v3"},
			{"op": "replace", "path": "/spec/versions/1/name", "value": "v4"}
		]`)
		crdMultiVer.Crd, err = crdMultiVer.APIExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Patch(context.TODO(), crdMultiVer.Crd.Name, types.JSONPatchType, patch, metav1.PatchOptions{})
		if err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("check the new version name is served")
		if err := waitForDefinition(f, definitionName(crdMultiVer, "v4"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		ginkgo.By("check the old version name is removed")
		if err := waitForDefinitionCleanup(f, definitionName(crdMultiVer, "v3")); err != nil {
			framework.Failf("%v", err)
		}
		ginkgo.By("check the other version is not changed")
		if err := waitForDefinition(f, definitionName(crdMultiVer, "v2"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}

		// TestCrd.Versions is different from TestCrd.Crd.Versions, we have to manually
		// update the name there. Used by cleanupCRD
		crdMultiVer.Crd.Spec.Versions[1].Name = "v4"
		if err := cleanupCRD(f, crdMultiVer); err != nil {
			framework.Failf("%v", err)
		}
	})

	/*
		Release: v1.16
		Testname: Custom Resource OpenAPI Publish, stop serving version
		Description: Register a custom resource definition with multiple versions. OpenAPI definitions MUST be published
		for custom resource definitions. Update the custom resource definition to not serve one of the versions. OpenAPI
		definitions MUST be updated to not contain the version that is no longer served.
	*/
	framework.ConformanceIt("removes definition from spec when one version gets changed to not be served", func() {
		ginkgo.By("set up a multi version CRD")
		crd, err := setupCRD(f, schemaFoo, "multi-to-single-ver", "v5", "v6alpha1")
		if err != nil {
			framework.Failf("%v", err)
		}
		// just double check. setupCRD() checked this for us already
		if err := waitForDefinition(f, definitionName(crd, "v6alpha1"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}
		if err := waitForDefinition(f, definitionName(crd, "v5"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("mark a version not serverd")
		crd.Crd, err = crd.APIExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), crd.Crd.Name, metav1.GetOptions{})
		if err != nil {
			framework.Failf("%v", err)
		}
		crd.Crd.Spec.Versions[1].Served = false
		crd.Crd, err = crd.APIExtensionClient.ApiextensionsV1().CustomResourceDefinitions().Update(context.TODO(), crd.Crd, metav1.UpdateOptions{})
		if err != nil {
			framework.Failf("%v", err)
		}

		ginkgo.By("check the unserved version gets removed")
		if err := waitForDefinitionCleanup(f, definitionName(crd, "v6alpha1")); err != nil {
			framework.Failf("%v", err)
		}
		ginkgo.By("check the other version is not changed")
		if err := waitForDefinition(f, definitionName(crd, "v5"), schemaFoo); err != nil {
			framework.Failf("%v", err)
		}

		if err := cleanupCRD(f, crd); err != nil {
			framework.Failf("%v", err)
		}
	})
})

func setupCRD(f *framework.Framework, schema []byte, groupSuffix string, versions ...string) (*crd.TestCrd, error) {
	group := fmt.Sprintf("%s-test-%s.example.com", f.BaseName, groupSuffix)
	if len(versions) == 0 {
		return nil, fmt.Errorf("require at least one version for CRD")
	}

	props := &apiextensionsv1.JSONSchemaProps{}
	if schema != nil {
		if err := yaml.Unmarshal(schema, props); err != nil {
			return nil, err
		}
	}

	crd, err := crd.CreateMultiVersionTestCRD(f, group, func(crd *apiextensionsv1.CustomResourceDefinition) {
		var apiVersions []apiextensionsv1.CustomResourceDefinitionVersion
		for i, version := range versions {
			version := apiextensionsv1.CustomResourceDefinitionVersion{
				Name:    version,
				Served:  true,
				Storage: i == 0,
			}
			// set up validation when input schema isn't nil
			if schema != nil {
				version.Schema = &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: props,
				}
			} else {
				version.Schema = &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						XPreserveUnknownFields: pointer.BoolPtr(true),
						Type:                   "object",
					},
				}
			}
			apiVersions = append(apiVersions, version)
		}
		crd.Spec.Versions = apiVersions
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create CRD: %v", err)
	}
	return crd, nil
}

func setupCRDAndVerifySchema(f *framework.Framework, schema []byte, groupSuffix string, versions ...string) (*crd.TestCrd, error) {
	expect := schema
	if schema == nil {
		// to be backwards compatible, we expect CRD controller to treat
		// CRD with nil schema specially and publish an empty schema
		expect = []byte(`type: object`)
	}
	crd, err := setupCRD(f, schema, groupSuffix, versions...)
	if err != nil {
		return nil, err
	}

	for _, v := range crd.Crd.Spec.Versions {
		if err := waitForDefinition(f, definitionName(crd, v.Name), expect); err != nil {
			return nil, fmt.Errorf("%v", err)
		}
	}
	return crd, nil
}

func cleanupCRD(f *framework.Framework, crd *crd.TestCrd) error {
	crd.CleanUp()
	for _, v := range crd.Crd.Spec.Versions {
		name := definitionName(crd, v.Name)
		if err := waitForDefinitionCleanup(f, name); err != nil {
			return fmt.Errorf("%v", err)
		}
	}
	return nil
}

// waitForDefinition waits for given definition showing up in swagger with given schema for all API Servers.
// If schema is nil, only the existence of the given name is checked.
func waitForDefinition(f *framework.Framework, name string, schema []byte) error {
	apiServers, err := getAllAPIServersEndpoint(f)
	if err != nil {
		return err
	}
	framework.Logf("waiting for all %d servers to observe the same OpenAPI spec", len(apiServers))
	err = createAndWaitForPodCompletion(f, waitForCRDDefinitionInAllAPIServersPod(name, apiServers))
	if err != nil {
		return err
	}

	if schema == nil {
		return nil
	}

	expect := spec.Schema{}
	if err := convertJSONSchemaProps(schema, &expect); err != nil {
		return err
	}

	err = verifyOpenAPISchema(f.ClientSet, func(spec *spec.Swagger) (bool, string) {
		d, ok := spec.SwaggerProps.Definitions[name]
		if !ok {
			return false, fmt.Sprintf("spec.SwaggerProps.Definitions[\"%s\"] not found", name)
		}
		if schema != nil {
			// drop properties and extension that we added
			dropDefaults(&d)
			if !apiequality.Semantic.DeepEqual(expect, d) {
				return false, fmt.Sprintf("spec.SwaggerProps.Definitions[\"%s\"] not match; expect: %v, actual: %v", name, expect, d)
			}
		}
		return true, ""
	})
	if err != nil {
		return fmt.Errorf("failed to wait for definition %q to be served with the right OpenAPI schema: %v", name, err)
	}
	return nil
}

// waitForDefinitionCleanup waits for given definition to be removed from swagger
func waitForDefinitionCleanup(f *framework.Framework, name string) error {
	apiServers, err := getAllAPIServersEndpoint(f)
	if err != nil {
		return err
	}
	framework.Logf("waiting for all %d servers to observe the same OpenAPI spec (removal of the key)", len(apiServers))
	return createAndWaitForPodCompletion(f, waitForCRDDefinitionRemovalInAllAPIServersPod(name, apiServers))
}

func verifyOpenAPISchema(c k8sclientset.Interface, pred func(*spec.Swagger) (bool, string)) error {
	client := c.Discovery().RESTClient().(*rest.RESTClient).Client
	url := c.Discovery().RESTClient().Get().AbsPath("openapi", "v2").URL()
	lastMsg := ""
	if err := wait.Poll(500*time.Millisecond, 1*time.Second, func() (bool, error) {
		spec := &spec.Swagger{}
		req, err := http.NewRequest("GET", url.String(), nil)
		if err != nil {
			return false, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false, fmt.Errorf("unexpected response: %d", resp.StatusCode)
		} else if bs, err := ioutil.ReadAll(resp.Body); err != nil {
			return false, err
		} else if err := json.Unmarshal(bs, spec); err != nil {
			return false, err
		}

		var ok bool
		ok, lastMsg = pred(spec)
		return ok, nil
	}); err != nil {
		return fmt.Errorf("failed to wait for OpenAPI spec validating condition: %v; lastMsg: %s", err, lastMsg)
	}
	return nil
}

// convertJSONSchemaProps converts JSONSchemaProps in YAML to spec.Schema
func convertJSONSchemaProps(in []byte, out *spec.Schema) error {
	external := apiextensionsv1.JSONSchemaProps{}
	if err := yaml.UnmarshalStrict(in, &external); err != nil {
		return err
	}
	internal := apiextensions.JSONSchemaProps{}
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(&external, &internal, nil); err != nil {
		return err
	}
	if err := validation.ConvertJSONSchemaPropsWithPostProcess(&internal, out, validation.StripUnsupportedFormatsPostProcess); err != nil {
		return err
	}
	return nil
}

// dropDefaults drops properties and extension that we added to a schema
func dropDefaults(s *spec.Schema) {
	delete(s.Properties, "metadata")
	delete(s.Properties, "apiVersion")
	delete(s.Properties, "kind")
	delete(s.Extensions, "x-kubernetes-group-version-kind")
}

func verifyKubectlExplain(ns, name, pattern string) error {
	result, err := framework.RunKubectl(ns, "explain", name)
	if err != nil {
		return fmt.Errorf("failed to explain %s: %v", name, err)
	}
	r := regexp.MustCompile(pattern)
	if !r.Match([]byte(result)) {
		return fmt.Errorf("kubectl explain %s result {%s} doesn't match pattern {%s}", name, result, pattern)
	}
	return nil
}

// definitionName returns the openapi definition name for given CRD in given version
func definitionName(crd *crd.TestCrd, version string) string {
	return openapiutil.ToRESTFriendlyName(fmt.Sprintf("%s/%s/%s", crd.Crd.Spec.Group, version, crd.Crd.Spec.Names.Kind))
}

var schemaFoo = []byte(`description: Foo CRD for Testing
type: object
properties:
  spec:
    type: object
    description: Specification of Foo
    properties:
      bars:
        description: List of Bars and their specs.
        type: array
        items:
          type: object
          required:
          - name
          properties:
            name:
              description: Name of Bar.
              type: string
            age:
              description: Age of Bar.
              type: string
            bazs:
              description: List of Bazs.
              items:
                type: string
              type: array
  status:
    description: Status of Foo
    type: object
    properties:
      bars:
        description: List of Bars and their statuses.
        type: array
        items:
          type: object
          properties:
            name:
              description: Name of Bar.
              type: string
            available:
              description: Whether the Bar is installed.
              type: boolean
            quxType:
              description: Indicates to external qux type.
              pattern: in-tree|out-of-tree
              type: string`)

var schemaWaldo = []byte(`description: Waldo CRD for Testing
type: object
properties:
  spec:
    description: Specification of Waldo
    type: object
    properties:
      dummy:
        description: Dummy property.
        type: object
  status:
    description: Status of Waldo
    type: object
    properties:
      bars:
        description: List of Bars and their statuses.
        type: array
        items:
          type: object`)

var schemaPreserveRoot = []byte(`description: preserve-unknown-properties at root for Testing
x-kubernetes-preserve-unknown-fields: true
type: object
properties:
  spec:
    description: Specification of Waldo
    type: object
    properties:
      dummy:
        description: Dummy property.
        type: object
  status:
    description: Status of Waldo
    type: object
    properties:
      bars:
        description: List of Bars and their statuses.
        type: array
        items:
          type: object`)

var schemaPreserveNested = []byte(`description: preserve-unknown-properties in nested field for Testing
type: object
properties:
  spec:
    description: Specification of Waldo
    type: object
    x-kubernetes-preserve-unknown-fields: true
    properties:
      dummy:
        description: Dummy property.
        type: object
  status:
    description: Status of Waldo
    type: object
    properties:
      bars:
        description: List of Bars and their statuses.
        type: array
        items:
          type: object`)

func waitForCRDDefinitionInAllAPIServersPod(definitionName string, apiServers []string) *v1.Pod {
	script := `
       echo "$(date):installing dependencies"
       apk add curl jq
       TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
       KEY="${DEFINITION_KEY}"
       SEEN=0
       PREV_HTTP_BODY=""
       PREV_ETAG=""

       while [ ${SEEN} -lt ${SERVERS_LEN} ]
       do
         for server in ${SERVERS}
         do
           URL="https://$server/openapi/v2"
           echo "$(date):downloading the OpenAPI spec from ${URL}, prev_etag=${PREV_ETAG}, seen=${SEEN}"
           HTTP_RESPONSE=$(curl -k -s -w "HTTPSTATUS:%{http_code}" -H "If-None-Match:\"${PREV_ETAG}\"" -H "Authorization: Bearer $TOKEN" --dump-header rsp-header.json ${URL})
           HTTP_STATUS=$(echo $HTTP_RESPONSE | tr -d '\n' | sed -e 's/.*HTTPSTATUS://')
           HTTP_BODY=$(echo $HTTP_RESPONSE | sed -e 's/HTTPSTATUS\:.*//g')
           ETAG=$(cat rsp-header.json | grep etag: | sed -e 's/etag://' | sed -e 's/^ //' | tr -d '\r\n' | tr -d '"')
           echo "$(date):parsing the response"
           if [ $HTTP_STATUS -eq 304 ]; then
             echo "$(date):StatusNotModified returned, rewriting the previous body and etag"
             HTTP_BODY=${PREV_HTTP_BODY}
             ETAG=${PREV_ETAG}
           fi
           echo ${HTTP_BODY} | jq -e ${KEY} > /dev/null
           if [ $? -eq 0 ]; then
             echo "$(date)found the key in the response"
             PREV_HTTP_BODY=${HTTP_BODY}
             PREV_ETAG=${ETAG}
             SEEN=$(( SEEN + 1 ))
           else
            echo "$(date):haven't found the key=${KEY}, resetting the counter and trying one more time"
            PREV_HTTP_BODY=""
            PREV_ETAG=""
            SEEN=0
           fi
         done
       done
`

	r := strings.NewReplacer(
		"${SERVERS_LEN}", fmt.Sprintf("%d", len(apiServers)),
		"${SERVERS}", apiServersToString(apiServers),
		"${DEFINITION_KEY}", fmt.Sprintf(".definitions.\\\"%s\\\"", definitionName),
	)
	script = r.Replace(script)

	name := "wait-for-crd-definition"
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-" + string(uuid.NewUUID()),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   "alpine",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{script},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

func waitForCRDDefinitionRemovalInAllAPIServersPod(definitionName string, apiServers []string) *v1.Pod {
	script := `
       apk add curl jq
       TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
       KEY="${DEFINITION_KEY}"
       NOT_SEEN=0

       while [ ${NOT_SEEN} -lt ${SERVERS_LEN} ]
       do
         for server in ${SERVERS}
         do
           URL="https://$server/openapi/v2"
           echo "$(date):downloading the OpenAPI spec from ${URL}, not-seen=${NOT_SEEN}"
           curl -k -s -H "Authorization: Bearer $TOKEN" --dump-header rsp-header.json ${URL} | jq -e ${KEY} > /dev/null
           if [ $? -eq 1 ]; then
             echo "$(date):haven't found the key in the response"
             NOT_SEEN=$(( NOT_SEEN + 1 ))
           else
            echo "$(date):seen the key=${KEY}, resetting the counter and trying one more time"
            SEEN=0
           fi
         done
       done
`

	r := strings.NewReplacer(
		"${SERVERS_LEN}", fmt.Sprintf("%d", len(apiServers)),
		"${SERVERS}", apiServersToString(apiServers),
		"${DEFINITION_KEY}", fmt.Sprintf(".definitions.\\\"%s\\\"", definitionName),
	)
	script = r.Replace(script)

	name := "wait-for-crd-definition-removal"
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-" + string(uuid.NewUUID()),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   "alpine",
					Command: []string{"/bin/sh", "-c"},
					Args:    []string{script},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
}

func createAndWaitForPodCompletion(f *framework.Framework, pod *v1.Pod) error {
	_, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.Background(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	err = e2epod.WaitForPodCondition(f.ClientSet, f.Namespace.Name, pod.Name, "terminated", 2*time.Minute, func(pod *v1.Pod) (bool, error) {
		statuses := pod.Status.ContainerStatuses
		if len(statuses) == 0 || statuses[0].State.Terminated == nil {
			return false, nil
		}
		if statuses[0].State.Terminated.ExitCode == 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		framework.Logf("failed waiting for the pod's condition (last err = %v), polling logs (%s/%s in %s namespace)", err, pod.Name, pod.Spec.Containers[0].Name, f.Namespace.Name)
		logs, err := e2epod.GetPodLogs(f.ClientSet, f.Namespace.Name, pod.Name, pod.Spec.Containers[0].Name)
		if err != nil {
			framework.Logf("error pulling logs: %v", err)
			return err
		}
		framework.Logf("pooled logs:\n%v", logs)
	}
	return err
}

func getAllAPIServersEndpoint(f *framework.Framework) ([]string, error) {
	eps, err := f.ClientSet.CoreV1().Endpoints(metav1.NamespaceDefault).Get(context.Background(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	apiServers := []string{}
	for _, s := range eps.Subsets {
		var port int32
		for _, p := range s.Ports {
			if p.Name == "https" {
				port = p.Port
				break
			}
		}
		if port == 0 {
			continue
		}
		for _, ep := range s.Addresses {
			apiServers = append(apiServers, fmt.Sprintf("%s:%d", ep.IP, port))
		}
		break
	}
	if len(apiServers) == 0 {
		return nil, fmt.Errorf("didn't create api servers list from the default (\"kubernetes\") endpoint")
	}
	return apiServers, nil
}

func apiServersToString(apiServers []string) string {
	ret := ""
	for _, server := range apiServers {
		ret = fmt.Sprintf("%s %s", ret, server)
	}
	return ret
}

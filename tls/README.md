# TLS registry

This registry stores expected metadata for TLS artifacts in `openshift-*` namespaces. This metadata 
is used as the expected result in the e2e test which validates cluster secrets and configmaps.

## Registry purpose

The registry is used to collect all TLS artifacts used in OpenShift, certificate key pairs, and CA bundles alike. 
For simplicity, this document will use "TLS artifact" for both certificate, key, or CA bundle.

To ensure TLS artifacts are following a set of defined standards, we need a way to collect 
all TLS artifacts used in OpenShift. This is done via the "[sig-arch][Late] collect certificate data" test.
The test produces a JSON in `openshift-e2e-test/artifacts/rawTLSInfo/raw-tls-artifacts-<topology>-<arch>-<platform>-<network>.json`:
```yaml
{
  "LogicalName": "",
  "Description": "",
  "InClusterResourceData": {
    "certificateAuthorityBundles": [
      {
        "configMapLocation": {
          "Namespace": "openshift-apiserver-operator",
          "Name": "trusted-ca-bundle"
        },
        "certificateAuthorityBundleInfo": {
          "owningJiraComponent": "Networking / cluster-network-operator",
          "description": ""
        }
      },
...
  "certKeyPairs": [
      {
        "secretLocation": {
          "Namespace": "openshift-apiserver-operator",
          "Name": "openshift-apiserver-operator-serving-cert"
        },
        "certKeyInfo": {
          "owningJiraComponent": "service-ca",
          "description": ""
        }
      },
...
  "CertificateAuthorityBundles": {
    "Items": [
      {
        "LogicalName": "",
        "Description": "",
        "Name": "etcd-signer",
        "Spec": {
          "ConfigMapLocations": [
            {
              "Namespace": "openshift-apiserver",
              "Name": "etcd-serving-ca"
            },
            {
              "Namespace": "openshift-config",
              "Name": "etcd-ca-bundle"
            },
          ...
          ],
          "OnDiskLocations": null,
          "CertificateMetadata": [
            {
              "CertIdentifier": {
                "CommonName": "etcd-signer",
                "SerialNumber": "8395033630537409172",
                "Issuer": {
                  "CommonName": "etcd-signer",
                  "SerialNumber": "",
                  "Issuer": null
                }
              },
              "SignatureAlgorithm": "SHA256-RSA",
              "PublicKeyAlgorithm": "RSA",
              "PublicKeyBitSize": "2048 bit",
              "ValidityDuration": "10y",
              "Usages": [
                "KeyUsageDigitalSignature",
                "KeyUsageKeyEncipherment",
                "KeyUsageCertSign"
              ],
              "ExtendedUsages": []
            }
          ]
        },
        "Status": {
          "Errors": null
        }
...
  CertKeyPairs": {
    "Items": [
      {
        "LogicalName": "",
        "Description": "",
        "Name": "metrics.openshift-apiserver-operator.svc::3074380002740555304",
        "Spec": {
          "SecretLocations": [
            {
              "Namespace": "openshift-apiserver-operator",
              "Name": "openshift-apiserver-operator-serving-cert"
            }
          ],
          "OnDiskLocations": null,
          "CertMetadata": {
            "CertIdentifier": {
              "CommonName": "metrics.openshift-apiserver-operator.svc",
              "SerialNumber": "3074380002740555304",
              "Issuer": {
                "CommonName": "openshift-service-serving-signer@1701354509",
                "SerialNumber": "",
                "Issuer": null
              }
            },
            "SignatureAlgorithm": "SHA256-RSA",
            "PublicKeyAlgorithm": "RSA",
            "PublicKeyBitSize": "2048 bit",
            "ValidityDuration": "2y",
            "Usages": [
              "KeyUsageDigitalSignature",
              "KeyUsageKeyEncipherment"
            ],
            "ExtendedUsages": [
              "ExtKeyUsageServerAuth"
            ]
          },
          "Details": {
            "CertType": "ServingCertDetails",
            "SignerDetails": null,
            "ServingCertDetails": {
              "DNSNames": [
                "metrics.openshift-apiserver-operator.svc",
                "metrics.openshift-apiserver-operator.svc.cluster.local"
              ],
              "IPAddresses": null
            },
            "ClientCertDetails": null
          }
        },
        "Status": {
          "Errors": null
        }
      },
```

This stores the following info:
* all secrets / configmaps which are TLS artifacts along with their metadata
* parses them and stores metadata
* removes locations that are versioned or hashed copies of existing TLS artifacts
* deduplicates them by content, so copies of TLS artifacts are grouped by location

## Certificate metadata

TLS artifact contents may however be insufficient - i.e. it's not clear which product component is responsible 
for the TLS artifacts lifecycle. To do that the TLS artifacts should have additional annotations.

Recently we've added the `openshift.io/owning-component: foobar` annotation to (almost) all TLS artifacts
so that issues related to this TLS artifact would be routed to a proper location. For instance, 
problems with service serving CA bundles are filed in Jira for "Networking / cluster-network-operator" component 
regardless of the repo these are created or the namespace its being created, as this component is 
injecting CA bundle.

Along with manually adding annotations `library-go` was updated to pass `JiraComponent` and `Description` 
annotations for secrets when `RotatedSelfSignedCertKeySecret`, `RotatedSigningCASecret` or 
`CABundleConfigMap` structs are used. These structs would be later updated to add more necessary 
metadata - and users are encouraged to make use of them to keep up with metadata requirements.

## Reports and violations

After the TLS registry was generated by the e2e test, it can be analyzed to ensure it matches metadata requirements.
The file in `rawTLSInfo` is copied to `tls/raw-data` in this repository so that it can be analyzed by 
the `openshift-tests update-tls-artifacts` command.

The command will parse the registry JSON and verify that TLS artifacts match a set of requirements.
The most basic is "every TLS artifact has to have an owner annotations". The processed registry JSON is 
stored in `tls/ownership/ownership.json` so that it can be machine-readable.

Along with JSON human-readable report in Markdown at `tls/ownership/ownership.md` is generated. 
This report contains a list of TLS artifacts violating this requirement and a list of TLS artifacts 
grouped by owning components along with their description. 
These kinds of reports are useful to find the owning component for specific TLS artifact.

Apart from reports the registry is checked for requirement violations. 
`make update` creates `tls/violations/ownership/ownership-violations.json` listing TLS artifact locations
  that don't have ownership annotation set. This file is meant to be "remove-only", meaning adding 
  new entries is prohibited. This is enforced by using a separate `OWNERS` file for this directory and an e2e test (see below).

## Updating TLS registry

In order to include unregistered TLS artifact or update certificate metadata fresh raw TLS info 
needs to be placed in origin's `tls/raw-data`. Raw TLS info can be obtained from test artifacts, 
usually at `openshift-e2e-test/artifacts/rawTLSInfo`. This generated JSON file contains unfiltered 
certificate data, so in order to validate it and build reports it needs to be processed by `make update` 
command. The generated files should be committed in `origin` and a new PR updating TLS registry 
should be created.

## Adding a new requirement

Reports and violations mechanisms can be extended to add new requirements. To add a new 
certificate metadata requirements developers would need to:
* Implement [`Requirement` interface](https://github.com/openshift/origin/blob/main/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces/types.go#L9-L14):
```golang
type Requirement interface {
  GetName() string

  // InspectRequirement generates and returns the result for a particular set of raw data
  InspectRequirement(rawData []*certgraphapi.PKIList) (RequirementResult, error)
}
```

Example:
```golang
const annotationName string = "certificates.openshift.io/supports-offline-hostname-change"

type SupportsOfflineHostnameChange struct{}

func NewSupportsOfflineHostnameChange() tlsmetadatainterfaces.Requirement {

    md := tlsmetadatainterfaces.NewMarkdown("")
    md.Text("Offline hostname change is an SNO feature driven using tool (provide link here) while a cluster is not running.")

    return tlsmetadatainterfaces.NewAnnotationRequirement(
        // requirement name
        "offline-hostname-change",
        // cert or configmap annotation
        annotationName,
        "Supports Offline Hostname Change",
        string(md.ExactBytes()),
    )
}
```

Markdown report can also be customized, see [example `generateOwnershipMarkdown` method](https://github.com/openshift/origin/blob/main/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/ownership/requirement.go#L71-L160) for ownership requirement.

## Enforcing requirements in tests

Along with the "collect tls artifacts" test the e2e test ensures that cluster certificates don't add 
new certificates violating requirements via the "all registered tls artifacts must have no metadata violation regressions" test.
This test builds the TLS registry from the cluster and generates new violation files from it. If this new file 
adds new violations to known violations this test would fail, safeguarding us 
from PRs that add new TLS artifacts without required metadata across the entire platform. Another 
test verifis that metadata for actual TLS artifact matches metadata for known TLS artifact locations.

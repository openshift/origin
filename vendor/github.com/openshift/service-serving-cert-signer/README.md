# OpenShift Service Serving Cert Signer Operator

This operator runs the following OpenShift controllers:
* **service-ca controller:**
  * Issues a signed serving certificate/key pair to services annotated with 'service.alpha.openshift.io/serving-cert-secret-name' via a secret. [See the current OKD documentation for usage.](https://docs.okd.io/latest/dev_guide/secrets.html#service-serving-certificate-secrets)

* **configmap-cabundle-injector controller:**
  * Watches for configmaps annotated with 'service.alpha.openshift.io/inject-cabundle=true' and adds or updates a data item (key "service-ca.crt") containing the PEM-encoded CA signing bundle. Consumers of the configmap can then trust service-ca.crt in their TLS client configuration, allowing connections to services that utilize service-serving certificates.
  * Note: Explicitly referencing the "service-ca.crt" key in a volumeMount will prevent a pod from starting until the configMap has been injected with the CA bundle (https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/#restrictions). This behavior helps ensure that pods start with the CA bundle data available.

```
$ oc create configmap foobar --from-literal=key1=foo
configmap/foobar created
$ oc get configmap/foobar -o yaml
apiVersion: v1
data:
  key1: foo
kind: ConfigMap
metadata:
  creationTimestamp: 2018-09-11T23:44:56Z
  name: foobar
  namespace: myproject
  resourceVersion: "56490"
  selfLink: /api/v1/namespaces/myproject/configmaps/foobar
  uid: afee501b-b61c-11e8-833b-c85b762603b0
$ oc annotate configmap foobar service.alpha.openshift.io/inject-cabundle="true"
configmap/foobar annotated
$ oc get configmap/foobar -o yaml
apiVersion: v1
data:
  key1: foo
  service-ca.crt: |
    -----BEGIN CERTIFICATE-----
    MIIDCjCCAfKgAwIBAgIBATANBgkqhkiG9w0BAQsFADA2MTQwMgYDVQQDDCtvcGVu
    c2hpZnQtc2VydmljZS1zZXJ2aW5nLXNpZ25lckAxNTM2Njk1NTIxMB4XDTE4MDkx
    MTE5NTIwMVoXDTIzMDkxMDE5NTIwMlowNjE0MDIGA1UEAwwrb3BlbnNoaWZ0LXNl
    cnZpY2Utc2VydmluZy1zaWduZXJAMTUzNjY5NTUyMTCCASIwDQYJKoZIhvcNAQEB
    BQADggEPADCCAQoCggEBANP9Asc657SkWVPOohmMlrXQirl7taaarmM5l3/pNgeo
    /fwkaH5KrJ9D8OxiSd5aepURrxeAk22U9eicGWRNssoe1wukE4hlLcIUlwdvElBA
    5dS0xRI3Jld3WjqisVRdjTy9O4GEWFOIhkZlrL9ZcNWe8WhiCtn447rgI1QhtZtX
    mAxUZ/mZdswQgvP0eqWOGWarC1b+RBQFo7uF0No6N4vTlpNBCxoz3CYvlpXwODYU
    4dpdpsoF6PdZ+8uMh4hVY/2w1/6qgwwe4E85RkumBwyPHQGOFKkJDF26nBLM1HGF
    +BLCcpUatISgLO9eDm1thcDvmash9HmaH7nJ+195ck0CAwEAAaMjMCEwDgYDVR0P
    AQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBABwA
    aZNHvhla0QWznreqkPkd1bUbMit4R5JbTGYk6cd37zLAWA60inwaZ0A4GFk7VVom
    Zbru3/DdhoI4ojcY26eqY0CbrhizV10mlI8Q/cdu1EKpDFwrHiwNk2rsBVbox8Es
    Quy9jgb51WIFhUy4C0aqSmc495Gg9pCxzs4cCuqJtb8OyUEUBKbxyz9lA1a7ZUpx
    BofBpbbyBRtnf27mQTyxVcZBzkHAj1Ouq0mBiXs4c3YLGbNse00MP0G6Uwtmsbev
    PCmHDAHzPvb7N9vMZ4jrqulkaN1S2H9091pH0DxA8srUl0JCuB7p03uPrxCOSAwT
    6OkzAWkPxzToypA+7fU=
    -----END CERTIFICATE-----
kind: ConfigMap
metadata:
  annotations:
    service.alpha.openshift.io/inject-cabundle: "true"
  creationTimestamp: 2018-09-11T23:44:56Z
  name: foobar
  namespace: myproject
  resourceVersion: "56606"
  selfLink: /api/v1/namespaces/myproject/configmaps/foobar
  uid: afee501b-b61c-11e8-833b-c85b762603b0
```

* **apiservice-cabundle-injector controller:**
  * Watches for apiservices annotated with 'service.alpha.openshift.io/inject-cabundle=true' and updates the apiservice spec.caBundle with a base64url-encoded CA signing bundle. This is simply an apiservice variant of the above configmap injection feature.

```
$ oc get apiservice/v1.build.openshift.io -o yaml
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"apiregistration.k8s.io/v1beta1","kind":"APIService","metadata":{"annotations":{"service.alpha.openshift.io/inject-cabundle":"true"},"name":"v1.build.openshift.io","namespace":""},"spec":{"group":"build.openshift.io","groupPriorityMinimum":9900,"service":{"name":"api","namespace":"openshift-apiserver"},"version":"v1","versionPriority":15}}
    service.alpha.openshift.io/inject-cabundle: "true"
  creationTimestamp: 2018-09-11T19:52:16Z
  name: v1.build.openshift.io
  resourceVersion: "923"
  selfLink: /apis/apiregistration.k8s.io/v1/apiservices/v1.build.openshift.io
  uid: 2f55ec88-b5fc-11e8-833b-c85b762603b0
spec:
  caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURDakNDQWZLZ0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREEyTVRRd01nWURWUVFEREN0dmNHVnUKYzJocFpuUXRjMlZ5ZG1salpTMXpaWEoyYVc1bkxYTnBaMjVsY2tBeE5UTTJOamsxTlRJeE1CNFhEVEU0TURreApNVEU1TlRJd01Wb1hEVEl6TURreE1ERTVOVEl3TWxvd05qRTBNRElHQTFVRUF3d3JiM0JsYm5Ob2FXWjBMWE5sCmNuWnBZMlV0YzJWeWRtbHVaeTF6YVdkdVpYSkFNVFV6TmpZNU5UVXlNVENDQVNJd0RRWUpLb1pJaHZjTkFRRUIKQlFBRGdnRVBBRENDQVFvQ2dnRUJBTlA5QXNjNjU3U2tXVlBPb2htTWxyWFFpcmw3dGFhYXJtTTVsMy9wTmdlbwovZndrYUg1S3JKOUQ4T3hpU2Q1YWVwVVJyeGVBazIyVTllaWNHV1JOc3NvZTF3dWtFNGhsTGNJVWx3ZHZFbEJBCjVkUzB4UkkzSmxkM1dqcWlzVlJkalR5OU80R0VXRk9JaGtabHJMOVpjTldlOFdoaUN0bjQ0N3JnSTFRaHRadFgKbUF4VVovbVpkc3dRZ3ZQMGVxV09HV2FyQzFiK1JCUUZvN3VGME5vNk40dlRscE5CQ3hvejNDWXZscFh3T0RZVQo0ZHBkcHNvRjZQZForOHVNaDRoVlkvMncxLzZxZ3d3ZTRFODVSa3VtQnd5UEhRR09GS2tKREYyNm5CTE0xSEdGCitCTENjcFVhdElTZ0xPOWVEbTF0aGNEdm1hc2g5SG1hSDduSisxOTVjazBDQXdFQUFhTWpNQ0V3RGdZRFZSMFAKQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCL3dRRk1BTUJBZjh3RFFZSktvWklodmNOQVFFTEJRQURnZ0VCQUJ3QQphWk5IdmhsYTBRV3pucmVxa1BrZDFiVWJNaXQ0UjVKYlRHWWs2Y2QzN3pMQVdBNjBpbndhWjBBNEdGazdWVm9tClpicnUzL0RkaG9JNG9qY1kyNmVxWTBDYnJoaXpWMTBtbEk4US9jZHUxRUtwREZ3ckhpd05rMnJzQlZib3g4RXMKUXV5OWpnYjUxV0lGaFV5NEMwYXFTbWM0OTVHZzlwQ3h6czRjQ3VxSnRiOE95VUVVQktieHl6OWxBMWE3WlVweApCb2ZCcGJieUJSdG5mMjdtUVR5eFZjWkJ6a0hBajFPdXEwbUJpWHM0YzNZTEdiTnNlMDBNUDBHNlV3dG1zYmV2ClBDbUhEQUh6UHZiN045dk1aNGpycXVsa2FOMVMySDkwOTFwSDBEeEE4c3JVbDBKQ3VCN3AwM3VQcnhDT1NBd1QKNk9rekFXa1B4elRveXBBKzdmVT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  group: build.openshift.io
  groupPriorityMinimum: 9900
  service:
    name: api
    namespace: openshift-apiserver
  version: v1
  versionPriority: 15
status:
  conditions:
  - lastTransitionTime: 2018-09-11T19:54:16Z
    message: all checks passed
    reason: Passed
    status: "True"
    type: Available
```

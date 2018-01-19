// Package imagequalify contains the OpenShift ImageQualify admission
// control plugin. This plugin allows administrators to set a policy
// for bare image names. A "bare" image name is a docker image
// reference that contains no domain component (e.g., "repository.io",
// "docker.io", etc).
//
// The preferred domain component to use, and hence pull from, for a
// bare image name is computed from a set of path-based pattern
// matching rules in the admission configuration:
//
// admissionConfig:
//  pluginConfig:
//    openshift.io/ImageQualify:
//      configuration:
//        kind: ImageQualifyConfig
//        apiVersion: admission.config.openshift.io/v1
//        rules:
//          - pattern: "openshift*/*"
//            domain:  "access.redhat.registry.com"
//
//          - pattern: "*"
//            domain:  "access.redhat.registry.com"
//
//          - pattern: "nginx"
//            domain:  "nginx.com"
//
//          - pattern: "repo/jenkins"
//            domain:  "jenkins-ci.org"
//
// Rule Ordering
// -------------
//
// Rules are sorted into the set of all explicit patterns (i.e., those
// with no wildcards) and the set of wildcard patterns. In each set,
// the natural order is lexicographically by pattern
// {depth,digest,tag,path}. Pattern matching is first attempted
// against the explicit rules, then the wildcard rules.
//
// As we use path-based pattern matching you should be aware of what
// looks like a fallback pattern to cover any bare image reference:
//
//          - pattern: "*"
//          - domain:  "access.redhat.registry.com"
//
// This pattern would not match "repo/jenkins" as the pattern contains
// no path segments (i.e., '/'). To match both cases you should list
// wildcard patterns that cover just image names and images in any
// repository.
//
//          - pattern: "*"
//          - domain:  "access.redhat.registry.com"
//
//          - pattern: "*/*"
//          - domain:  "access.redhat.registry.com"
//
// Additionally, patterns can also reference tags:
//
//          - pattern: "nginx:latest"
//            domain:  "nginx-dev.com"
//
//          - pattern: "nginx:*"
//            domain:  "nginx-prod.com"
//
//          - pattern: "nginx:v1.2.*"
//            domain:  "nginx-prod.com"
//
//          - pattern: "next/nginx:v2*"
//            domain:  "next/nginx-next.com"
//
// Additionally, patterns can also reference digests:
//
//          - pattern: "nginx@sha256:abc*"
//            domain:  "nginx-staging.com"
//
//          - pattern: "reppo/nginx:latest@sha256:abc*"
//            domain:  "nginx-staging.com"
//
// The plugin is configured via the ImageQualifyConfig object in the
// origin and kubernetes master configs:
//
// kubernetesMasterConfig:
//   admissionConfig:
//     pluginConfig:
//       openshift.io/ImageQualify:
//         configuration:
//           kind: ImageQualifyConfig
//           apiVersion: admission.config.openshift.io/v1
//           rules:
//             - pattern: nginx
//               domain: localhost:5000
package imagequalify

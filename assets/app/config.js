// This is the default configuration for the dev mode of the web console.
// A generated version of this config is created at run-time when running
// the web console from the openshift binary.
//
// To change configuration for local development, copy this file to
// assets/app/config.local.js and edit the copy.
window.OPENSHIFT_CONFIG = {
  apis: {
    hostPort: "localhost:8443",
    prefix: "/apis"
  },
  api: {
    openshift: {
      hostPort: "localhost:8443",
      prefixes: {
        "v1": "/oapi"
      },
      resources: {
        "buildconfigs": true,
        "buildlogoptions": true,
        "buildlogs": true,
        "buildrequests": true,
        "builds": true,
        "clusternetworks": true,
        "clusterpolicies": true,
        "clusterpolicybindings": true,
        "clusterrolebindings": true,
        "clusterroles": true,
        "deploymentconfigrollbacks": true,
        "deploymentconfigs": true,
        "groups": true,
        "hostsubnets": true,
        "identities": true,
        "images": true,
        "imagestreamimages": true,
        "imagestreammappings": true,
        "imagestreams": true,
        "imagestreamtags": true,
        "ispersonalsubjectaccessreviews": true,
        "localresourceaccessreviews": true,
        "localsubjectaccessreviews": true,
        "netnamespaces": true,
        "oauthaccesstokens": true,
        "oauthauthorizetokens": true,
        "oauthclientauthorizations": true,
        "oauthclients": true,
        "policies": true,
        "policybindings": true,
        "processedtemplates": true,
        "projectrequests": true,
        "projects": true,
        "resourceaccessreviewresponses": true,
        "resourceaccessreviews": true,
        "rolebindings": true,
        "roles": true,
        "routes": true,
        "statuses": true,
        "subjectaccessreviewresponses": true,
        "subjectaccessreviews": true,
        "templateconfigs": true,
        "templates": true,
        "useridentitymappings": true,
        "users": true
      }
    },
    k8s: {
      hostPort: "localhost:8443",
      prefixes: {
        "v1": "/api"
      },
      resources: {
        "bindings": true,
        "componentstatuses": true,
        "daemons": true,
        "deleteoptions": true,
        "endpoints": true,
        "events": true,
        "limitranges": true,
        "listoptions": true,
        "minions": true,
        "namespaces": true,
        "nodes": true,
        "persistentvolumeclaims": true,
        "persistentvolumes": true,
        "podattachoptions": true,
        "podexecoptions": true,
        "podlogoptions": true,
        "podproxyoptions": true,
        "pods": true,
        "podstatusresults": true,
        "podtemplates": true,
        "rangeallocations": true,
        "replicationcontrollers": true,
        "resourcequotas": true,
        "secrets": true,
        "securitycontextconstraints": true,
        "serializedreferences": true,
        "serviceaccounts": true,
        "services": true
      }
    }
  },
  auth: {
    oauth_authorize_uri: "https://localhost:8443/oauth/authorize",
    oauth_redirect_base: "https://localhost:9000",
    oauth_client_id: "openshift-web-console",
    logout_uri: ""
  },
  loggingURL: "",
  metricsURL: "",
  cli: {
    downloadURL: {
      "Linux (32 bits)": "https://github.com/openshift/origin/releases/download/v1.1.1/openshift-origin-client-tools-v1.1.1-e1d9873-linux-32bit.tar.gz",
      "Linux (64 bits)": "https://github.com/openshift/origin/releases/download/v1.1.1/openshift-origin-client-tools-v1.1.1-e1d9873-linux-64bit.tar.gz",
      "Windows": "https://github.com/openshift/origin/releases/download/v1.1.1/openshift-origin-client-tools-v1.1.1-e1d9873-windows.zip",
      "Mac OS X": "https://github.com/openshift/origin/releases/download/v1.1.1/openshift-origin-client-tools-v1.1.1-e1d9873-mac.zip"
    }
  }
};

// This is the default version info for the dev mode of the web console.
// A generated version of this version info is created at run-time when running
// the web console from the openshift binary.
window.OPENSHIFT_VERSION = {
  openshift: {
    major: "dev-mode",
    minor: "dev-mode",
    gitCommit: "dev-mode",
    gitVersion: "dev-mode"
  },
  kubernetes: {
    major: "dev-mode",
    minor: "dev-mode",
    gitCommit: "dev-mode",
    gitVersion: "dev-mode"
  }
}
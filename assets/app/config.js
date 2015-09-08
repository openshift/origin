// This is the default configuration for the dev mode of the web console.
// A generated version of this config is created at run-time when running
// the web console from the openshift binary
window.OPENSHIFT_CONFIG = {
  api: {
    openshift: {
      hostPort: "localhost:8443",
      prefixes: {
        "v1beta3": "/osapi",
        "*":       "/oapi"
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
      	"*": "/api"
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
  }
};

// This is the default configuration for the dev mode of the web console.
// A generated version of this config is created at run-time when running
// the web console from the openshift binary
window.OPENSHIFT_CONFIG = {
  api: {
    openshift: {
      hostPort: "localhost:8443",
      prefix: "/osapi"
    },
    k8s: {
      hostPort: "localhost:8443",
      prefix: "/api"
    }
  },
  auth: {
  	oauth_authorize_uri: "https://localhost:8443/oauth/authorize",
  	oauth_redirect_base: "https://localhost:9000",
  	oauth_client_id: "openshift-web-console",
  	logout_uri: ""
  }
};
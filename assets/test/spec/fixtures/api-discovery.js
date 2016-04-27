// Mocked API discovery
window.OPENSHIFT_CONFIG.api.k8s.resources = {
  "v1": {
    "bindings": {
      "name": "bindings",
      "namespaced": true,
      "kind": "Binding"
    },
    "componentstatuses": {
      "name": "componentstatuses",
      "namespaced": false,
      "kind": "ComponentStatus"
    },
    "configmaps": {
      "name": "configmaps",
      "namespaced": true,
      "kind": "ConfigMap"
    },
    "endpoints": {
      "name": "endpoints",
      "namespaced": true,
      "kind": "Endpoints"
    },
    "events": {
      "name": "events",
      "namespaced": true,
      "kind": "Event"
    },
    "limitranges": {
      "name": "limitranges",
      "namespaced": true,
      "kind": "LimitRange"
    },
    "namespaces": {
      "name": "namespaces",
      "namespaced": false,
      "kind": "Namespace"
    },
    "namespaces/finalize": {
      "name": "namespaces/finalize",
      "namespaced": false,
      "kind": "Namespace"
    },
    "namespaces/status": {
      "name": "namespaces/status",
      "namespaced": false,
      "kind": "Namespace"
    },
    "nodes": {
      "name": "nodes",
      "namespaced": false,
      "kind": "Node"
    },
    "nodes/proxy": {
      "name": "nodes/proxy",
      "namespaced": false,
      "kind": "Node"
    },
    "nodes/status": {
      "name": "nodes/status",
      "namespaced": false,
      "kind": "Node"
    },
    "persistentvolumeclaims": {
      "name": "persistentvolumeclaims",
      "namespaced": true,
      "kind": "PersistentVolumeClaim"
    },
    "persistentvolumeclaims/status": {
      "name": "persistentvolumeclaims/status",
      "namespaced": true,
      "kind": "PersistentVolumeClaim"
    },
    "persistentvolumes": {
      "name": "persistentvolumes",
      "namespaced": false,
      "kind": "PersistentVolume"
    },
    "persistentvolumes/status": {
      "name": "persistentvolumes/status",
      "namespaced": false,
      "kind": "PersistentVolume"
    },
    "pods": {
      "name": "pods",
      "namespaced": true,
      "kind": "Pod"
    },
    "pods/attach": {
      "name": "pods/attach",
      "namespaced": true,
      "kind": "Pod"
    },
    "pods/binding": {
      "name": "pods/binding",
      "namespaced": true,
      "kind": "Binding"
    },
    "pods/exec": {
      "name": "pods/exec",
      "namespaced": true,
      "kind": "Pod"
    },
    "pods/log": {
      "name": "pods/log",
      "namespaced": true,
      "kind": "Pod"
    },
    "pods/portforward": {
      "name": "pods/portforward",
      "namespaced": true,
      "kind": "Pod"
    },
    "pods/proxy": {
      "name": "pods/proxy",
      "namespaced": true,
      "kind": "Pod"
    },
    "pods/status": {
      "name": "pods/status",
      "namespaced": true,
      "kind": "Pod"
    },
    "podtemplates": {
      "name": "podtemplates",
      "namespaced": true,
      "kind": "PodTemplate"
    },
    "replicationcontrollers": {
      "name": "replicationcontrollers",
      "namespaced": true,
      "kind": "ReplicationController"
    },
    "replicationcontrollers/scale": {
      "name": "replicationcontrollers/scale",
      "namespaced": true,
      "kind": "Scale"
    },
    "replicationcontrollers/status": {
      "name": "replicationcontrollers/status",
      "namespaced": true,
      "kind": "ReplicationController"
    },
    "resourcequotas": {
      "name": "resourcequotas",
      "namespaced": true,
      "kind": "ResourceQuota"
    },
    "resourcequotas/status": {
      "name": "resourcequotas/status",
      "namespaced": true,
      "kind": "ResourceQuota"
    },
    "secrets": {
      "name": "secrets",
      "namespaced": true,
      "kind": "Secret"
    },
    "securitycontextconstraints": {
      "name": "securitycontextconstraints",
      "namespaced": false,
      "kind": "SecurityContextConstraints"
    },
    "serviceaccounts": {
      "name": "serviceaccounts",
      "namespaced": true,
      "kind": "ServiceAccount"
    },
    "services": {
      "name": "services",
      "namespaced": true,
      "kind": "Service"
    },
    "services/proxy": {
      "name": "services/proxy",
      "namespaced": true,
      "kind": "Service"
    },
    "services/status": {
      "name": "services/status",
      "namespaced": true,
      "kind": "Service"
    }
  }
};

window.OPENSHIFT_CONFIG.api.openshift.resources = {
  "v1": {
    "buildconfigs": {
      "name": "buildconfigs",
      "namespaced": true,
      "kind": "BuildConfig"
    },
    "buildconfigs/instantiate": {
      "name": "buildconfigs/instantiate",
      "namespaced": true,
      "kind": "BuildRequest"
    },
    "buildconfigs/instantiatebinary": {
      "name": "buildconfigs/instantiatebinary",
      "namespaced": true,
      "kind": "BinaryBuildRequestOptions"
    },
    "buildconfigs/webhooks": {
      "name": "buildconfigs/webhooks",
      "namespaced": true,
      "kind": "Status"
    },
    "builds": {
      "name": "builds",
      "namespaced": true,
      "kind": "Build"
    },
    "builds/clone": {
      "name": "builds/clone",
      "namespaced": true,
      "kind": "BuildRequest"
    },
    "builds/details": {
      "name": "builds/details",
      "namespaced": true,
      "kind": "Build"
    },
    "builds/log": {
      "name": "builds/log",
      "namespaced": true,
      "kind": "BuildLog"
    },
    "clusternetworks": {
      "name": "clusternetworks",
      "namespaced": false,
      "kind": "ClusterNetwork"
    },
    "clusterpolicies": {
      "name": "clusterpolicies",
      "namespaced": false,
      "kind": "ClusterPolicy"
    },
    "clusterpolicybindings": {
      "name": "clusterpolicybindings",
      "namespaced": false,
      "kind": "ClusterPolicyBinding"
    },
    "clusterrolebindings": {
      "name": "clusterrolebindings",
      "namespaced": false,
      "kind": "ClusterRoleBinding"
    },
    "clusterroles": {
      "name": "clusterroles",
      "namespaced": false,
      "kind": "ClusterRole"
    },
    "deploymentconfigrollbacks": {
      "name": "deploymentconfigrollbacks",
      "namespaced": true,
      "kind": "DeploymentConfigRollback"
    },
    "deploymentconfigs": {
      "name": "deploymentconfigs",
      "namespaced": true,
      "kind": "DeploymentConfig"
    },
    "deploymentconfigs/log": {
      "name": "deploymentconfigs/log",
      "namespaced": true,
      "kind": "DeploymentLog"
    },
    "deploymentconfigs/scale": {
      "name": "deploymentconfigs/scale",
      "namespaced": true,
      "kind": "Scale"
    },
    "generatedeploymentconfigs": {
      "name": "generatedeploymentconfigs",
      "namespaced": true,
      "kind": "DeploymentConfig"
    },
    "groups": {
      "name": "groups",
      "namespaced": false,
      "kind": "Group"
    },
    "hostsubnets": {
      "name": "hostsubnets",
      "namespaced": false,
      "kind": "HostSubnet"
    },
    "identities": {
      "name": "identities",
      "namespaced": false,
      "kind": "Identity"
    },
    "images": {
      "name": "images",
      "namespaced": false,
      "kind": "Image"
    },
    "imagestreamimages": {
      "name": "imagestreamimages",
      "namespaced": true,
      "kind": "ImageStreamImage"
    },
    "imagestreamimports": {
      "name": "imagestreamimports",
      "namespaced": true,
      "kind": "ImageStreamImport"
    },
    "imagestreammappings": {
      "name": "imagestreammappings",
      "namespaced": true,
      "kind": "ImageStreamMapping"
    },
    "imagestreams": {
      "name": "imagestreams",
      "namespaced": true,
      "kind": "ImageStream"
    },
    "imagestreams/secrets": {
      "name": "imagestreams/secrets",
      "namespaced": true,
      "kind": "SecretList"
    },
    "imagestreams/status": {
      "name": "imagestreams/status",
      "namespaced": true,
      "kind": "ImageStream"
    },
    "imagestreamtags": {
      "name": "imagestreamtags",
      "namespaced": true,
      "kind": "ImageStreamTag"
    },
    "localresourceaccessreviews": {
      "name": "localresourceaccessreviews",
      "namespaced": true,
      "kind": "LocalResourceAccessReview"
    },
    "localsubjectaccessreviews": {
      "name": "localsubjectaccessreviews",
      "namespaced": true,
      "kind": "LocalSubjectAccessReview"
    },
    "netnamespaces": {
      "name": "netnamespaces",
      "namespaced": false,
      "kind": "NetNamespace"
    },
    "oauthaccesstokens": {
      "name": "oauthaccesstokens",
      "namespaced": false,
      "kind": "OAuthAccessToken"
    },
    "oauthauthorizetokens": {
      "name": "oauthauthorizetokens",
      "namespaced": false,
      "kind": "OAuthAuthorizeToken"
    },
    "oauthclientauthorizations": {
      "name": "oauthclientauthorizations",
      "namespaced": false,
      "kind": "OAuthClientAuthorization"
    },
    "oauthclients": {
      "name": "oauthclients",
      "namespaced": false,
      "kind": "OAuthClient"
    },
    "policies": {
      "name": "policies",
      "namespaced": true,
      "kind": "Policy"
    },
    "policybindings": {
      "name": "policybindings",
      "namespaced": true,
      "kind": "PolicyBinding"
    },
    "processedtemplates": {
      "name": "processedtemplates",
      "namespaced": true,
      "kind": "Template"
    },
    "projectrequests": {
      "name": "projectrequests",
      "namespaced": false,
      "kind": "ProjectRequest"
    },
    "projects": {
      "name": "projects",
      "namespaced": false,
      "kind": "Project"
    },
    "resourceaccessreviews": {
      "name": "resourceaccessreviews",
      "namespaced": true,
      "kind": "ResourceAccessReview"
    },
    "rolebindings": {
      "name": "rolebindings",
      "namespaced": true,
      "kind": "RoleBinding"
    },
    "roles": {
      "name": "roles",
      "namespaced": true,
      "kind": "Role"
    },
    "routes": {
      "name": "routes",
      "namespaced": true,
      "kind": "Route"
    },
    "routes/status": {
      "name": "routes/status",
      "namespaced": true,
      "kind": "Route"
    },
    "subjectaccessreviews": {
      "name": "subjectaccessreviews",
      "namespaced": true,
      "kind": "SubjectAccessReview"
    },
    "templates": {
      "name": "templates",
      "namespaced": true,
      "kind": "Template"
    },
    "useridentitymappings": {
      "name": "useridentitymappings",
      "namespaced": false,
      "kind": "UserIdentityMapping"
    },
    "users": {
      "name": "users",
      "namespaced": false,
      "kind": "User"
    }
  }
};

window.OPENSHIFT_CONFIG.apis.groups = {
  "autoscaling": {
    "name": "autoscaling",
    "preferredVersion": "v1",
    "versions": {
      "v1": {
        "version": "v1",
        "groupVersion": "autoscaling/v1",
        "resources": {
          "horizontalpodautoscalers": {
            "name": "horizontalpodautoscalers",
            "namespaced": true,
            "kind": "HorizontalPodAutoscaler"
          },
          "horizontalpodautoscalers/status": {
            "name": "horizontalpodautoscalers/status",
            "namespaced": true,
            "kind": "HorizontalPodAutoscaler"
          }
        }
      }
    }
  },
  "batch": {
    "name": "batch",
    "preferredVersion": "v1",
    "versions": {
      "v1": {
        "version": "v1",
        "groupVersion": "batch/v1",
        "resources": {
          "jobs": {
            "name": "jobs",
            "namespaced": true,
            "kind": "Job"
          },
          "jobs/status": {
            "name": "jobs/status",
            "namespaced": true,
            "kind": "Job"
          }
        }
      }
    }
  },
  "extensions": {
    "name": "extensions",
    "preferredVersion": "v1beta1",
    "versions": {
      "v1beta1": {
        "version": "v1beta1",
        "groupVersion": "extensions/v1beta1",
        "resources": {
          "daemonsets": {
            "name": "daemonsets",
            "namespaced": true,
            "kind": "DaemonSet"
          },
          "daemonsets/status": {
            "name": "daemonsets/status",
            "namespaced": true,
            "kind": "DaemonSet"
          },
          "deployments": {
            "name": "deployments",
            "namespaced": true,
            "kind": "Deployment"
          },
          "deployments/rollback": {
            "name": "deployments/rollback",
            "namespaced": true,
            "kind": "DeploymentRollback"
          },
          "deployments/scale": {
            "name": "deployments/scale",
            "namespaced": true,
            "kind": "Scale"
          },
          "deployments/status": {
            "name": "deployments/status",
            "namespaced": true,
            "kind": "Deployment"
          },
          "horizontalpodautoscalers": {
            "name": "horizontalpodautoscalers",
            "namespaced": true,
            "kind": "HorizontalPodAutoscaler"
          },
          "horizontalpodautoscalers/status": {
            "name": "horizontalpodautoscalers/status",
            "namespaced": true,
            "kind": "HorizontalPodAutoscaler"
          },
          "ingresses": {
            "name": "ingresses",
            "namespaced": true,
            "kind": "Ingress"
          },
          "ingresses/status": {
            "name": "ingresses/status",
            "namespaced": true,
            "kind": "Ingress"
          },
          "jobs": {
            "name": "jobs",
            "namespaced": true,
            "kind": "Job"
          },
          "jobs/status": {
            "name": "jobs/status",
            "namespaced": true,
            "kind": "Job"
          },
          "replicasets": {
            "name": "replicasets",
            "namespaced": true,
            "kind": "ReplicaSet"
          },
          "replicasets/scale": {
            "name": "replicasets/scale",
            "namespaced": true,
            "kind": "Scale"
          },
          "replicasets/status": {
            "name": "replicasets/status",
            "namespaced": true,
            "kind": "ReplicaSet"
          },
          "replicationcontrollers": {
            "name": "replicationcontrollers",
            "namespaced": true,
            "kind": "ReplicationControllerDummy"
          },
          "replicationcontrollers/scale": {
            "name": "replicationcontrollers/scale",
            "namespaced": true,
            "kind": "Scale"
          }
        }
      }
    }
  }
};
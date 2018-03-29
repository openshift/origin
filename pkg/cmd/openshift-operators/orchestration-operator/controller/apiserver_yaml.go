package controller

const apiserverCRD = `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: openshiftapiserverconfigs.operator.openshift.io
spec:
  scope: Cluster
  group: operator.openshift.io
  version: v1
  names:
    kind: OpenShiftAPIServerConfig
    plural: openshiftapiserverconfigs
    singular: openshiftapiserverconfig
`

const apiserverConfig = `apiVersion: operator.openshift.io/v1
kind: OpenShiftAPIServerConfig
metadata:
  name: instance
spec:
  imagePullSpec: openshift/origin:v3.10
  version: 3.10.0
  apiServerConfig:
    logLevel: 2
    port: 8445
    hostPath: ${OPENSHIFT_APISERVER_CONFIG_HOST_PATH}
`

const apiserverClusterRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  namespace: openshift-core-operators
  name: apiserver-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: apiserver-operator
  namespace: openshift-core-operators
`

const apiserverDeploymentYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: openshift-core-operators
  name: apiserver-operator
  labels:
    openshift.io/operator: "true"
    openshift.io/operator-name: "apiserver"
spec:
  replicas: 1
  selector:
    matchLabels:
      openshift.io/operator: "true"
      openshift.io/operator-name: "apiserver"
  template:
    metadata:
      name: apiserver-operator
      labels:
        openshift.io/operator: "true"
        openshift.io/operator-name: "apiserver"
    spec:
      serviceAccountName: apiserver-operator
      restartPolicy: Always
      containers:
      - name: apiserver-operator
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command: ["hypershift", "experimental", "openshift-apiserver-operator"]
        args:
        - "-v=5"
`

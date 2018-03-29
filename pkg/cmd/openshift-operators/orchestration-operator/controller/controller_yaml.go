package controller

const controllerCRD = `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: openshiftcontrollerconfigs.operator.openshift.io
spec:
  scope: Cluster
  group: operator.openshift.io
  version: v1
  names:
    kind: OpenShiftControllerConfig
    plural: openshiftcontrollerconfigs
    singular: openshiftcontrollerconfig
`

const controllerConfig = `apiVersion: operator.openshift.io/v1
kind: OpenShiftControllerConfig
metadata:
  name: instance
spec:
  imagePullSpec: openshift/origin:v3.10
  version: 3.10.0
  apiServerConfig:
    logLevel: 2
    port: 8444
    hostPath: ${OPENSHIFT_CONTROLLER_MANAGER_CONFIG_HOST_PATH}
`

const controllerClusterRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  namespace: openshift-core-operators
  name: controller-manager-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: controller-manager-operator
  namespace: openshift-core-operators
`

const controllerDeploymentYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: openshift-core-operators
  name: controller-manager-operator
  labels:
    openshift.io/operator: "true"
    openshift.io/operator-name: "controller-manager"
spec:
  replicas: 1
  selector:
    matchLabels:
      openshift.io/operator: "true"
      openshift.io/operator-name: "controller-manager"
  template:
    metadata:
      name: controller-manager-operator
      labels:
        openshift.io/operator: "true"
        openshift.io/operator-name: "controller-manager"
    spec:
      serviceAccountName: controller-manager-operator
      restartPolicy: Always
      containers:
      - name: controller-manager-operator
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command: ["hypershift", "experimental", "openshift-controller-operator"]
        args:
        - "-v=5"
`

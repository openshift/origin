package controller

const webconsoleCRD = `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: openshiftwebconsoleconfigs.operator.openshift.io
spec:
  scope: Cluster
  group: operator.openshift.io
  version: v1
  names:
    kind: OpenShiftWebConsoleConfig
    plural: openshiftwebconsoleconfigs
    singular: openshiftwebconsoleconfig
`

const webconsoleConfig = `apiVersion: operator.openshift.io/v1
kind: OpenShiftWebConsoleConfig
metadata:
  name: instance
spec:
  imagePullSpec: openshift/origin-web-console:v3.10
  version: 3.10.0
  replicas: 1
`

const webconsoleClusterRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  namespace: openshift-core-operators
  name: webconsole-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: webconsole-operator
  namespace: openshift-core-operators
`

const webconsoleDeploymentYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: openshift-core-operators
  name: webconsole-operator
  labels:
    openshift.io/operator: "true"
    openshift.io/operator-name: "webconsole"
spec:
  replicas: 1
  selector:
    matchLabels:
      openshift.io/operator: "true"
      openshift.io/operator-name: "webconsole"
  template:
    metadata:
      name: webconsole-operator
      labels:
        openshift.io/operator: "true"
        openshift.io/operator-name: "webconsole"
    spec:
      serviceAccountName: webconsole-operator
      restartPolicy: Always
      containers:
      - name: webconsole-operator
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command: ["hypershift", "experimental", "openshift-webconsole-operator"]
        args:
        - "-v=5"
`

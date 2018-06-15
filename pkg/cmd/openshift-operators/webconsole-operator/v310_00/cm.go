package v310_00

const ConfigConfigMapKey = "webconsole-config.yaml"

const ConfigMapYaml = `apiVersion: v1
kind: ConfigMap
metadata:
  namespace: openshift-web-console
  name: webconsole-config
data:
  webconsole-config.yaml:
`

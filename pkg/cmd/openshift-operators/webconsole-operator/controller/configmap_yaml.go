package controller

const configConfigMapName = ""
const configConfigMapKey = "webconsole-config.yaml"

const configMapYaml = `apiVersion: v1
kind: ConfigMap
metadata:
  namespace: openshift-web-console
  name: webconsole-config
data:
  webconsole-config.yaml:
`

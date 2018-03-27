package controller

const configConfigMap10Key = "webconsole-config.yaml"

const configMap10Yaml = `apiVersion: v1
kind: ConfigMap
metadata:
  namespace: openshift-web-console
  name: webconsole-config
data:
  webconsole-config.yaml:
`

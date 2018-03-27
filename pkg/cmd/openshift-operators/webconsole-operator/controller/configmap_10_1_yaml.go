package controller

const configConfigMap10_1Key = "web-console-config.yaml"

const configMap10_1Yaml = `apiVersion: v1
kind: ConfigMap
metadata:
  namespace: openshift-web-console
  name: web-console-config
data:
  web-console-config.yaml:
`

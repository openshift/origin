package controller

const nsYaml = `apiVersion: v1
kind: Namespace
metadata:
  name: openshift-controller-manager
  labels:
    "openshift.io/run-level": "1"
`

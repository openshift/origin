package controller

const nsYaml = `apiVersion: v1
kind: Namespace
metadata:
  name: openshift-apiserver
  labels:
    "openshift.io/run-level": "1"
`

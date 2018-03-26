package controller

const serviceYaml = `apiVersion: v1
kind: Service
metadata:
  namespace: openshift-apiserver
  name: api
  annotations:
    service.alpha.openshift.io/serving-cert-secret-name: apiserver-serving-cert
spec:
  selector:
    openshift.io/component: api
  ports:
  - name: https
    port: 443
    targetPort: 8445
`

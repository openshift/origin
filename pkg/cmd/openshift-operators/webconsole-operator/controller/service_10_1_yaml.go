package controller

const service10_1Yaml = `apiVersion: v1
kind: Service
metadata:
  namespace: openshift-web-console
  # the apiserver missed, so this piece still uses the old name
  name: webconsole
  labels:
    app: openshift-web-console
  annotations:
    service.alpha.openshift.io/serving-cert-secret-name: web-console-serving-cert
    prometheus.io/scrape: "true"
    prometheus.io/scheme: https
spec:
  selector:
    web-console: "true"
  ports:
  - name: https
    port: 443
    targetPort: 8443
`

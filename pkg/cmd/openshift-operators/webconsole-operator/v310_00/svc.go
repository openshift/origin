package v310_00

const ServiceYaml = `apiVersion: v1
kind: Service
metadata:
  namespace: openshift-web-console
  name: webconsole
  labels:
    app: openshift-web-console
  annotations:
    service.alpha.openshift.io/serving-cert-secret-name: webconsole-serving-cert
    prometheus.io/scrape: "true"
    prometheus.io/scheme: https
spec:
  selector:
    webconsole: "true"
  ports:
  - name: https
    port: 443
    targetPort: 8443
`

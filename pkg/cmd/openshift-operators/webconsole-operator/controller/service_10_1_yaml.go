package controller

const service10_1_pre10_2APIServerYaml = `apiVersion: v1
kind: Service
metadata:
  namespace: openshift-web-console
  # the apiserver missed, so this piece still uses the old name
  name: webconsole
  labels:
    app: openshift-web-console
  annotations:
    service.alpha.openshift.io/serving-cert-secret-name: webconsole-serving-cert
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

const service10_1_post10_2APIServerYaml = `apiVersion: v1
kind: Service
metadata:
  namespace: openshift-web-console
  # the apiserver missed, so this piece still uses the old name
  name: web-console
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

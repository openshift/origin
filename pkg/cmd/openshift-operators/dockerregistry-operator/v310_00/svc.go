package v310_00

const ServiceYaml = `apiVersion: v1
kind: Service
metadata:
  namespace: openshift-docker-registry
  name: docker-registry
  labels:
    app: openshift-docker-registry
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/scheme: http
spec:
  ports:
  - name: 5000-tcp
    port: 5000
    protocol: TCP
    targetPort: 5000
  selector:
    docker-registry: default
  sessionAffinity: ClientIP
  type: ClusterIP
`

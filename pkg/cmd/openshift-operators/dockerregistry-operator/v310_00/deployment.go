package v310_00

const DeploymentYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: default
  name: docker-registry
  labels:
    app: openshift-docker-registry
    webconsole: "true"
spec:
  replicas: 1
  strategy:
    type: Rolling
  selector:
    matchLabels:
      app: openshift-docker-registry
  template:
    metadata:
      name: docker-registry
      labels:
        app: openshift-docker-registry
		docker-registry: default
    spec:
      serviceAccountName: registry
      containers:
	  - env:
	    - name: REGISTRY_HTTP_ADDR
		  value: :5000
		- name: REGISTRY_HTTP_NET
		  value: tcp
        - name: REGISTRY_MIDDLEWARE_REPOSITORY_OPENSHIFT_ENFORCEQUOTA
          value: "false"
		- name: REGISTRY_CONFIGURATION_PATH
		  value: /etc/docker/registry/config.yml
      - name: registry
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 5000
		  protocol: TCP
        readinessProbe:
          httpGet:
            path: /healthz
            port: 5000
            scheme: HTTP
        livenessProbe:
          httpGet:
            path: /healthz
            port: 5000
            scheme: HTTP
        resources:
          requests:
            cpu: 100m
            memory: 250Mi
		volumeMounts:
		- mountPath: /registry
	      name: registry-storage
		- mountPath: /etc/docker/registry
		  name: config
	  restartPolicy: Always
	  serviceAccount: registry
	  serviceAccountName: registry
      volumes:
      - name: registry-storage
	    emptyDir: {}
      - name: config
        secret:
          defaultMode: 440
          name: registry-config
  triggers:
  - type: ConfigChange
`

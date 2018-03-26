package controller

const dsYaml = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  namespace: openshift-apiserver
  name: openshift-apiserver
  labels:
    openshift.io/control-plane: "true"
    openshift.io/component: api
spec:
  selector:
    matchLabels:
      openshift.io/control-plane: "true"
      openshift.io/component: api
  template:
    metadata:
      name: openshift-apiserver
      labels:
        openshift.io/control-plane: "true"
        openshift.io/component: api
    spec:
      serviceAccountName: openshift-apiserver
      restartPolicy: Always
      hostNetwork: true
      containers:
      - name: apiserver
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command: ["hypershift", "openshift-apiserver"]
        args:
        - "--config=/etc/origin/master/master-config.yaml"
        ports:
        - containerPort: 8445
        securityContext:
          privileged: true
          runAsUser: 0
        volumeMounts:
         - mountPath: /etc/origin/master/
           name: master-config
         - mountPath: /etc/origin/cloudprovider/
           name: master-cloud-provider
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8445
            scheme: HTTPS
      # sensitive files still sit on disk for now
      volumes:
      - name: master-config
        hostPath:
          path: /unlikely/path/to/override
      - name: master-cloud-provider
        hostPath:
          path: /etc/origin/cloudprovider
`

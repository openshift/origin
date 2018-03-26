package controller

const dsYaml = `apiVersion: apps/v1
kind: DaemonSet
metadata:
  namespace: openshift-controller-manager
  name: openshift-controller-manager
  labels:
    openshift.io/control-plane: "true"
    openshift.io/component: controller
spec:
  selector:
    matchLabels:
      openshift.io/control-plane: "true"
      openshift.io/component: controller
  template:
    metadata:
      name: openshift-controller-manager
      labels:
        openshift.io/control-plane: "true"
        openshift.io/component: controller
    spec:
      serviceAccountName: openshift-controller-manager
      restartPolicy: Always
      hostNetwork: true
      containers:
      - name: controller-manager
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command: ["hypershift", "openshift-controller-manager"]
        args:
        - "--config=/etc/origin/master/master-config.yaml"
        ports:
        - containerPort: 8444
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
            port: 8444
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

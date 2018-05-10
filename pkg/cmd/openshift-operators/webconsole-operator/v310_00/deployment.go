package v310_00

const DeploymentYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: openshift-web-console
  name: webconsole
  labels:
    app: openshift-web-console
    webconsole: "true"
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: openshift-web-console
      webconsole: "true"
  template:
    metadata:
      name: webconsole
      labels:
        app: openshift-web-console
        webconsole: "true"
    spec:
      serviceAccountName: webconsole
      containers:
      - name: webconsole
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command:
        - "/usr/bin/origin-web-console"
        - "--audit-log-path=-"
        - "--config=/var/webconsole-config/webconsole-config.yaml"
        ports:
        - containerPort: 8443
        volumeMounts:
        - mountPath: /var/serving-cert
          name: serving-cert
        - mountPath: /var/webconsole-config
          name: webconsole-config
        readinessProbe:
          httpGet:
            path: /healthz
            port: 8443
            scheme: HTTPS
        livenessProbe:
          exec:
            command:
              - /bin/sh
              - -i
              - -c
              - |-
                if [[ ! -f /tmp/webconsole-config.hash ]]; then \
                  md5sum /var/webconsole-config/webconsole-config.yaml > /tmp/webconsole-config.hash; \
                elif [[ $(md5sum /var/webconsole-config/webconsole-config.yaml) != $(cat /tmp/webconsole-config.hash) ]]; then \
                  exit 1; \
                fi && curl -k -f https://0.0.0.0:8443/console/
        resources:
          requests:
            cpu: 100m
            memory: 100Mi
      volumes:
      - name: serving-cert
        secret:
          defaultMode: 400
          secretName: webconsole-serving-cert
      - name: webconsole-config
        configMap:
          defaultMode: 440
          name: webconsole-config
`

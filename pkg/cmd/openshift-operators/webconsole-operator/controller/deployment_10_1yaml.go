package controller

const deployment10_1Yaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: openshift-web-console
  name: web-console
  labels:
    app: openshift-web-console
    web-console: "true"
spec:
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: openshift-web-console
      web-console: "true"
  template:
    metadata:
      name: web-console
      labels:
        app: openshift-web-console
        web-console: "true"
    spec:
      serviceAccountName: web-console
      containers:
      - name: web-console
        image: ${IMAGE}
        imagePullPolicy: IfNotPresent
        command:
        - "/usr/bin/origin-web-console"
        - "--audit-log-path=-"
        - "--config=/var/web-console-config/web-console-config.yaml"
        ports:
        - containerPort: 8443
        volumeMounts:
        - mountPath: /var/serving-cert
          name: serving-cert
        - mountPath: /var/web-console-config
          name: web-console-config
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
                if [[ ! -f /tmp/web-console-config.hash ]]; then \
                  md5sum /var/web-console-config/web-console-config.yaml > /tmp/web-console-config.hash; \
                elif [[ $(md5sum /var/web-console-config/web-console-config.yaml) != $(cat /tmp/web-console-config.hash) ]]; then \
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
          secretName: web-console-serving-cert
      - name: web-console-config
        configMap:
          defaultMode: 440
          name: web-console-config
`

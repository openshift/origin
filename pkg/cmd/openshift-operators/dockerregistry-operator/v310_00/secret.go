package v310_00

const ConfigSecretKey = "docker-registry-config.yaml"

const SecretYaml = `apiVersion: v1
kind: Secret
metadata:
  namespace: default
  name: registry-config
data:
  config.yml:
type: Opaque
`

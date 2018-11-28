package openshiftkubeapiserver

const defaultAuditPolicy = `
apiVersion: audit.k8s.io/v1beta1
kind: Policy
# Don't generate audit events for all requests in RequestReceived stage.
omitStages:
  - "RequestReceived"
rules:
  # Don't log requests for events
  - level: None
    verbs: ["*"]
    resources:
    - group: ""
      resources: ["events"]

  # Don't log authenticated requests to certain non-resource URL paths.
  - level: None
    userGroups: ["system:authenticated", "system:unauthenticated"]
    nonResourceURLs:
    - "/api*" # Wildcard matching.
    - "/version"
    - "/healthz"


  # A catch-all rule to log all other requests at the Metadata level.
  - level: Metadata
    # Long-running requests like watches that fall under this rule will not
    # generate an audit event in RequestReceived.
    omitStages:
      - "RequestReceived"
`

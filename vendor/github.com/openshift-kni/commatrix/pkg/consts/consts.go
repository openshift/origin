package consts

const (
	DefaultAddressType      = "IPv4"
	IngressLabel            = "Ingress"
	OptionalLabel           = "optional"
	OptionalTrue            = "true"
	RoleLabel               = "node-role.kubernetes.io/"
	DefaultDebugNamespace   = "openshift-commatrix-debug"
	DefaultDebugPodImage    = "openshift/tools:latest"
	FilesDefaultFormat      = "csv"
	CommatrixFileNamePrefix = "communication-matrix"
	SSMatrixFileNamePrefix  = "ss-generated-matrix"
	CommatrixDefaultDir     = "communication-matrix"
	SSRawTCP                = "raw-ss-tcp"
	SSRawUDP                = "raw-ss-udp"
	MatrixDiffSSfileName    = "matrix-diff-ss"
)

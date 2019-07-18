package network

const (
	// Maximum VXLAN Virtual Network Identifier(VNID) as per RFC#7348
	MaxVNID = uint32((1 << 24) - 1)
	// VNID: 1 to 9 are internally reserved for any special cases in the future
	MinVNID = uint32(10)
	// VNID: 0 reserved for default namespace and can reach any network in the cluster
	GlobalVNID = uint32(0)
)

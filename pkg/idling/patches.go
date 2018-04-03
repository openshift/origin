package idling

var (
	// IdlerPatchData is the patch that represents setting wantIdled to true
	IdlePatchData = []byte(`[{"op": "replace", "path": "/spec/wantIdle", "value": true}]`)
	// UnidlePatchData is the patch that represents setting wantIdled to false
	UnidlePatchData = []byte(`[{"op": "replace", "path": "/spec/wantIdle", "value": false}]`)
)

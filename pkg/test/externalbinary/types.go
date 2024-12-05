package externalbinary

type ExtensionTestSpecs []*ExtensionTestSpec

type ExtensionTestSpec struct { // TODO: convert to OTE ExtensionTestSpec format
	Name   string
	Labels string
	Binary string
}

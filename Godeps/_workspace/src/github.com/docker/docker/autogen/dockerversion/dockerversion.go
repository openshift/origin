package dockerversion

var (
	GITCOMMIT = "7d2188f9955d3f2002ff8c2e566ef84121de5217"
	VERSION   = "1.5"

	IAMSTATIC string // whether or not Docker itself was compiled statically via ./hack/make.sh binary ("true" or not "true")
	INITSHA1  string // sha1sum of separate static dockerinit, if Docker itself was compiled dynamically via ./hack/make.sh dynbinary
	INITPATH  string // custom location to search for a valid dockerinit binary (available for packagers as a last resort escape hatch)
)

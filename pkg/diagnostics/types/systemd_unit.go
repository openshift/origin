package types

type SystemdUnit struct {
	Name       string
	Exists     bool
	Enabled    bool
	Active     bool
	ExitStatus int
}

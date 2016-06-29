package types

// SystemdUnit represents the information we gather about a single systemd unit of interest.
type SystemdUnit struct {
	// The systemd unit name, e.g. "openshift-master"
	Name string
	// Whether it is present on the system at all
	Exists bool
	// Whether it is enabled (starts on its own at boot)
	Enabled bool
	// Whether it is currently started (and not crashed)
	Active bool
	// If it's not active, the exit code from its last execution
	ExitStatus int
}

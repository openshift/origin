package rsync

// NewDefaultCopyStrategy returns a copy strategy that to uses rsync and falls back to tar if needed.
func NewDefaultCopyStrategy(o *RsyncOptions) CopyStrategy {
	strategies := copyStrategies{}
	if hasLocalRsync() {
		if isWindows() {
			strategies = append(strategies, NewRsyncDaemonStrategy(o))
		} else {
			strategies = append(strategies, NewRsyncStrategy(o))
		}
	} else {
		warnNoRsync(o.ErrOut)
	}
	return append(strategies, NewTarStrategy(o))
}

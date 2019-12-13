package enablement

func ForceOpenShift() {
	isOpenShift = true
}

var isOpenShift = false

func IsOpenShift() bool {
	return isOpenShift
}

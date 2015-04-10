// +build !linux !cgo

package procfs

func ticks() float64 {
	return 100.0
}

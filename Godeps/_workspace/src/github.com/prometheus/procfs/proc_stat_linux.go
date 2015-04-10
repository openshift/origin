// +build cgo,linux

package procfs

// #include <unistd.h>
import "C"

func ticks() float64 {
	return float64(C.sysconf(C._SC_CLK_TCK)) // most likely 100
}

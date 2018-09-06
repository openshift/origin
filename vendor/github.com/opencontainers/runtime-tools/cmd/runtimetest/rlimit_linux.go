package main

import "fmt"

// These values map to rlimit constants defined in linux
const (
	RlimitCPU        = iota // CPU time in sec
	RlimitFsize             // Maximum filesize
	RlimitData              // max data size
	RlimitStack             // max stack size
	RlimitCore              // max core file size
	RlimitRss               // max resident set size
	RlimitNproc             // max number of processes
	RlimitNofile            // max number of open files
	RlimitMemlock           // max locked-in-memory address space
	RlimitAs                // address space limit
	RlimitLocks             // maximum file locks held
	RlimitSigpending        // max number of pending signals
	RlimitMsgqueue          // maximum bytes in POSIX mqueues
	RlimitNice              // max nice prio allowed to raise to
	RlimitRtprio            // maximum realtime priority
	RlimitRttime            // timeout for RT tasks in us
)

var rlimitMap = map[string]int{
	"RLIMIT_CPU":       RlimitCPU,
	"RLIMIT_FSIZE":     RlimitFsize,
	"RLIMIT_DATA":      RlimitData,
	"RLIMIT_STACK":     RlimitStack,
	"RLIMIT_CORE":      RlimitCore,
	"RLIMIT_RSS":       RlimitRss,
	"RLIMIT_NPROC":     RlimitNproc,
	"RLIMIT_NOFILE":    RlimitNofile,
	"RLIMIT_MEMLOCK":   RlimitMemlock,
	"RLIMIT_AS":        RlimitAs,
	"RLIMIT_LOCKS":     RlimitLocks,
	"RLIMIT_SGPENDING": RlimitSigpending,
	"RLIMIT_MSGQUEUE":  RlimitMsgqueue,
	"RLIMIT_NICE":      RlimitNice,
	"RLIMIT_RTPRIO":    RlimitRtprio,
	"RLIMIT_RTTIME":    RlimitRttime,
}

func strToRlimit(key string) (int, error) {
	rl, ok := rlimitMap[key]
	if !ok {
		return 0, fmt.Errorf("Wrong rlimit value: %s", key)
	}
	return rl, nil
}

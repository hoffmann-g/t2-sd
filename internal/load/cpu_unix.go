//go:build unix

package load

import (
	"syscall"
	"time"
)

// cpuTime returns the CPU time, user plus system, consumed by this process.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

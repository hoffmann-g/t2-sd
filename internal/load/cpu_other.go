//go:build !unix

package load

import "time"

// cpuTime is not available on this platform: the summary reports 0.
func cpuTime() time.Duration { return 0 }

package timing

import (
    "time"
    "math/rand"
)

// Timeout functions to start and stop the timer
func Timeout(d int, timer *time.Timer, r1 *rand.Rand) *time.Timer {
	StopTimeout(timer)
	duration := float32(r1.Intn(d*1000)/1000)
    timeout := time.NewTimer(time.Millisecond * time.Duration(1000+duration))
    return timeout
}

func StopTimeout(timer *time.Timer) {
    if timer != nil {
        timer.Stop()
    }
}
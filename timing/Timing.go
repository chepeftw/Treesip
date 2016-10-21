package timing

import (
    "time"
    "math/rand"
)

var s1 = rand.NewSource(time.Now().UnixNano())
var r1 = rand.New(s1)

// Timeout functions to start and stop the timer
func Timeout(d int, timer *time.Timer) *time.Timer {
	StopTimeout(timer)
	duration := float32(r1.Intn(d*1000)/1000)
    timeout := time.NewTimer(time.Millisecond * time.Duration(500+duration))
    return timeout
}

func StopTimeout(timer *time.Timer) {
    if timer != nil {
        timer.Stop()
    }
}
package main

import (
    "time"
    "math/rand"
)

// Timeout functions to start and stop the timer
func startTimeout(d int, timer *time.Timer, r1 *rand.Rand) *time.Timer {
	stopTimeout(timer)
	duration := float32(r1.Intn(d*1000)/1000)
    timeout := time.NewTimer(time.Millisecond * time.Duration(1000+duration))
    return timeout
}

func stopTimeout(timer *time.Timer) {
    if timer != nil {
        timer.Stop()
    }
}
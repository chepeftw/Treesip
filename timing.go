package main

import (
    "time"
    "math/rand"
)

// Timeout functions to start and stop the timer
func startTimeout(d int, r1 *rand.Rand) *time.Timer {
	duration := float32(r1.Intn(d*1000)/1000)
    timeout := time.NewTimer(time.Millisecond * time.Duration(1000+duration))
    return timeout
}

func startTimeoutF(d float32, r1 *rand.Rand) *time.Timer {
    timeout := time.NewTimer(time.Millisecond * time.Duration(1000+d))
    return timeout
}

func stopTimeout(timer *time.Timer) {
    if timer != nil {
        timer.Stop()
    }
}
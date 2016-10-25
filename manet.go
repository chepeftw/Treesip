package treesip

// Accumulating the monitored value
func functionValue( acc float32 ) float32 {
    v := float32(50.0)

    if acc > 0 {
        v += acc
    }

    return v
}

func aggregateValue( v float32, o int, acc float32, obs int ) (float32, int) {
    acc += v
    obs += o

    return acc, obs
}
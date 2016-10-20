package manet

// Accumulating the monitored value
func FunctionValue( acc float32 ) float32 {
    v := float32(50.0)

    if acc > 0 {
        v += acc
    }

    return v
}

func AggregateValue( v float32, o int, acc float32, obs int ) (float32, int) {
    acc += v
    obs += o

    return acc, obs
}
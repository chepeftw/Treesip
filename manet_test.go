package treesip

import (
    "testing"
)

func TestFunctionValue(t *testing.T) {
	acc := float32(100.0)
	acc  = functionValue(acc)

	if acc != 150 {
		t.Fail()
	}
}

func TestAggregateValue(t *testing.T) {
	acc := float32(100.0)
	obs := 1
	acc, obs  = aggregateValue( float32(200.0), 1, acc, obs)

	if acc != 300 || obs != 2 {
		t.Fail()
	}
}
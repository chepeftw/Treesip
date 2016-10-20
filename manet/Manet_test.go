package manet

import (
    "testing"
)

func TestFunctionValue(t *testing.T) {
	acc := float64(100.0)
	acc  = FunctionValue(acc)

	if acc != 150 {
		t.Fail()
	}
}

func TestAggregateValue(t *testing.T) {
	acc := float64(100.0)
	obs := 1
	acc, obs  = AggregateValue( float64(200.0), 1, acc, obs)

	if acc != 300 || obs != 2 {
		t.Fail()
	}
}
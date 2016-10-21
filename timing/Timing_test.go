package timing

import (
	"time"
    "testing"
)

var timer *time.Timer

func TestTimeout(t *testing.T) {
	timer = Timeout(1000, timer)

	if timer == nil {
		t.Fail()
	}

	timer = Timeout(500, timer)

	if timer == nil {
		t.Fail()
	}

	<- timer.C

	if timer == nil {
		t.Fail()
	}
}
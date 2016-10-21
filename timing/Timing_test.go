package timing

import (
	"time"
    "testing"
    "math/rand"
)

var timer *time.Timer

func TestTimeout(t *testing.T) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	timer = Timeout(1000, timer, r1)

	if timer == nil {
		t.Fail()
	}

	timer = Timeout(500, timer, r1)

	if timer == nil {
		t.Fail()
	}

	<- timer.C

	if timer == nil {
		t.Fail()
	}
}
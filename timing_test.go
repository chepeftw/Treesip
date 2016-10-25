package treesip

import (
	"time"
    "testing"
    "math/rand"
)

var timerTest *time.Timer

func TestTimeout(t *testing.T) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	timerTest = startTimeout(1000, timerTest, r1)

	if timerTest == nil {
		t.Fail()
	}

	timerTest = startTimeout(200, timerTest, r1)

	if timerTest == nil {
		t.Fail()
	}

	<- timerTest.C

	if timerTest == nil {
		t.Fail()
	}
}
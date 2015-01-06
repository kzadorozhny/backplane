package swim

import (
	"testing"
)

func TestLamportClock(t *testing.T) {
	var l LamportClock

	if l.GetEpoch() != 0 {
		t.Fatalf("bad time value")
	}

	if l.IncrementEpoch() != 1 {
		t.Fatalf("bad time value")
	}

	if l.GetEpoch() != 1 {
		t.Fatalf("bad time value")
	}

	ret := l.OnReceivedEpoch(41)

	if ret != 42 || l.GetEpoch() != 42 {
		t.Fatalf("bad time value")
	}

	ret = l.OnReceivedEpoch(41)

	if ret != 42 || l.GetEpoch() != 42 {
		t.Fatalf("bad time value")
	}

	ret = l.OnReceivedEpoch(30)

	if ret != 42 || l.GetEpoch() != 42 {
		t.Fatalf("bad time value")
	}
}

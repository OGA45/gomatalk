package discord

import (
	"testing"
	"time"
)

// run feeds one probe per minute for the given duration and returns the first
// exit reason (or "").
func run(m *gatewayMonitor, start time.Time, d time.Duration, probe func(i int) (bool, time.Time, bool)) string {
	for i := 1; time.Duration(i)*time.Minute <= d; i++ {
		locked, ack, ready := probe(i)
		if r := m.observe(start.Add(time.Duration(i)*time.Minute), locked, ack, ready); r != "" {
			return r
		}
	}
	return ""
}

func TestWatchdogHealthy(t *testing.T) {
	start := time.Now()
	m := newGatewayMonitor(start)
	// A fresh ACK every probe, lock sometimes briefly busy (Close/Open).
	r := run(m, start, 3*time.Hour, func(i int) (bool, time.Time, bool) {
		return i%7 != 0, start.Add(time.Duration(i) * time.Minute), true
	})
	if r != "" {
		t.Fatalf("healthy gateway flagged: %s", r)
	}
}

func TestWatchdogLockHeldForever(t *testing.T) {
	start := time.Now()
	m := newGatewayMonitor(start)
	m.observe(start, true, start, true)
	r := run(m, start, 30*time.Minute, func(int) (bool, time.Time, bool) { return false, time.Time{}, false })
	if r == "" {
		t.Fatal("deadlocked session lock not detected")
	}
}

func TestWatchdogConnectedWithoutAck(t *testing.T) {
	start := time.Now()
	m := newGatewayMonitor(start)
	ack := start
	r := run(m, start, 30*time.Minute, func(int) (bool, time.Time, bool) { return true, ack, true })
	if r == "" {
		t.Fatal("wedged heartbeat (DataReady, stale ACK) not detected")
	}
}

func TestWatchdogToleratesReconnectingOutage(t *testing.T) {
	start := time.Now()
	m := newGatewayMonitor(start)
	ack := start
	// Reconnecting (DataReady=false, lock free) for 45 min, then recovered.
	r := run(m, start, 45*time.Minute, func(int) (bool, time.Time, bool) { return true, ack, false })
	if r != "" {
		t.Fatalf("45 min outage killed the process: %s", r)
	}
	r = run(m, start.Add(45*time.Minute), 30*time.Minute, func(i int) (bool, time.Time, bool) {
		return true, start.Add(time.Duration(45+i) * time.Minute), true
	})
	if r != "" {
		t.Fatalf("recovered gateway flagged: %s", r)
	}
}

func TestWatchdogReconnectCeiling(t *testing.T) {
	start := time.Now()
	m := newGatewayMonitor(start)
	ack := start
	r := run(m, start, 2*time.Hour, func(int) (bool, time.Time, bool) { return true, ack, false })
	if r == "" {
		t.Fatal("reconnect that never succeeds not detected")
	}
}

func TestWatchdogIgnoresWallClockSteps(t *testing.T) {
	start := time.Now()
	m := newGatewayMonitor(start)
	// ACK values jump backwards/forwards by hours (NTP step) but keep changing.
	r := run(m, start, time.Hour, func(i int) (bool, time.Time, bool) {
		return true, start.Add(time.Duration((i%2)*2-1) * time.Duration(i) * time.Hour), true
	})
	if r != "" {
		t.Fatalf("clock steps caused an exit: %s", r)
	}
}

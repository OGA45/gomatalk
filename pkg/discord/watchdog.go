package discord

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"
)

// Limits for gatewayMonitor. The library itself reconnects after ~3.5 min
// without a heartbeat ACK, so these only fire when it failed to recover.
const (
	watchdogLockLimit      = 10 * time.Minute // session lock never obtainable: deadlock
	watchdogConnectedLimit = 10 * time.Minute // "connected" but no ACK: wedged heartbeat
	watchdogReconnectLimit = 60 * time.Minute // reconnecting without success (outage or stuck loop)
)

// gatewayMonitor decides from periodic probes whether the gateway is dead
// beyond the library's own recovery. Durations are measured with the probe
// times passed in (monotonic), never with LastHeartbeatAck's wall clock: a
// change of the ACK value is all that counts as progress.
type gatewayMonitor struct {
	lastAck      time.Time
	lastProgress time.Time
	lastLocked   time.Time
}

func newGatewayMonitor(now time.Time) *gatewayMonitor {
	return &gatewayMonitor{lastProgress: now, lastLocked: now}
}

// observe records one probe. locked reports whether the session read lock was
// obtained; ack (LastHeartbeatAck) and ready (DataReady) are only meaningful
// when it was. It returns a non-empty reason when the process should exit.
func (m *gatewayMonitor) observe(now time.Time, locked bool, ack time.Time, ready bool) string {
	if !locked {
		if d := now.Sub(m.lastLocked); d > watchdogLockLimit {
			return fmt.Sprintf("session lock unavailable for %v", d.Round(time.Second))
		}
		return ""
	}
	m.lastLocked = now
	if !ack.Equal(m.lastAck) {
		m.lastAck, m.lastProgress = ack, now
	}
	d := now.Sub(m.lastProgress)
	switch {
	case ready && d > watchdogConnectedLimit:
		return fmt.Sprintf("no heartbeat ACK for %v while connected", d.Round(time.Second))
	case d > watchdogReconnectLimit:
		// DataReady is false while reconnect() retries; give an ordinary
		// outage time to heal in-process (resuming keeps voice connections).
		return fmt.Sprintf("gateway not reconnected for %v", d.Round(time.Second))
	}
	return ""
}

// gatewayWatchdog exits the process when the gateway is dead or wedged in a
// way the library does not recover from (e.g. a deadlocked reconnect).
// restart: always then brings the bot back; the goroutine dump written first
// shows what was stuck.
func gatewayWatchdog() {
	go func() {
		m := newGatewayMonitor(time.Now())
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			var ack time.Time
			var ready bool
			// Never wait for the session lock: a lock held forever is exactly
			// what this has to detect.
			locked := Dg.TryRLock()
			if locked {
				ack, ready = Dg.LastHeartbeatAck, Dg.DataReady
				Dg.RUnlock()
			}
			if reason := m.observe(time.Now(), locked, ack, ready); reason != "" {
				DumpAndExit("gateway watchdog: " + reason)
			}
		}
	}()
}

// DumpAndExit logs reason, writes every goroutine's stack to stderr (so it
// lands in the container log and Loki) and exits with status 1.
func DumpAndExit(reason string) {
	log.Printf("FATAL: %s; dumping goroutines and exiting", reason)
	pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
	os.Exit(1)
}

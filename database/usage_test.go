package database

import (
	"sort"
	"testing"
	"time"

	"github.com/atomic77/nethadone/config"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func resetUsage(t *testing.T) {
	t.Helper()
	usage.last, usage.totals, usage.snaps = nil, nil, nil
	t.Cleanup(func() { usage.last, usage.totals, usage.snaps = nil, nil, nil })
}

// feed records one collection every 30s for `ticks` ticks, starting at `from`.
// bytes(i) returns the cumulative counters at tick i.
func feed(from time.Time, ticks int, bytes func(i int) []PairBytes) time.Time {
	var now time.Time
	for i := 0; i < ticks; i++ {
		now = from.Add(time.Duration(i) * 30 * time.Second)
		RecordUsage(now, bytes(i))
	}
	return now.Add(30 * time.Second)
}

func srcs(sgs []SrcGlob) []string {
	out := []string{}
	for _, sg := range sgs {
		out = append(out, sg.Src+"/"+sg.Glob)
	}
	sort.Strings(out)
	return out
}

func wantSrcs(t *testing.T, got []SrcGlob, want ...string) {
	t.Helper()
	g := srcs(got)
	if len(g) != len(want) {
		t.Errorf("got %v, want %v", g, want)
		return
	}
	for i := range want {
		if g[i] != want[i] {
			t.Errorf("got %v, want %v", g, want)
			return
		}
	}
}

func TestUsageRates(t *testing.T) {
	resetUsage(t)
	// A streams 3000 B/s to two yt IPs (summed per glob); B is idle on yt; C only on an unknown domain.
	feed(t0, 21, func(i int) []PairBytes {
		n := uint64(i) * 30 * 1500
		return []PairBytes{
			{Src: "A", Dst: "yt1", Glob: "yt", Bytes: n},
			{Src: "A", Dst: "yt2", Glob: "yt", Bytes: n},
			{Src: "B", Dst: "yt1", Glob: "yt", Bytes: 500},
			{Src: "C", Dst: "other", Glob: "", Bytes: n},
		}
	})

	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 2000, true), "A/yt")
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 4000, true))
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 2000, false), "B/yt")
	wantSrcs(t, GetSrcGlobUsage(2*time.Minute, 5*time.Minute, 2000, true), "A/yt")
}

func TestUsageOffsetWindow(t *testing.T) {
	resetUsage(t)
	// A was active for the first 5 minutes only, then stopped.
	feed(t0, 21, func(i int) []PairBytes {
		return []PairBytes{{Src: "A", Dst: "yt1", Glob: "yt", Bytes: uint64(min(i, 10)) * 30 * 1000}}
	})

	wantSrcs(t, GetSrcGlobUsage(2*time.Minute, 0*time.Minute, 500, true))
	wantSrcs(t, GetSrcGlobUsage(2*time.Minute, 5*time.Minute, 500, true), "A/yt")
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 500, false), "A/yt")
}

func TestUsageNeedsHistory(t *testing.T) {
	resetUsage(t)
	feed(t0, 6, func(i int) []PairBytes { // 2.5 minutes of data
		return []PairBytes{{Src: "A", Dst: "yt1", Glob: "yt", Bytes: uint64(i) * 1e6}}
	})

	wantSrcs(t, GetSrcGlobUsage(2*time.Minute, 0*time.Minute, 1, true), "A/yt")
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 1, true))
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 1, false))
	wantSrcs(t, GetSrcGlobUsage(2*time.Minute, 5*time.Minute, 1, true))
}

func TestUsageCounterResetAndLateGlob(t *testing.T) {
	resetUsage(t)
	feed(t0, 11, func(i int) []PairBytes {
		switch {
		case i < 5: // domain not known yet: big counter, no glob
			return []PairBytes{{Src: "A", Dst: "yt1", Bytes: 1e9 + uint64(i)}}
		case i < 8: // glob learned; counter keeps growing slowly
			return []PairBytes{{Src: "A", Dst: "yt1", Glob: "yt", Bytes: 1e9 + uint64(i)}}
		default: // map was cleared: counter restarts small
			return []PairBytes{{Src: "A", Dst: "yt1", Glob: "yt", Bytes: uint64(i - 7)}}
		}
	})

	// Neither the pre-glob history nor the reset may look like a burst
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 1, true))
	wantSrcs(t, GetSrcGlobUsage(5*time.Minute, 0*time.Minute, 1, false), "A/yt")
	if got := usage.totals[SrcGlob{"A", "yt"}]; got != 6 {
		t.Errorf("total = %d, want 6 (3 before reset + 3 after)", got)
	}
}

func TestUsageRetention(t *testing.T) {
	resetUsage(t)
	feed(t0, 60, func(i int) []PairBytes { return nil }) // 30 minutes
	if span := usage.snaps[len(usage.snaps)-1].t.Sub(usage.snaps[0].t); span > usageRetention() {
		t.Errorf("kept %v of snapshots, want <= %v", span, usageRetention())
	}
}

// With throttle_per_device off, two devices each below the threshold add up to one active "all" source.
func TestUsageSharedAcrossDevices(t *testing.T) {
	resetUsage(t)
	config.Cfg.ThrottlePerDevice = false
	t.Cleanup(func() { config.Cfg.ThrottlePerDevice = true })
	feed(t0, 21, func(i int) []PairBytes {
		n := uint64(i) * 30 * 600
		return []PairBytes{
			{Src: "phone", Dst: "yt1", Glob: "yt", Bytes: n},
			{Src: "laptop", Dst: "yt2", Glob: "yt", Bytes: n},
		}
	})
	wantSrcs(t, GetSrcGlobUsage(2*time.Minute, 0, 1000, true), AllDevices+"/yt")
}

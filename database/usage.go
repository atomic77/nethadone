package database

import (
	"sync"
	"time"

	"github.com/atomic77/nethadone/config"
)

// In-process replacement for the Prometheus queries the policy loop used to make.
// pollMetrics feeds per IP pair byte counters in; we keep a few minutes of
// cumulative per (src_ip, glob) totals and answer windowed rate questions.

// usageRetention must cover the longest window+offset the policy asks for (2 units ending 5 units ago).
func usageRetention() time.Duration { return 10 * config.Cfg.PolicyTimeUnit }

type SrcGlob struct {
	Src  string
	Glob string
}

type PairBytes struct {
	Src, Dst, Glob string
	Bytes          uint64 // cumulative counter from the trafficmon map
}

type usageSnapshot struct {
	t      time.Time
	totals map[SrcGlob]uint64
}

var usage struct {
	sync.Mutex
	last   map[[2]string]uint64 // previous counter per IP pair
	totals map[SrcGlob]uint64   // running sum of deltas, never decreases
	snaps  []usageSnapshot      // oldest first
}

// RecordUsage adds one collection of the trafficmon counters, taken at now.
func RecordUsage(now time.Time, pairs []PairBytes) {
	usage.Lock()
	defer usage.Unlock()
	if usage.totals == nil {
		usage.totals = map[SrcGlob]uint64{}
	}
	last := make(map[[2]string]uint64, len(pairs))
	for _, p := range pairs {
		k := [2]string{p.Src, p.Dst}
		prev, seen := usage.last[k]
		last[k] = p.Bytes
		// Deltas per pair, so a pair whose glob becomes known later doesn't
		// count its whole history as a burst. A smaller value means the counter reset.
		delta := p.Bytes
		if seen && p.Bytes >= prev {
			delta = p.Bytes - prev
		}
		if p.Glob != "" {
			usage.totals[SrcGlob{p.Src, p.Glob}] += delta
		}
	}
	usage.last = last

	snap := usageSnapshot{t: now, totals: make(map[SrcGlob]uint64, len(usage.totals))}
	for k, v := range usage.totals {
		snap.totals[k] = v
	}
	usage.snaps = append(usage.snaps, snap)
	i := 0
	for i < len(usage.snaps) && usage.snaps[i].t.Before(now.Add(-usageRetention())) {
		i++
	}
	usage.snaps = usage.snaps[i:]
}

// snapshotAt returns the newest snapshot taken at or before t.
func snapshotAt(t time.Time) *usageSnapshot {
	for i := len(usage.snaps) - 1; i >= 0; i-- {
		if !usage.snaps[i].t.After(t) {
			return &usage.snaps[i]
		}
	}
	return nil
}

// GetSrcGlobUsage returns the (src_ip, glob) pairs whose byte rate per second,
// averaged over `window` ending `ago` before the latest collection, is above (or below) k.
// It returns nil until enough history has been collected, and a non-nil
// (possibly empty) slice afterwards, so callers can tell the two apart.
// It is a variable so tests can substitute canned results.
var GetSrcGlobUsage = getSrcGlobUsage

func getSrcGlobUsage(window, ago time.Duration, k int, above bool) []SrcGlob {
	usage.Lock()
	defer usage.Unlock()
	if len(usage.snaps) == 0 {
		return nil
	}
	end := snapshotAt(usage.snaps[len(usage.snaps)-1].t.Add(-ago))
	if end == nil {
		return nil
	}
	start := snapshotAt(end.t.Add(-window))
	if start == nil || start == end {
		return nil
	}
	secs := end.t.Sub(start.t).Seconds()
	res := []SrcGlob{}
	for sg, total := range end.totals {
		r := float64(total-start.totals[sg]) / secs
		if (above && r > float64(k)) || (!above && r < float64(k)) {
			res = append(res, sg)
		}
	}
	return res
}

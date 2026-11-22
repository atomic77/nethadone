package config

import (
	"runtime/debug"
	"strings"
	"time"
)

// Version is the release name, set by the release build: make all VERSION=nethadone-2026-11-22
var Version = "dev"

// BuildInfo describes this binary, e.g. "nethadone-2026-11-22 · c5105d1 · committed 2026-11-22 21:04 UTC".
// The commit comes from Go's VCS stamping, which needs a package build (./cmd) inside a git checkout.
func BuildInfo() string {
	parts := []string{Version}
	if bi, ok := debug.ReadBuildInfo(); ok {
		s := map[string]string{}
		for _, kv := range bi.Settings {
			s[kv.Key] = kv.Value
		}
		if rev := s["vcs.revision"]; rev != "" {
			if len(rev) > 7 {
				rev = rev[:7]
			}
			if s["vcs.modified"] == "true" {
				rev += "+dirty"
			}
			parts = append(parts, rev)
		}
		if t, err := time.Parse(time.RFC3339, s["vcs.time"]); err == nil {
			parts = append(parts, "committed "+t.UTC().Format("2006-01-02 15:04 UTC"))
		}
	}
	return strings.Join(parts, " · ")
}

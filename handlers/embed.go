package handlers

import "embed"

// Currently we are handling the management of the throttler
// ebpf outside of bpf2go due to challenges figuring out how to allow
// live recompiling

//go:embed throttle.bpf.c.tpl
var EmbedThrottlerCode embed.FS

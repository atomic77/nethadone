# AGENTS.md

Guidance for AI coding agents working on Nethadone. Read `README.md` for the product pitch and `policy/README.md` for the throttling maths.

## What this is

A single Go binary (Go 1.22, module `github.com/atomic77/nethadone`) that runs **as root on a Linux router** (Orange Pi R1/R1 Plus/Zero, armv7 + arm64) and slows down traffic to "addictive" sites using eBPF and tc qdiscs. It also serves a small web UI on `:3000`.

Data flow:

1. `ebpf/dnspkt.bpf.c` (tc filter on the WAN, ingress and egress) sniffs DNS answers and sends them to userspace through a perf array. `handlers/bpf.go:pollDnsResponses` stores the A records in the sqlite `dns` table.
2. `ebpf/trafficmon.bpf.c` (tc filter on LAN egress) counts bytes per (src,dst) IP pair in the hash map `src_dest_bytes`.
3. Every `map_collection_interval`, `handlers/prom.go:pollMetrics` walks that map, looks up the domain and glob group for each pair, and exports `ip_pair_vic_bytes_total{src_ip,dest_ip,prob_domain,glob}` on `/metrics`.
4. An **external Prometheus** scrapes `/metrics`. Every `policy_check_interval`, `policy/simpleloadavg.go` queries Prometheus (`database/prom.go`) and moves each (src_ip, glob) policy up or down one bandwidth class. Policies live in the sqlite `policy` table.
5. When any policy changes, `handlers.ApplyPolicies` renders `handlers/throttle.bpf.c.tpl` with one `if` per IP pair, **compiles it with `clang` at runtime**, and swaps it in as the tc filter on the LAN HTB root (`1:`). The filter sets `skb->tc_classid`.

## Layout

| Path | Purpose |
|---|---|
| `cmd/nethadone.go` | Entry point: flags, config, routes. |
| `config/` | `Config` struct, defaults in `baseConfig`, global `config.Cfg`. `config/nethadone.yml` is the documented example. |
| `database/` | sqlite through sqlx + `modernc.org/sqlite` (pure Go, no cgo). Two DBs: `cfg_db` (glob_group, device, policy) and `dns_db` (dns). Also the Prometheus query client. Schema is created in `Connect()`. |
| `handlers/` | Mixes HTTP handlers (`handlers.go`), eBPF loading and attaching (`bpf.go`), qdisc setup (`qdisc.go`) and metrics (`prom.go`). |
| `handlers/*_bpfe[lb].{go,o}` | **Generated** by bpf2go and committed. Do not hand-edit. |
| `handlers/throttle.bpf.c.tpl` | Throttler template, embedded via `handlers/embed.go`. This is the real source; `ebpf/throttle.bpf.c` is a gitignored render. |
| `ebpf/` | BPF C sources, `vmlinux.h`, plus an ad-hoc Makefile and the `dnsprobe.py` (bcc) prototype. |
| `policy/` | The SimpleLoadAverage policy loop. |
| `models/` | Plain structs with `db:` tags. |
| `views/` | Fiber html templates (`.tpl`), embedded through `views/embed.go`. |
| `scripts/` | Armbian image customisation (`build.sh` mounts an image and chroots with qemu-static). |
| `doc/` | Install, dev VM, qdisc and notes docs. `doc/todo.md` lists known open problems (DoH, CNAME chains). |

## Build and run

```bash
go build ./...                 # compiles on any Linux host, no cgo
make build-binary              # ./nethadone in the repo root (not gitignored, don't commit it)
make all                       # cross-compile arm, arm64, amd64 into build/
make run-root NTH_LAN=lan0 NTH_WAN=eth0   # build + sudo run with config/nethadone.yml
go generate ./handlers         # regenerate bpf2go output after editing dnspkt/trafficmon .bpf.c (needs clang)
```

Flags: `-wan-interface`, `-lan-interface` and `-config-file` (default `nethadone.yml` in CWD). The flags override the YAML. A missing config file silently falls back to the defaults.

Runtime requirements on the target: root, a kernel with BTF, `clang` and the libbpf/kernel headers (the throttler is compiled on the device; include paths are hardcoded for aarch64 and armv7 in `handlers/bpf.go:rebuildBpf`), and a Prometheus that scrapes `:3000/metrics`. The Docker image is only a convenient way to build a dev VM (see `doc/development.md`, which uses d2vm). Containers cannot run nethadone itself.

The CI workflow `.github/workflows/build-nethadone-binary.yml` (manual dispatch) runs `make all` and publishes a dated release.

## Tests

See `doc/development.md` for the four levels. In short:

- `make test`: `go vet ./...` + `go test ./...`. No root needed. Run this for every change.
- `make test-bpf`: `go test -exec sudo ./handlers -run BPF`. Loads the BPF programs and runs them on synthetic packets with `BPF_PROG_TEST_RUN`. Attaches nothing. Needs root, clang and libbpf-dev.
- `make test-vm KERNEL=v6.6.40`: the same, inside a virtme-ng VM on a chosen upstream kernel.
- `scripts/netns-lab.sh`: the full binary inside a namespaced router topology. This is the only safe way to run the binary on a workstation.

Test seams are package variables: `database.GetSrcGlobUsage` (Prometheus) and `policy.applyToKernel` (compile + attach). Tests point `config.Cfg.DnsDb/CfgDb` at `t.TempDir()`. `handlers/qdisc_test.go` lists qdiscs (read-only) and is skipped unless running as root.

## Conventions and invariants

- **Global state everywhere**: `config.Cfg`, `database.dnsDb/cfgDb/promDb` and `handlers.BpfCtx`. Init order in `main` matters: `ParseConfig` → `database.Connect` → `handlers.Initialize` (qdiscs + BPF) → `InitMetrics` → `InitPolicy`.
- **Errors**: setup and kernel failures call `log.Fatal`. Request-time and loop failures log and continue. There is no logging framework, only `log.Println`.
- **Class IDs**: a policy `class` is a multiple of 10 (10, 20, …). The template emits it as `0x{{ .ClassId }}`, so class 10 becomes tc minor `0x10`. `qdisc.go:createLeafQdiscs` creates leaf classes `1:(idx+1)*16`, which are 0x10, 0x20, and so on. If you change either side, keep them in sync.
- **IP format for BPF**: `IpPolicy.SrcIpAddr` and `DestIpAddr` use commas (`192,168,0,1`) because they feed the `IP_ADDRESS(o1,o2,o3,o4)` macro. The conversion happens in `database.GetIpPolicies`.
- The throttler matches `l3->saddr == dest` and `l3->daddr == src` because it runs on **LAN egress**, so it sees reply traffic coming *back* from the remote site.
- Glob groups use sqlite `GLOB` syntax (e.g. `*.reddit.com`), matched against the `dns` table. Only IPs seen in the last ~5 days count.
- The throttler is deliberately **not** built with bpf2go: bpf2go embeds bytecode, which would prevent live recompilation. Keep it template-based unless you redesign the reload path.
- Config: add new fields to `Config` with a `yaml:"snake_case"` tag, a default in `baseConfig`, and a commented entry in `config/nethadone.yml`.
- Target hardware is weak (32-bit armv7, ~512MB RAM). `pollMetrics` is already slow with thousands of IP pairs, so avoid adding per-packet or per-pair work in hot loops.

## Known issues found during review (not yet fixed)

- The throttler emits one `if` per (client, IP) pair. clang memory grows faster than the rule count (~360 MB at 500 rules, ~1.1 GB at 1000). Compilation fails at ~2000 rules and the 7.0 verifier rejects ~1000. Any failure is a `log.Fatal` in `rebuildBpf`. A BPF hash map keyed by IP pair would remove the limit.
- Class IDs are hex-encoded decimals, so the class ↔ leaf invariant breaks if `num_qdisc_classes` ≥ 9 (class 100 → `0x100`, but leaf 9 is `0xa0`).
- `database.Connect`: `sqlx.Open` errors are checked only after `.DB.SetMaxOpenConns` has already been called on the result.
- `handlers.Initialize` applies a hardcoded placeholder policy (`192,168,0,174`, class 10) at startup.
- `/favicon.ico` serves `static/laptop.svg` from a relative path, which is not embedded, so it 404s unless CWD is the repo or `/app`.
- `handlers/bpf.go`: `attachTrafficMonitor` and `reattachThrottler` log "DNS sniffer" in their error messages (copy-paste).
- The uncommitted `go.mod` diff changes a comment from `// indirect` to `// inditect`. It looks like a typo.
- Large commented-out blocks remain (old `CompileBpf`, tcnl close). Prefer deleting dead code over adding more.

## Working in this repo

- Never run the binary, `make run-root`, `make clean-tc` or the `ebpf/Makefile` `rebuild*` and `detach-all` targets on a dev workstation. They delete or replace qdiscs and tc filters on real interfaces (`eth0`/`eth1`/`lan0`) and can cut your network. Use `scripts/netns-lab.sh`, which runs the binary inside a network namespace.
- `Makefile` targets `up`/`up-silent` reference `./app` and a `fiber dev` tool. They are stale template leftovers.

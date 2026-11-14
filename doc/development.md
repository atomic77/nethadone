## Development and Contribution

Most of Nethadone can be developed and tested on an ordinary Linux workstation.
There are four levels, from cheapest to closest to the real router.

### Prerequisites

Go (see `go.mod`). For the BPF levels: root, a kernel with BTF, and
`clang` + `libbpf-dev` (the throttler is compiled at runtime, as on the router).
Running `go generate ./handlers` (rebuilding `dnspkt`/`trafficmon`) on x86 also
needs `gcc-multilib`, because those sources include libc headers.

### 1. Unit tests: `make test`

No root, no kernel features. Covers the policy loop (with Prometheus and the
kernel replaced by stubs), sqlite queries on temp databases, the Prometheus
client against a fake API, DNS answer parsing, the throttler template, and the
policy class ↔ qdisc leaf handle invariant.

The seams used by the tests are package variables: `database.GetSrcGlobUsage`
(Prometheus) and `policy.applyToKernel` (BPF compile + attach).

### 2. BPF programs on synthetic packets: `make test-bpf`

`handlers/bpf_prog_test.go` loads `dnspkt`, `trafficmon` and a freshly compiled
throttler and runs them with `BPF_PROG_TEST_RUN`. **Nothing is attached to any
interface**, so this is safe on a workstation. It needs root (`go test -exec sudo`).

`BPF_PROG_TEST_RUN` does not return `skb->tc_classid`, so throttler tests assert
on the `throttle_stats` counters; the class value itself is checked by the
template test in level 1.

`TestBPFThrottlerScale` compiles and loads 500 rules. Set `NTH_THROTTLE_RULES`
to probe further. Known limits of the one-`if`-per-IP-pair design:

| Rules | clang peak memory | Result |
|---|---|---|
| 250 | ~160 MB | ok |
| 500 | ~360 MB | ok |
| 1000 | ~1.1 GB | ok on kernel 6.6; the 7.0 verifier rejects it ("cannot be patched due to 16-bit range") |
| 2000 | | clang fails: "Branch target out of insn range" |

On a 512 MB board, clang will likely be OOM-killed well before 1000 rules.

### 3. Full router in network namespaces: `scripts/netns-lab.sh`

Builds a client / router / "internet" topology from veth pairs in three
network namespaces, with dnsmasq and iperf3 on the internet side and Prometheus
(Docker) on the host. Nethadone runs inside the router namespace, where it can
only see `lan0`, `wan0` and `mgmt0`, so the host's interfaces are never touched.

```bash
sudo scripts/netns-lab.sh up        # or: make lab-up
sudo scripts/netns-lab.sh run       # foreground; UI at http://10.30.0.2:3000
sudo scripts/netns-lab.sh traffic   # another shell: DNS lookups + iperf3 downloads
sudo scripts/netns-lab.sh status    # policies, per-class qdisc counters, metrics
sudo scripts/netns-lab.sh down
```

The lab config uses short intervals. The first policy appears after about 5
minutes of traffic (the policy queries look back 5 minutes), then the client
steps down one bandwidth class per backoff interval (1 minute).

Unthrottled traffic shows up in the HTB `direct_packets_stat`, not class `1:10`:
the throttler filter's default classid is `1:0`, which HTB treats as direct.

### 4. Specific kernels: `make test-vm`

[virtme-ng](https://github.com/arighi/virtme-ng) boots a kernel against your host
filesystem in a few seconds. Pass an upstream version, downloaded from the
Ubuntu mainline archive (not every point release exists there):

```bash
make test-vm KERNEL=v6.6.40
```

`vng -r` with no version boots the host kernel, but Ubuntu makes
`/boot/vmlinuz-*` root-only. `vng --arch arm64` can boot arm kernels under TCG
(slow); that path has not been tried yet.

CI (`.github/workflows/test.yml`) runs levels 1 and 2 on the runner kernel and
level 2 again under virtme-ng.

### Full VM image (d2vm)

For a complete VM, build the Docker image and convert it with
[`d2vm`](https://github.com/linka-cloud/d2vm):

```bash
docker buildx build -t atomic77/nethadone:latest .
sudo d2vm convert atomic77/nethadone -o nethadone.qcow2 -p 1234
d2vm run qemu --networking bridge,virbr0 --mem 4096 --cpus 4 ../nethadone.qcow2
```

For VMWare, convert with `qemu-img convert nethadone.qcow2 -O vmdk nethadone.vmdk`.
VirtualBox has not worked well.

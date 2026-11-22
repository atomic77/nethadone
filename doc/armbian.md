# Nethadone on Armbian

This page projects, for every board in the [Armbian build system](https://github.com/armbian/build),
whether a stock Armbian kernel has what nethadone needs, or whether you need a custom-built kernel.
It complements the [boards tested so far](../README.md#getting-started): only those have actually
been run. Everything below is read from Armbian's kernel configuration, not tried on hardware.

**Scanned:** [`armbian/build`](https://github.com/armbian/build) at commit
[`5fd95448bada0f3f2e24b8c9191d98bd2567abf6`](https://github.com/armbian/build/commit/5fd95448bada0f3f2e24b8c9191d98bd2567abf6)
(committed 2026-11-21), scanned on 2026-11-21. Armbian changes kernel configs often, so recheck
before relying on an old scan.

## Summary

- **Boards with standard support** (`config/boards/*.conf`, 157 boards): out of the box: **107** · some branches: **2** · custom kernel: **42** · unsupported arch: **6**
- **Community-maintained boards** (`*.csc`, 246 boards): out of the box: **168** · custom kernel: **72** · unsupported arch: **6**

Kernel families whose stock configs have everything nethadone needs on every branch:
`bcm2711`, `cix-p1`, `meson`, `meson-s7`, `meson64`, `mt7623`, `mvebu`, `mvebu64`, `qcs6490`, `rk35xx`,
`rockchip64`, `sc8280xp`, `sun60iw2`, `sunxi64`, `uefi-arm64-dt`. Three more do on some branches only: the
UEFI families `arm64` and `x86` on every branch except their stripped-down `cloud` one, and `rockchip` on
`current` and `edge` but not `vendor`.

How the boards in the main README project:

| Board | Kernel family | Projection |
|---|---|---|
| Orange Pi R1, Orange Pi Zero (`.csc`) | `sunxi` (armv7) | custom kernel on every branch: stock configs lack `CONFIG_NET_CLS_BPF` |
| Orange Pi R1 Plus, R1 Plus LTS (`.csc`) | `rockchip64` | out of the box on `current` and `edge` |
| Orange Pi Zero2 (`.csc`), Orange Pi 3 LTS (`.conf`) | `sunxi64` | kernel is fine on every branch, but their UWE5622 wifi cannot run a reliable hotspot (see the README) |

Armbian's `rpi4b` board file covers all 64-bit Raspberry Pi models from the BCM2837 (the 3B+) onwards, and
projects to work out of the box on every branch. Armbian may therefore be an alternative to Ubuntu Server
on the Raspberry Pi 3B+. This is untested.

## What is checked

These are the kernel options `nethadone check` requires (`handlers/check.go`), plus BTF:

| Requirement | Why |
|---|---|
| `NET_SCH_HTB`, `NET_SCH_PRIO`, `NET_SCH_NETEM` | the bandwidth classes on the LAN interface |
| `NET_SCH_INGRESS`, `NET_CLS_ACT` | the clsact hooks the BPF programs attach to |
| `NET_CLS_BPF` | attaching any of the BPF programs |
| `BPF_EVENTS` | the DNS sniffer's helpers |
| BTF (`/sys/kernel/btf/vmlinux`) | currently required by `nethadone check` |

Built-in (`y`) or module (`m`) both count. An option passes when it is enabled in either of these places:

1. **The family's defconfig**, `config/kernel/linux-<family>-<branch>.config`. Armbian copies it and runs
   `make olddefconfig`. These files are minimal, so an option they leave out gets its Kconfig default,
   which is "off" for every tc option above.
2. **Armbian's build hooks** in `lib/functions/compilation/armbian-kernel.sh`, which run for every family.
   `armbian_kernel_config__600_enable_ebpf_and_btf_info` turns on `DEBUG_INFO_BTF`, `BPF_EVENTS` and
   `KPROBE_EVENTS` unless the family or board sets `KERNEL_BTF=no`. No family or extension changes the
   tc options above.

The board-to-family-to-config mapping comes from running `./compile.sh config-dump-json` for every
board and branch, which sets `LINUXFAMILY`, `LINUXCONFIG`, `ARCH` and `KERNEL_BTF` as a real build would.

**Projection** column:

- **out of the box**: every branch has all of the above.
- **some branches**: pick a branch marked ✅.
- **custom kernel**: no branch qualifies. Build a kernel that adds the missing options. The
  `armbian-btf-kernel-build` workflow (`.github/workflows/build-btf-kernel.yml`) does this for `sunxi`, but
  it only appends the BPF options: families also missing HTB, prio or netem need those added too.
- **unsupported arch**: riscv64 or loong64. Nethadone releases only arm (armv7), arm64 and amd64.

"BPF_EVENTS (probably)" means `KERNEL_BTF=no` stops the hook from enabling it, and the defconfig does not
set it. It may still end up on through Kconfig defaults, so run `nethadone check` to be sure.

## Limits of this projection

- **It does not check the hardware.** Nethadone also needs two network interfaces, or wifi that can run a
  stable access point (see the README), and enough RAM to run `clang` on the device.
- **Downloadable images can differ from what the build system produces today.** Images built before the
  BTF hook existed, or on a build host with too little RAM (Armbian then stops unless `KERNEL_BTF` is set),
  may lack BTF and `BPF_EVENTS`. To check an installed board, run `sudo nethadone check`, or
  `zgrep -E 'NET_SCH_(HTB|PRIO|NETEM|INGRESS)|NET_CLS_(BPF|ACT)|BPF_EVENTS|DEBUG_INFO_BTF=' /proc/config.gz`.
- **Only stock Armbian configs were read.** `userpatches/` configs, vendor images and other distributions are
  not covered.

## Kernel families that need changes

| Kernel family | Branches | Missing |
|---|---|---|
| `agilex5` | `current` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `arm64` | `cloud` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `filogic` | `current`, `edge` | NET_SCH_NETEM |
| `genio` | `edge` | NET_SCH_PRIO |
| `imx6` | `current`, `edge` | BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `imx8m` | `current`, `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `imx8ulp` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `imx93` | `current`, `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `k3` | `edge`, `vendor`, `vendor-edge`, `vendor-rt` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `k3-beagle` | `edge`, `vendor`, `vendor-rt` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `k3-tq` | `edge`, `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `ls1046a-ask` | `current` | NET_SCH_NETEM, NET_CLS_BPF |
| `meson-s4t7` | `legacy` | BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `meson-s4t7` | `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `nuvoton-ma35d1` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `odroidu2` | `current`, `edge` | BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `odroidxu4` | `current`, `edge` | BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `phytium-embedded` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `qrb2210` | `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `realtek-rtd129x` | `current`, `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `realtek-rtd1619b` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `rockchip` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `rockchip-rv1106` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `rockchip-rv1126` | `vendor` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `s5p6818` | `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `sm8250` | `current`, `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `sm8550` | `current`, `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `sm8550-sheng` | `edge` | NET_SCH_NETEM, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `sm8750` | `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| `sophgo-sg200x-arm64` | `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `sprd-ums512` | `edge` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| `sun55iw3-syterkit` | `legacy` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| `sunxi` | `current`, `edge`, `legacy` | NET_CLS_BPF |
| `x86` | `cloud` | NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |

## Boards with standard support (`.conf`)

| Board | Board file | Arch | Kernel family | Projection | Branches |
|---|---|---|---|---|---|
| A64 OLinuXino | `lime-a64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Banana Pi | `bananapi` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi CM4IO | `bananapicm4io` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Banana Pi M2S | `bananapim2s` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Banana Pi M5 | `bananapim5` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Banana Pi M5 Pro | `bananapim5pro` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Banana Pi M7 | `bananapim7` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Banana Pi Pro | `bananapipro` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi R2 | `bananapir2` | armhf | mt7623 | out of the box | `current` 6.18 ✅<br>`edge` 7.2 ✅ |
| BananaPi BPI-F3 | `bananapif3` | riscv64 | spacemit | unsupported arch | `current` 6.18<br>`edge` 7.3<br>`legacy` 6.6 |
| BananaPi BPI-M4-Zero | `bananapim4zero` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| BananaPi M4 Berry | `bananapim4berry` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| BeagleBadge | `beaglebadge` | arm64 | k3 | custom kernel | `vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| BeagleBone AI-64 | `beaglebone-ai64` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| BeaglePlay | `beagleplay` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| BeagleY-AI | `beagley-ai` | arm64 | k3-beagle | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.12 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.12 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| CB1 | `bigtreetech-cb1` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| CB2 | `bigtreetech-cb2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Cix-ACPI | `cix-acpi` | arm64 | cix-p1 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Clearfog Pro | `clearfogpro` | armhf | mvebu | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| CM4-IO | `radxa-cm4-io` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| CM5 IO | `radxa-cm5-io` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Cubox i2eX/i4 | `cubox-i` | armhf | imx6 | custom kernel | `current` 6.18 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`)<br>`edge` 7.1 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| DG SVR 865 Tiny | `dg-svr-865-tiny` | arm64 | sm8250 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 6.19 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Dragon Q6A | `radxa-dragon-q6a` | arm64 | qcs6490 | out of the box | `edge` 7.3 ✅<br>`vendor` 7.0 ✅ |
| Dragon Q8B | `radxa-dragon-q8b` | arm64 | sc8280xp | out of the box | `edge` 7.3 ✅<br>`vendor` 7.0 ✅ |
| Duo S (ARM) | `milkv-duos-arm` | arm64 | sophgo-sg200x-arm64 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| Duo S (RISC-V) | `milkv-duos-riscv` | riscv64 | sophgo-sg200x-riscv64 | unsupported arch | `edge` 7.3 |
| E24C | `radxa-e24c` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| E25 | `radxa-e25` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| E52C | `radxa-e52c` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| E54C | `radxa-e54c` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| EasePi-A2 | `easepi-a2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| EasePi-R2 | `easepi-r2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Edge2 | `khadas-edge2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Elish | `xiaomi-elish` | arm64 | sm8250 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 6.19 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| EM3588 | `boardcon-em3588` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Espressobin | `espressobin` | arm64 | mvebu64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Gateway Development Kit | `gateway-dk` | arm64 | ls1046a-ask | custom kernel | `current` 6.12 ❌ NET_SCH_NETEM, NET_CLS_BPF |
| Gateway GZ80x | `gateway-gz80x` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Helios4 | `helios4` | armhf | mvebu | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.6 ✅ |
| Helios64 | `helios64` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| IMB3588 | `imb3588` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| JetHub D1 | `jethubj100` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| JetHub D2 | `jethubj200` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| JetHub D3 | `jethubj310` | arm64 | meson-s7 | out of the box | `vendor` 5.15 ✅ |
| JetHub H1 | `jethubj80` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| K3 Pico-ITX | `k3picoitx` | riscv64 | spacemit-k3 | unsupported arch | `current` 6.18<br>`legacy` 6.18 |
| K3B | `kickpi-k3b` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅ |
| Kebab | `oneplus-kebab` | arm64 | sm8250 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| La Frite | `lafrite` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Le potato | `lepotato` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| LubanCat 5IO | `lubancat-5io` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Magicsee C400+ | `aml-c400-plus` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| MBa62xx | `mba62xx-tqma62xx` | arm64 | k3-tq | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| MBa67xx | `mba67xx-tqma67xx` | arm64 | k3-tq | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| MBa8MP-RAS314 | `mba8mpxl-ras314` | arm64 | imx8m | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| MBa8MPxL | `mba8mpxl` | arm64 | imx8m | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| MBa93xxCA | `mba93xxca-tqma93xx` | arm64 | imx93 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| MBa93xxLA-MINI | `mba93xxla-mini` | arm64 | imx93 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| mksklipad50 | `mksklipad50` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| MUSE Book | `musebook` | riscv64 | spacemit | unsupported arch | `current` 6.18<br>`legacy` 6.6 |
| MusePi Pro | `musepipro` | riscv64 | spacemit | unsupported arch | `current` 6.18<br>`edge` 7.3<br>`legacy` 6.6 |
| NanoPC-T3+ | `nanopct3plus` | arm64 | s5p6818 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| NanoPi Fire3 | `nanopifire3` | arm64 | s5p6818 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| NanoPi K1+ | `nanopik1plus` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi K2 | `nanopik2-s905` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi M3 / NanoPC-T3 | `nanopim3` | arm64 | s5p6818 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| NanoPi M4V2 | `nanopim4v2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi M5 | `nanopi-m5` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| NanoPi R3S LTS | `nanopi-r3s-lts` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R4S | `nanopi-r4s` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R6/M6, NanoPC T6 | `friendlyelec-rk3588` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| NanoPi R76S | `nanopi-r76s` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Nio 12L | `radxa-nio-12l` | arm64 | genio | custom kernel | `edge` 7.3 ❌ NET_SCH_PRIO |
| Nova | `luckfox-rk3308b-nova` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid C1 | `odroidc1` | armhf | meson | out of the box | `current` 6.12 ✅ |
| Odroid C2 | `odroidc2` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid C4 | `odroidc4` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid HC4 | `odroidhc4` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid M1 | `odroidm1` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid M2 | `odroidm2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid N2 | `odroidn2` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid U2 | `odroidu2` | armhf | odroidu2 | custom kernel | `current` 6.18 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`)<br>`edge` 7.3 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Odroid XU4 | `odroidxu4` | armhf | odroidxu4 | custom kernel | `current` 6.6 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`)<br>`edge` 7.2 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Orange Pi 4 LTS | `orangepi4-lts` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi 5 | `orangepi5` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi 5 Ultra | `orangepi5-ultra` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi 5+ | `orangepi5-plus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi One | `orangepione` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi PC2 | `orangepipc2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi Zero 3W | `orangepizero3w` | arm64 | sun60iw2 | out of the box | `vendor` 6.6 ✅ |
| OrangePi 3 LTS | `orangepi3-lts` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Pi RK3308B | `sakurapi-rk3308b` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅ |
| PocketBeagle 2 | `pocketbeagle2` | arm64 | k3-beagle | custom kernel | `vendor` 6.12 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.12 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Qidi-x6 | `qidi-x6` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Quadra | `inovato-quadra` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| R1 v3 | `youyeetoo-r1-v3` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| R58-4X4 | `mekotronics-r58-4x4` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| R58HD | `mekotronics-r58hd` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| R58S2 | `mekotronics-r58s2` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Raspberry Pi | `rpi4b` | arm64 | bcm2711 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| reComputer RK3576 | `recomputer-rk3576-devkit` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| reComputer RK3588 | `recomputer-rk3588-devkit` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Renegade | `renegade` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| RG DS | `anbernic-rg-ds` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅ |
| RG Vita Pro | `anbernic-rg-vita-pro` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅ |
| ROCK 2A | `rock-2a` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| ROCK 2F | `rock-2f` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 3A | `rock-3a` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 4D | `radxa-rock-4d` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 5 ITX | `rock-5-itx` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 5A | `rock-5a` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 5B | `rock-5b` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 5B+ | `rock-5b-plus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 5C | `rock-5c` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock 5T | `rock-5t` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Rock S0 | `rock-s0` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi 4B+ | `rockpi-4bplus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi E | `rockpi-e` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi S | `rockpi-s` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| RockPro 64 | `rockpro64` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| SBC3568 | `boardcon-sbc3568` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| SK-AM62-LP | `sk-am62-lp` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-AM62-SIP | `sk-am62-sip` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-AM62B | `sk-am62b` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-AM62P | `sk-am62p` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-AM64B | `sk-am64b` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-AM68 | `sk-am68` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-AM69 | `sk-am69` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| SK-TDA4VM | `sk-tda4vm` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Sweet Potato | `sweet-potato` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Tanix TX6s | `tanix-tx6s` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Tanix TX6s AXP313 | `tanix-tx6s-axp313` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| ThinkPad X13s | `thinkpad-x13s` | arm64 | sc8280xp | out of the box | `edge` 7.3 ✅ |
| Tinker Board | `tinkerboard` | armhf | rockchip | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| TMDS62LEVM | `tmds62levm` | arm64 | k3 | custom kernel | `vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| TMDS64EVM | `tmds64evm` | arm64 | k3 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-edge` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`vendor-rt` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| TQ-SMARC2-TQMa8MPxS | `tq-smarc2-tqma8mpxs` | arm64 | imx8m | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Turris Omnia | `turris-omnia` | armhf | mvebu | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Udoo | `udoo` | armhf | imx6 | custom kernel | `current` 6.18 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`)<br>`edge` 7.1 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| UEFI arm64 | `uefi-arm64` | arm64 | arm64 | some branches | `cloud` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT<br>`current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| UEFI Loongarch64 | `uefi-loong64` | loong64 | uefi-loong64 | unsupported arch | `current`<br>`edge` |
| UEFI x86 | `uefi-x86` | amd64 | x86 | some branches | `cloud` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT<br>`current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| UEFI/DT arm64 | `uefi-arm64-dt` | arm64 | uefi-arm64-dt | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| UNO Q | `arduino-uno-q` | arm64 | qrb2210 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Verdin iMX8M Mini | `verdin-imx8mm` | arm64 | imx8m | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| VIM1 | `khadas-vim1` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| VIM1S | `khadas-vim1s` | arm64 | meson-s4t7 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT<br>`legacy` 5.15 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| VIM2 | `khadas-vim2` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| VIM3 | `khadas-vim3` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| VIM3L | `khadas-vim3l` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| VIM4 | `khadas-vim4` | arm64 | meson-s4t7 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT<br>`legacy` 5.15 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| YY3588 | `youyeetoo-yy3588` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Zero | `radxa-zero` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| ZERO 3 | `radxa-zero3` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Zero 3D Printer | `sovol-zero` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |

## Community-maintained boards (`.csc`)

<details>
<summary>All 246 community boards</summary>

| Board | Board file | Arch | Kernel family | Projection | Branches |
|---|---|---|---|---|---|
| 100ASK DShanPI A1 | `dshanpi-a1` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| 100ASK DshanPi A1 CM5 on Waveshare CM5-to-Pi5-Adapter | `dshanpi-a1-cm5-waveshare-cm5-to-pi5-adapter` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| 100ASK DshanPi R1 / R1+ | `dshanpi-r1` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| 29X | `bobcat-29x` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| A13-SOM | `olinux-som-a13` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| A20 OLinuXino Lime | `lime` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| A20 OLinuXino Lime 2 | `lime2` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| A20 OLinuXino Micro | `olimex-a20-olinuxino-micro` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| A33 OLinuXino | `lime-a33` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Agilex 5 SoCDK | `agilex5-socdk` | arm64 | agilex5 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| AIBOX-K3 | `firefly-aibox-k3` | riscv64 | spacemit-k3 | unsupported arch | `legacy` 6.18 |
| AIM7 IO | `armsom-aim7-io` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Alta | `aml-a311d-cc` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| AM40 | `smart-am40` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Avaota A1 | `avaota-a1` | arm64 | sunxi64 | out of the box | `edge` 7.3 ✅ |
| Ayn Odin2 | `ayn-odin2` | arm64 | sm8550 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Ayn Odin2 Mini | `ayn-odin2mini` | arm64 | sm8550 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Ayn Odin2 Portal | `ayn-odin2portal` | arm64 | sm8550 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Ayn Odin3 | `ayn-odin3` | arm64 | sm8750 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Ayn Thor | `ayn-thor` | arm64 | sm8550 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Banana Pi M1+ | `bananapim1plus` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi M2 | `bananapim2` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi M2 Ultra | `bananapim2ultra` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi M2 Zero | `bananapim2zero` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi M2+ | `bananapim2plus` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi M2Pro | `bananapim2pro` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Banana Pi M3 | `bananapim3` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Banana Pi M64 | `bananapim64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Banana Pi R2 Pro | `bananapir2pro` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Banana Pi R3 Mini | `bananapir3mini` | arm64 | filogic | custom kernel | `current` 6.18 ❌ NET_SCH_NETEM |
| Banana Pi R4 | `bananapir4` | arm64 | filogic | custom kernel | `current` 6.18 ❌ NET_SCH_NETEM<br>`edge` 7.2 ❌ NET_SCH_NETEM |
| BananaPi M4 | `bananapim4` | arm64 | realtek-rtd129x | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| BananaPi W2 | `bananapiw2` | arm64 | realtek-rtd129x | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Blade 3 | `mixtile-blade3` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| CherryBa M1 | `cherryba-m1` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Clearfog Base | `clearfogbase` | armhf | mvebu | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| CM3588 NAS | `cm3588-nas` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| CM5 | `rock-5-cmio` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| CM5 CM4 IO | `armsom-cm5-rpi-cm4-io` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| CM5 EVB | `coolpi-cm5` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅ |
| CM5 IO (RK3576) | `armsom-cm5-io` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| CM5 on RPi CM4 IO | `rock-5-cm-rpi-cm4-io` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| CM7s IO | `armsom-cm7s-io` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| CNIoT-CORE | `cainiao-cniot-core` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Core3566 | `luckfox-core3566` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Core3588E | `mixtile-core3588e` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Cubie A5E | `radxa-cubie-a5e` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Cubie A7Z | `cubie-a7z` | arm64 | sun60iw2 | out of the box | `vendor` 6.6 ✅ |
| Cubieboard 1 | `cubieboard` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Cubieboard 2 | `cubieboard2` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Cubieboard 4 | `cubieboard4` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF |
| Cubietruck | `cubietruck` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Cyber Aib RK3588 | `cyber-aib-rk3588` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| DEBIX Model A | `debix-model-a` | arm64 | imx8m | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| DEBIX Model C | `debix-model-c` | arm64 | imx93 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| DevTerm A06 | `clockworkpi-a06` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Dusun DSOM 010R SoM | `dusun-dsom-010r` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| E20C | `radxa-e20c` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| EasePi-R1 | `easepi-r1` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| ECB41-PGE | `ebyte-ecb41-pge` | armhf | rockchip | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| Edge | `khadas-edge` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Edge 2 | `mixtile-edge2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| ELTAY RM66 | `eltay-rm66` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| EMB-3531 | `norco-emb-3531` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Fine3399 | `fine3399` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Fly-C5 | `mellowflyc5` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Forge1 | `armsom-forge1` | armhf | rockchip | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| FxBlox RK1 | `fxblox-rk1` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| G3568 v2 | `graperain-g3568-v2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| GenBook | `coolpi-genbook` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| GL-MT2500 | `gl-mt2500` | arm64 | filogic | custom kernel | `edge` 7.2 ❌ NET_SCH_NETEM |
| GL-MT2500 v2 | `gl-mt2500-v2` | arm64 | filogic | custom kernel | `edge` 7.2 ❌ NET_SCH_NETEM |
| H28k | `hinlink-h28k` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| H66K | `hinlink-h66k` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| H68K | `hinlink-h68k` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| H88K | `hinlink-h88k` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| HiKey 960 | `hikey960` | arm64 | arm64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| HNAS | `hinlink-hnas` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| HT2 | `hinlink-ht2` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Iagent Recore | `recore` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Indiedroid Nova | `indiedroid-nova` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| ITX-3588J | `firefly-itx-3588j` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| K2B | `kickpik2b` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| K2B V2.2 DDR3 | `kickpi-k2b-v2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| LCKFB Taishan Pi | `lckfb-taishanpi` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Leez P710 | `leez-p710` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Longan Pi 3H | `longanpi-3h` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| LonganPi 4B | `longanpi-4b` | arm64 | sun55iw3-syterkit | custom kernel | `legacy` 5.15 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| LubanCat 3 v2 | `lubancat-3-v2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| LubanCat 4 | `lubancat-4` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Lubancat2 | `lubancat2` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅ |
| Lyra Ultra / Ultra W | `luckfox-lyra-ultra-w` | armhf | rockchip | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| Lyra Zero W | `luckfox-lyra-zero-w` | armhf | rockchip | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| Lyra+ | `luckfox-lyra-plus` | armhf | rockchip | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| M28K | `mangopi-m28k` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| MaaXBoard 8ULP | `maaxboard-8ulp` | arm64 | imx8ulp | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Mele A1000 | `melea1000` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| MK808C | `mk808c` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| mkspi | `mkspi` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| MQ Pro | `mangopi-mq` | riscv64 | d1 | unsupported arch | `edge` 7.3 |
| Nano | `easy-eai-nano` | armhf | rockchip-rv1126 | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| NanoPC T4 | `nanopct4` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi A64 | `nanopia64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi Air | `nanopiair` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| NanoPi Duo | `nanopiduo` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| NanoPi Duo2 | `nanopiduo2` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| NanoPi M4 | `nanopim4` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi Neo | `nanopineo` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| NanoPi Neo 2 | `nanopineo2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi Neo 2 Black | `nanopineo2black` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi Neo 3 | `nanopineo3` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi Neo 4 | `nanopineo4` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi Neo Core 2 | `nanopineocore2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi Neo+ 2 | `nanopineoplus2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi NEO3+ | `nanopi-neo3-plus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R1 | `nanopi-r1` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| NanoPi R1S H5 | `nanopi-r1s-h5` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| NanoPi R28S | `nanopi-r28s` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| NanoPi R2C | `nanopi-r2c` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R2S | `nanopi-r2s` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R2S+ | `nanopi-r2s-plus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R3S | `nanopi-r3s` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R4SE | `nanopi-r4se` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R5C | `nanopi-r5c` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi R5S | `nanopi-r5s` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| NanoPi Zero2 | `nanopi-zero2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| NextThing C.H.I.P. | `pocketchip-sd` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF |
| NuMaker IoT MA35D16F90 | `numaker-iot-ma35d16f90` | arm64 | nuvoton-ma35d1 | custom kernel | `vendor` 5.10 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| ODROID M1S | `odroidm1s` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid N2L | `odroidn2l` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Odroid U3 | `odroidu3` | armhf | odroidu2 | custom kernel | `current` 6.18 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`)<br>`edge` 7.3 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Odroid X2 | `odroidx2` | armhf | odroidu2 | custom kernel | `current` 6.18 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`)<br>`edge` 7.3 ❌ BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| OK3506-S12 | `forlinx-ok3506-s12` | armhf | rockchip | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| OneCloud | `onecloud` | armhf | meson | out of the box | `current` 6.12 ✅ |
| Orange Pi 2 | `orangepi2` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi 3 | `orangepi3` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi 3B | `orangepi3b` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi 4 | `orangepi4` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi 4 Pro | `orangepi4pro` | arm64 | sun60iw2 | out of the box | `vendor` 6.6 ✅ |
| Orange Pi 4A | `orangepi-4a` | arm64 | sunxi64 | out of the box | `edge` 7.3 ✅ |
| Orange Pi 5 Max | `orangepi5-max` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi 5 Pro | `orangepi5pro` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi 5B | `orangepi5b` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Orange Pi A31s | `orangepia31s` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF |
| Orange Pi Lite | `orangepilite` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi Lite 2 | `orangepilite2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi One+ | `orangepioneplus` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi PC | `orangepipc` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi PC + | `orangepipcplus` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi Prime | `orangepiprime` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi R1 | `orangepi-r1` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi R1+ | `orangepi-r1plus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi R1+ LTS | `orangepi-r1plus-lts` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi R2S | `orangepir2s` | riscv64 | spacemit | unsupported arch | `current` 6.18<br>`legacy` 6.6 |
| Orange Pi RK3399 | `orangepi-rk3399` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi RV2 | `orangepirv2` | riscv64 | spacemit | unsupported arch | `current` 6.18<br>`legacy` 6.6 |
| Orange Pi Win | `orangepiwin` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi Zero | `orangepizero` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi Zero+ | `orangepizeroplus` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi Zero+ 2 H3 | `orangepizeroplus2-h3` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi Zero+ 2 H5 | `orangepizeroplus2-h5` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Orange Pi Zero2 | `orangepizero2` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi Zero2W | `orangepizero2w` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi Zero3 | `orangepizero3` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Orange Pi+ | `orangepiplus` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Orange Pi+ 2E | `orangepiplus2e` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Pad 6S Pro | `xiaomi-sheng` | arm64 | sm8550-sheng | custom kernel | `edge` 7.1 ❌ NET_SCH_NETEM, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT |
| PBS TV Stick | `pbstvstick` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| pcDuino 3 | `pcduino3` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Photonicat2 | `photonicat2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅ |
| Phytium Pi | `phytiumpi` | arm64 | phytium-embedded | custom kernel | `vendor` 6.6 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Pi 1B | `walnutpi-1b` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pi 1B DDR3 | `walnutpi-1b-ddr3` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pi Box | `walnutpi-box` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pi ZeroW | `walnutpi-zerow` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pico Mini | `luckfox-pico-mini` | armhf | rockchip-rv1106 | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Pico Pro / Pico Max | `luckfox-pico-max` | armhf | rockchip-rv1106 | custom kernel | `vendor` 6.1 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Pine A64 / A64+ | `pine64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Pine Cube | `pinecube` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Pine H64 | `pineh64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Pine H64 model B | `pineh64-b` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Pine Quartz64 A | `quartz64a` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pine Quartz64 B | `quartz64b` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pinebook A64 | `pinebook-a64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Pinebook Pro | `pinebook-pro` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Pocket RP5 | `retroidpocket-rp5` | arm64 | sm8250 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 6.19 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Pocket RPMini | `retroidpocket-rpmini` | arm64 | sm8250 | custom kernel | `current` 6.18 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF<br>`edge` 6.19 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| Quartz64 Zero | `quartz64-zero` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| R10D | `torder-r10d` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| R3300-L | `bestv-r3300-l` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| R58 MiniPC | `mekotronics-r58-minipc` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| R58X | `mekotronics-r58x` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| R58X-4G | `mekotronics-r58x-4g` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| R58X-Pro | `mekotronics-r58x-pro` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| reComputer RK3576 Industrial | `recomputer-rk3576-industrial` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| reComputer RK3576 Module Dev Kit | `recomputer-rk3576-module-devkit` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Renegade Elite | `roc-rk3399-pc` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Retro Lite CM5 | `retro-lite-cm5` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| RG Rotate | `anbernic-rg-rotate` | arm64 | sprd-ums512 | custom kernel | `edge` 7.2 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| RK3328-Heltec | `rk3328-heltec` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| RK3399 | `firefly-rk3399` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| RK3566 BOX DEMO | `rk3566-box-demo` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rock 3C | `rock-3c` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rock 4SE | `rock-4se` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rock 64 | `rock64` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi 4A | `rockpi-4a` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi 4B | `rockpi-4b` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi 4C | `rockpi-4c` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi 4C+ | `rockpi-4cplus` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Rockpi N10 | `rockpi-n10` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Sige1 | `armsom-sige1` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Sige3 | `armsom-sige3` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| Sige5 | `armsom-sige5` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Sige6 | `armsom-sige6` | arm64 | sun60iw2 | out of the box | `vendor` 6.6 ✅ |
| Sige7 | `armsom-sige7` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Sige7s | `armsom-sige7s` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅ |
| Solitude | `aml-s905d3-cc` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| SoPine A64 | `pine64so` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Station M1 | `station-m1` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Station M2 | `station-m2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Station M3 | `station-m3` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`vendor` 6.1 ✅ |
| Station P1 | `station-p1` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Station P2 | `station-p2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| SV08 | `sovol-sv08` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| SV08 Max | `sovol-sv08-max` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| T3 | `xpressreal-t3` | arm64 | realtek-rtd1619b | custom kernel | `vendor` 6.6 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_SCH_INGRESS, NET_CLS_BPF, NET_CLS_ACT, BPF_EVENTS (probably), no BTF (`KERNEL_BTF=no`) |
| Taishan Pi 3M | `lckfb-taishanpi-3m` | arm64 | rockchip64 | out of the box | `edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Teres A64 | `olimex-teres-a64` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Tinker Board 2 | `tinkerboard-2` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Tinker Edge R | `tinker-edge-r` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Tritium H3 | `tritium-h3` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF<br>`legacy` 6.12 ❌ NET_CLS_BPF |
| Tritium H5 | `tritium-h5` | arm64 | sunxi64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| Turing RK1 | `turing-rk1` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| UEFI riscv64 | `uefi-riscv64` | riscv64 | riscv64 | unsupported arch | `current` 6.18<br>`edge` 7.3<br>`legacy` 6.12 |
| UEFI x86 (QEMU) | `qemu-uefi-x86` | amd64 | x86 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`legacy` 6.12 ✅ |
| uefi-arm64 | `qemu-uboot-arm64` | arm64 | arm64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| uefi-x86 | `qemu-uboot-x86` | amd64 | x86 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| VisionFive2 | `visionfive2` | riscv64 | starfive2 | unsupported arch | `vendor` 6.6 |
| W3 | `armsom-w3` | arm64 | rk35xx | out of the box | `vendor` 6.1 ✅ |
| X3568 v4 | `9tripod-x3568-v4` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| XC3399FR | `xc3399fr` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| xiaobao-nas | `xiaobao-nas` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| Y700 Gen4 | `lenovo-y700-gen4` | arm64 | sm8750 | custom kernel | `edge` 7.3 ❌ NET_SCH_HTB, NET_SCH_NETEM, NET_SCH_PRIO, NET_CLS_BPF |
| YY3568 | `yy3568` | arm64 | rockchip64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅<br>`vendor` 6.1 ✅ |
| Zero 2 | `radxa-zero2` | arm64 | meson64 | out of the box | `current` 6.18 ✅<br>`edge` 7.3 ✅ |
| ZeroPi | `zeropi` | armhf | sunxi | custom kernel | `current` 6.18 ❌ NET_CLS_BPF<br>`edge` 7.3 ❌ NET_CLS_BPF |


</details>

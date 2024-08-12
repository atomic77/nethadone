# Installation

## Requirements

Nethadone has been tested on the latest Ubuntu Noble-based versions of
Armbian (24.04 or 24.08). You can download an image for your board at
the [Armbian download page](https://www.armbian.com/download/?tx_maker=xunlong).


## Image prep

The contents of `scripts/customize-image.sh` will need to be run, 
as root, in a fresh installation of the image you downloaded. 
If you want to save time, you can prep using the chroot build 
script, eg:

```bash
cd scripts
./build.sh -i /tmp/Armbian_community_24.8.0-trunk.554_Orangepizero_noble_current_6.6.43.img -m ~/mnt -a arm
```

`build.sh` makes liberal use of `sudo` so you may want to run this
on a VM or cloud server with a fast link.

Replace the `-i` and `-m` options with wherever you uncompressed the
Armbian image above, and an empty folder to use for a mount point.
The `-a` option indicates whether you are building for an arm64
or armv7 image.

> [!TIP]  
>  `build.sh` is just a chroot wrapper that pre-installs everything
required for nethadone to run. If you prefer to do the installation 
of required packages directly on the SBC, you can copy paste from 
`customize-image.sh` after you have flashed the stock image using 
[Balena etcher](https://etcher.balena.io/) or whichever flashing tool you prefer.


At a high level, the `customize-image.sh` script:

* Installs and configures compiler dependencies, bpftool and prometheus
* Creates a systemd service for nethadone to run on boot
* Downloads and installs a BTF-enabled kernel
* Downloads the nethadone binary from Github


## Nethadone post-installation

The easiest way to test out Nethadone is with a board like the
Zero or R1 that has functional wifi with Armbian. You can spin
up a [hotspot](https://ubuntu.com/core/docs/networkmanager/configure-wifi-access-points) with `nmcli`, eg:

```bash
sudo nmcli dev wifi hotspot ifname wlan0 ssid nethadone password $password channel 8 band bg
```

The network-manager hotspot conveniently sets up a LAN for you
for any devices that connect wirelessly (default seems to be `10.42.0.0/24`), and NAT out to your wired interface, which should be plugged in
directly to your primary router.

> [!NOTE]
> The network-manager hotspot also has the advantage of setting up DNS
> caching automatically with `dnsmasq`. This makes it easier for 
> nethadone to capture these packets, as clients receiving a local
> IP as a DNS server in DHCP configuration typically do not use DNS-over-HTTPS.

You can drop the `nmcli` command you used into `/etc/rc.local` to have
it start up on reboot.

### Adjusting cofiguration

If you want to customize any parameters, or have different interface
names, you can create a `/etc/nethadone.yml` file. 
See the sample [config](../config/nethadone.yml) for more details.

## Wired-only Deployment

Faster devices like the OrangePi R1+, as of my last tests, do not have
functional wifi with stock Armbian. This means they need to be introduced
between your wifi router and your device providing internet access using
the two ethernet ports.

This provides fast, full coverage for all devices in your home, but could take down your internet in the event of an issue with nethadone.
As this is still very much in development, it's recommended to stick
with a secondary access point for now.

If you have access to an extra wifi router, this is an example of how
you can still make use of a wired-only device:

![Secondary network](nethadone-secondary-network.drawio.png)

In such as case, you will need to set up NAT for the interface
connected to the , eg. if `eth0` is your wan interface:

```bash
iptables -t nat -A POSTROUTING -o end0 -j MASQUERADE
```

*TODO - properly document other steps required for this scenario*

## Runtime logs

If all goes well and nethadone is routing and inspecting traffic, 
you should see be able to see log entries like:

```bash
$ journalctl -u nethadone.service --follow
...
2024/07/20 15:48:58 Registering prometheus metrics
2024/07/20 15:48:58 Setting up SimpleLoadAverage policy
2024/07/20 15:48:58 Setting up metrics collector
2024/07/20 15:48:58 Setting up metrics collector

 ┌───────────────────────────────────────────────────┐
 │                   Fiber v2.52.4                   │
 │               http://127.0.0.1:3000               │
 │       (bound on host 0.0.0.0 and port 3000)       │
 │                                                   │
 │ Handlers ............ 18  Processes ........... 1 │
 │ Prefork ....... Disabled  PID ............ 185115 │
 └───────────────────────────────────────────────────┘
2024/07/20 15:51:41 Tick happened, collected  6  pairs
15:51:44 | 200 |     706.661µs | 127.0.0.1 | GET | /metrics | -
2024/07/20 15:51:56 Tick happened, collected  22  pairs
...
2024/07/20 15:52:25 Checking policy
2024/07/20 15:52:25 Found  0  globs for throttling
2024/07/20 15:52:25 0  globs that can be decreased
```

You can inspect the qdisc and class configuration with:

```bash
$ sudo tc -s class show dev lan0
class htb 1:10 parent 1:1 leaf 10: prio 0 rate 95Mbit ceil 95Mbit burst 1579b cburst 1579b
 Sent 0 bytes 0 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
 lended: 0 borrowed: 0 giants: 0
 tokens: 2093 ctokens: 2093

class htb 1:1 root rate 100Mbit ceil 100Mbit burst 1600b cburst 1600b
 Sent 0 bytes 0 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
 lended: 0 borrowed: 0 giants: 0
 tokens: 2000 ctokens: 2000

...

class htb 1:60 parent 1:1 leaf 60: prio 0 rate 50Kbit ceil 50Kbit burst 1600b cburst 1600b
 Sent 0 bytes 0 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
 lended: 0 borrowed: 0 giants: 0
 tokens: 4000000 ctokens: 4000000

$ sudo tc -s qdisc show dev lan0
qdisc htb 1: root refcnt 2 r2q 10 default 0x10 direct_packets_stat 3568 direct_qlen 1000
 Sent 11025865 bytes 8567 pkt (dropped 0, overlimits 0 requeues 1)
 backlog 0b 0p requeues 1
qdisc netem 30: parent 1:30 limit 1000 delay 20ms  2ms
 Sent 0 bytes 0 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
...
qdisc clsact ffff: parent ffff:fff1
 Sent 11025865 bytes 8567 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0

```

There is a basic admin interface where you can configure glob
groups, see the currently active policy and inspect currently
mapped DNS to IP addresses, it will be running at:

http://orangepi-r1.local:3000/

mDNS is configured by default - if this address does not resolve
on your network, replace `r1plus.local` with whatever IP address
was assigned. 

Example glob group configuration:

![glob groups](glob_groups.png)

If DNS packet inspection is working, you should see IP addresses
mapped to likely domains on the bandwidth page:

![bandwidth](bandwidth.png)

The local prometheus server that is used for policy decisions
should be available on port 9090, and you can check that metrics
are being collected there using similar queries used by nethadone:


![prometheus](prometheus.png)

## Other boards 

Armbian does not ship with BTF-enabled kernels out of the box at this
time; the two kernel builds available for Armbian `BOARDFAMILY` 
[`rockchip64`](https://github.com/armbian/build/blob/752ba047b3cb600095f981cbabb8be53d12bb9a2/config/boards/orangepi-r1plus.csc#L3) 
and [`sunxi`](https://github.com/armbian/build/blob/752ba047b3cb600095f981cbabb8be53d12bb9a2/config/boards/orangepi-r1.csc#L3), are targeted for the Orange Pi R1 Plus
and R1 and Zero respectively. Others with the same family are likely,
but not certain to work.

If you are interested in trying out another board and are having 
trouble getting a BTF kernel working, please raise an issue and I'll
try adding it to the Github Action.
## QDisc Notes

For reasons I've yet to understand, less data that goes into `lan0` seems to go through 
the qdisc than if throttling is attempted on `eth0`. But this prevents us from applying
source-based filtering, since by the time the packet arrives on `eth0`, it has been natted
to appear as if it's coming from the ip of `eth0`. 

The following qdisc structure still allows much more data through on `iperf3` tests than these 
rate limits suggest, and the quantity of data that runs through the bpf classifier is maybe 10-25%
of the actual transfer, but it does a good enough job of deteriorating traffic that we can 
leave this aside for another day.

Example of qdisc setup that we can use to steadily deteriorate traffic:

```bash

sudo tc qdisc delete dev lan0 root

sudo tc qdisc add dev lan0 root handle 1: htb default 10

sudo tc class add dev lan0 parent 1: classid 1:1 htb rate 100Mbit ceil 100Mbit

sudo tc class add dev lan0 parent 1:1 classid 1:10 htb rate 95Mbit ceil 100Mbit
sudo tc class add dev lan0 parent 1:1 classid 1:20 htb rate 500kbit ceil 500kbit
sudo tc class add dev lan0 parent 1:1 classid 1:30 htb rate 50kbit ceil 50kbit

sudo tc qdisc add dev lan0 parent 1:10 handle 10: sfq perturb 10
sudo tc qdisc add dev lan0 parent 1:20 handle 20: netem delay 25ms 5ms
sudo tc qdisc add dev lan0 parent 1:30 handle 30: netem delay 250ms 50ms

sudo tc filter add dev lan0 protocol ip parent 1:0 bpf obj ebpf/throttle.o classid 1: direct-action

```


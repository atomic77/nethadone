## Nethadone

A pihole-inspired adaptive L4 router to discourage addictive web usage.

## Background

Social media addiction is a topic that has received growing attention in recent years. My own struggle with this has been ongoing. After years of improvised cold-turkey style solutions (modifying my hosts file, applying filters in a pihole and screen time filters on my phone), I decided to see if I could come up with something a little more sophisticated. 

Ironically, the inspiration came from a flaky orangepi zero I had reinstalled pihole onto. I noticed that a site I was in a doom-scroll on was having trouble loading, and this annoyed me to the point that I stopped. After checking to see if it was the DNS resolution failing, I realized I had stumbled onto something.

## Getting Started

Not quite usable yet :) Stay tuned. 

## Design Goals

* Protect all devices in a network with zero client-side configuration or software
* Dynamically throttle traffic from clients to configurable sites or groups of sites to "train" good habits [1]
* Use only IP and (sniffed) DNS (i.e. as close to a pure L4 solution as possible)
* Introduce no latency on "good" traffic
* Usable on minimal hardware like an SBC

What I came up with is an eBPF-based tool that is intended to be run on a SBC with two network interfaces, and introduces an additional hop between your cable modem / fiber / dsl etc. and your local access point/wifi router.

## How does it work? 

A Traffic Control (TC) eBPF program is attached to the interfaces on the device, and monitors bandwidth used from all devices on the network. A second eBPF program sniffs for DNS packets , in order to provide a means to classify destination IPs . Based on traffic patterns, a steadily increasing amount of latency is introduced to "bad" traffic until some window of time passed, after which . The mechanism is described in more detail in this paper [2]



[1] What is a "good habit" you say? A blunt filter of all Facebook traffic might be better than rotting away all day fighting with people on the other side of the political spectrum. But it blocks positive uses for these platforms. I've always admired people who have the discipline to drop in to Facebook or Instagram, respond to a message or browse for a couple of minutes to take a break, and then pop back up without getting sucked in. This type of usage should be rewarded with fast responses, while doom-scrolls should get progressively slower to nudge the user off.
[2] https://netdevconf.info//0x14/pub/papers/55/0x14-paper55-talk-paper.pdf



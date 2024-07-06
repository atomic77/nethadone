# Policy-related logic for nethadone

There are many different ways we could approach the problem of deciding what
"undesirable" access patterns look like from the perspective of a router. 

Over time, we may add multiple policy types and a way of configuring them more flexibly. 

## Base Definitions

Let:

-  $B = \set{b_0, ... b_n}$ represent bandwidth classes, where $B_0$ is
unrestricted access, and $B_n$ represents the maximum service 
degradation.
- $P$ is the set of $(s, d)$ source and destination IP pair/tuples seen
- $G = \set{d_1, ..., d_m}$ is a set of destination IP addresses
that match a given restriction policy, eg. `*.scroll.forever.com` 

At a given point of time, a policy is one that maps a $(s,d)$ IP pair 
to a bandwidth class, i.e. $f: P \to B$

$G$ is called tentatively a "glob group" in the code but probably needs a better name.

## SimpleLoadAverage policy

Inspired by the classic unix load average triplet of 1, 5 and 15 minutes, 
aggregated traffic to all IPs matching a glob group is tracked using a 
prometheus query like:

```
sum by (src_ip, glob) (rate(ip_pair_vic_bytes_total{glob!=""}[1m]))
```

Bandwidth usage for different sites can vary, eg. video streaming will obviously consume much more than scrolling through
mostly text-based tweets/messages. From the point of view of nethadone,
both above use cases are undesireable when done excessively and 
compulsively, so assume a relatively low constant rate factor `k` that
indicates general use of some site. How much above this rate is ignored
for this simple policy.


### Policy

Let $k$ be a constant value in bytes that represents the rate per second
above which activity is assumed for a glob group $G$. Based on early experimentation this has been set to 1000 but could be tuned after collecting more usage data.

Let $g(G, s, t) = \sum_{d \in G} r_{dst} $, where $r_{dst}$ represents
the $s$-minute average rate for destination ip $d$ in group $G$ at time $t$ (i.e. the prometheus query above)

#### Increasing throttling

Assume $f(s,d) = B_c$

i.e. $c=0$ would be usual base case, i.e. no throttling enabled for an IP pair.

If all of

$$ g(G, 1, 0) > k $$
   
$$ g(G, 1, -5) > k $$
    
$$ g(G, 5, 0) > k $$

then $f(s,d) = B_{c+1}$ for $d \in G$

i.e. if the one-minute rate now and five minutes ago is above $k$, and the five-minute rate, increase the bandwidth restriction class, until we hit the max.


#### Decreasing throttling

Again assume $f(s,d) = B_c$

If $g(G, 5, 0) < k$ then $f(s,d) = B_{c-1}$, for $d \in G$

i.e. if the five-minute average goes below $k$, decrement the bandwidth class. 



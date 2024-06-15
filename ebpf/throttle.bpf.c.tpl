#include <linux/bpf.h>
#include <arpa/inet.h>
#include <linux/pkt_cls.h>
#include <stdint.h>
#include <iproute2/bpf_elf.h>

#include <stdint.h>
#include <stdbool.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <asm/types.h>
#include <linux/in.h>
#include <linux/if.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/if_tunnel.h>
#include <linux/filter.h>
#include <linux/bpf.h>
#include <linux/icmp.h>


#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define IP_ADDRESS(o1, o2, o3, o4) (unsigned int)(o1 + (o2 << 8) + (o3 << 16) + (o4 << 24))
#define MILLIS 1000*1000

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, __u32);
    __uint(max_entries, 2);
} throttle_stats SEC(".maps");

// id = 0 -> packets scanned 
// id = 1 -> packets throttled
static inline void increment_scanned() {
	__u32 id = 0;
    __u32 val = 0;
    __u32 *p = bpf_map_lookup_elem(&throttle_stats, &id);
    if (p != 0) {
        val = *p + 1;
    }
    bpf_map_update_elem(&throttle_stats, &id, &val, BPF_ANY);
}

static inline void increment_throttled() {
	__u32 id = 1;
    __u32 val = 0;
    __u32 *p = bpf_map_lookup_elem(&throttle_stats, &id);
    if (p != 0) {
        val = *p + 1;
    }
    bpf_map_update_elem(&throttle_stats, &id, &val, BPF_ANY);
}

SEC("tc")
int throttle(struct __sk_buff *skb)
{
    void *data_end = (void *)(__u64)skb->data_end;
    void *data = (void *)(__u64)skb->data;
    struct ethhdr *l2;
    struct iphdr *l3;
    
    if (skb->protocol != bpf_htons(ETH_P_IP))
        return TC_ACT_OK;

    l2 = data;
    if ((void *)(l2 + 1) > data_end)
        return TC_ACT_OK;

    l3 = (struct iphdr *)(l2 + 1);

    if ((void *)(l3 + 1) > data_end)
        return TC_ACT_OK;

    increment_scanned();
	
    if (l3->daddr == IP_ADDRESS(192,168,0,176)) {
      skb->tstamp = skb->tstamp + (500 * MILLIS);
      increment_throttled();
    }
    
    {{ range .FiltParams }}
    if (l3->daddr == IP_ADDRESS({{ .DestIpAddr }})) {
      skb->tstamp = skb->tstamp + ({{ .DelayMs }} * MILLIS);
      increment_throttled();
    }
    {{ end }}

    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";

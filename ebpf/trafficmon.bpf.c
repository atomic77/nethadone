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

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u64);
    __type(value, __u64);
    __uint(max_entries, 25000);
} src_dest_bytes SEC(".maps");

SEC("tc")
int traffic_mon(struct __sk_buff *skb)
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

    __u64 key = ((__u64) l3->saddr) << 32 | ((__u64)l3->daddr);
    __u64 val = bpf_ntohs(l3->tot_len);
    __u64 *p = bpf_map_lookup_elem(&src_dest_bytes, &key);
    if (p != 0) {
      val = *p + bpf_ntohs(l3->tot_len);
    }
    bpf_map_update_elem(&src_dest_bytes, &key, &val, BPF_ANY);

    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";
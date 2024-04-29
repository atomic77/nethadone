/* Old reference example; 
Actual changes should go to .tpl which gets rendered and injected
on the fly 
*/
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
    __type(key, __u64); // 64-bit int for src/dest IP addr pair
    __type(value, __u64); // number of packets 
    __uint(max_entries, 255);
} src_dest_bytes SEC(".maps");


#define IP_ADDRESS(o1, o2, o3, o4) (unsigned int)(o1 + (o2 << 8) + (o3 << 16) + (o4 << 24))
#define MILLIS 1000*1000


SEC("tc")
int tc_ingress(struct __sk_buff *skb)
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

    if (
      l3->saddr == IP_ADDRESS(192,168,0,108) && 
      l3->daddr == IP_ADDRESS(192,168,0,14)
    ) {
     __bpf_vprintk(
        "Got packet: tot_len: %d, ttl: %d, saddr: %pI4, daddr: %pI4, tstmp: %lld, cls: %d", 
        bpf_ntohs(l3->tot_len), 
        l3->ttl, &l3->saddr, &l3->daddr, skb->tstamp, skb->tc_classid
      );
      skb->tstamp = skb->tstamp + (10 * MILLIS);
      __u64 key = ((__u64) l3->saddr) << 32 | ((__u64)l3->daddr);
      __u64 val = bpf_ntohs(l3->tot_len);
      __u64 *p = bpf_map_lookup_elem(&src_dest_bytes, &key);
      if (p != 0) {
        val = *p + bpf_ntohs(l3->tot_len);
      } 
      bpf_map_update_elem(&src_dest_bytes, &key, &val, BPF_ANY);
      /* Sample python code for splitting 
        >>> i
        7782405698918328512
        >>> b0 = i >> 32
        >>> b1 = i & 0x00000000FFFFFFFF
        >>> socket.inet_ntoa(struct.pack("!I", b0))
        '108.0.168.192'
        >>> socket.inet_ntoa(struct.pack("!I", b1))
        '14.0.168.192'
      */
    }


    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";
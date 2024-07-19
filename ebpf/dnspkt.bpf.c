#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

#include <linux/bpf.h>
#include <linux/bpf_common.h>

#include <linux/if_ether.h>
#include <linux/if_packet.h>
#include <linux/in.h>
#include <linux/in6.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/pkt_cls.h>

#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

#include <bpf/bpf_core_read.h>
#include <bpf/bpf_tracing.h>

// Still not quite sure when you're supposed to include this and not..
// #include "vmlinux.h"

#define ETH_P_IP 0x0800 /* Internet Protocol packet	*/ // ipv4
#define ETH_HLEN 14 /* Total octets in header.	 */

#define DATA_LEN 508
struct payload_t {
	__u32 len;
    unsigned char data[DATA_LEN];
};


struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __type(key, __u32);
    __type(value, struct payload_t);
    __uint(max_entries, 1);
} tmp_map SEC(".maps");


struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
	__uint(key_size, sizeof(__u32));
	__uint(value_size, sizeof(__u32));
} dns_arr SEC(".maps");


struct eth_hdr {
	unsigned char   h_dest[ETH_ALEN];
	unsigned char   h_source[ETH_ALEN];
	unsigned short  h_proto;
};


SEC("tc")
int udp_dns_sniff(struct __sk_buff *skb)
{
	void *data = (void *)(long)skb->data;
	void *data_end = (void *)(long)skb->data_end;
	struct eth_hdr *eth = data;
	struct iphdr *ip = data + sizeof(*eth);
    struct udphdr *udp_hdr = data + sizeof(*eth) + sizeof(*ip);
	__u32 magic = sizeof(*eth) + sizeof(*ip) + sizeof(*udp_hdr);

	/* Since we only care about UDP packets, combine into single length check */
	 
	if (data + sizeof(*eth) + sizeof(*ip) + sizeof(*udp_hdr) > data_end)
		return TC_ACT_OK;

	if (
			eth->h_proto == bpf_htons(ETH_P_IP) &&
			ip->protocol == IPPROTO_UDP &&
			(udp_hdr->dest == bpf_htons(53) || udp_hdr->source == bpf_htons(53) )
		) {
		bpf_printk("udp port dest %d, src port: %d", bpf_htons(udp_hdr->dest), bpf_htons(udp_hdr->source));
		//, bpf_htons(udp_hdr->dest));// , skb->len, bpf_ntohs(udp_hdr->len));	

		__u32 id = 0;
		struct payload_t *payload = bpf_map_lookup_elem(&tmp_map, &id);
		if (!payload)
			return TC_ACT_UNSPEC;
	
		__u32 len = skb->len < DATA_LEN ? skb->len : DATA_LEN;
		bpf_printk("Submitting pkt len %d", len); 
		__u32 offset = sizeof(*eth) + sizeof(*ip) + sizeof(*udp_hdr);
		__u32 udp_len = skb->len - offset;
		payload->len = udp_len;
		bpf_probe_read_kernel(&payload->data, len, data + offset);

		bpf_perf_event_output(skb, &dns_arr, BPF_F_CURRENT_CPU, payload->data, sizeof(payload->data));
		
	}

	return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";

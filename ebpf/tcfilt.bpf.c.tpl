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


SEC("tc")
int tc_ingress(struct __sk_buff *skb)
{
    void *data_end = (void *)(__u64)skb->data_end;
    void *data = (void *)(__u64)skb->data;
    struct ethhdr *l2;
    struct iphdr *l3;
    
    // struct sockaddr_in sa;
    // unsigned int filt_addr;
    // char 

    if (skb->protocol != bpf_htons(ETH_P_IP))
        return TC_ACT_OK;

    l2 = data;
    if ((void *)(l2 + 1) > data_end)
        return TC_ACT_OK;

    l3 = (struct iphdr *)(l2 + 1);

    if ((void *)(l3 + 1) > data_end)
        return TC_ACT_OK;

    // Basic working:
    // sudo tc filter add dev enp0s3 egress bpf direct-action obj tcfilt.o sec tc
    //
    // In order to make use of the tstamp marker for packets, need to switch interface
    // to fair queuing mode:
    // sudo tc qdisc replace dev enp0s3 root fq
    // Then add filter as usual:
    // sudo tc filter add dev enp0s3 egress bpf direct-action obj tcfilt.o sec tc
    

    // if (bpf_ntohs(l3->tot_len) > 300) {
    if (
      l3->saddr == IP_ADDRESS({{ .SrcIpAddr}}) && 
      l3->daddr == IP_ADDRESS({{ .DestIpAddr}})
    ) {
     __bpf_vprintk(
        "Got packet: tot_len: %d, ttl: %d, saddr: %pI4, daddr: %pI4, tstmp: %lld, cls: %d", 
        bpf_ntohs(l3->tot_len), 
        l3->ttl, &l3->saddr, &l3->daddr, skb->tstamp, skb->tc_classid
      );
      skb->tstamp = skb->tstamp + ({{ .DelayMs }} * MILLIS);
      
    }


    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";

# Outstanding issues

## DNS Over HTTPS

[DoH](https://datatracker.ietf.org/doc/html/rfc8484) mitigates the DNS sniffing that is ultimately a required "feature" for 
nethadone to work. This is increasingly enabled by default on many browsers and systems. There are two ways I've found to
avoid this:

1) Configure your browser to use the system configured DNS server (Cumbersome for clients and not ideal)
2) Configure a network-local DNS server proxy (eg. Pihole, perhaps running on the same system as nethadone); 
   this seems to cause most clients to avoid DoH

More research on this required

## DNS CNAME Handling

Will have to figure out how to deal with CNAME chains like `media.licdn.com` used by LinkedIn, so that
we can associate the original request with the final IPs that are being used:

```bash

$ dig media.licdn.com

; <<>> DiG 9.16.48-Ubuntu <<>> media.licdn.com
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 36730
;; flags: qr rd ra; QUERY: 1, ANSWER: 5, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;media.licdn.com.               IN      A

;; ANSWER SECTION:
media.licdn.com.        64      IN      CNAME   2-01-2c3e-005c.cdx.cedexis.net.
2-01-2c3e-005c.cdx.cedexis.net. 118 IN  CNAME   od.linkedin.edgesuite.net.
od.linkedin.edgesuite.net. 13326 IN     CNAME   a1916.dscg2.akamai.net.
a1916.dscg2.akamai.net. 17      IN      A       206.248.168.162
a1916.dscg2.akamai.net. 17      IN      A       206.248.168.145

```

DNS record repr dump:

```go
&dns.Msg{MsgHdr: dns.MsgHdr{Id: 54874, Response: true, RecursionDesired: true, RecursionAvailable: true},
	Question: []dns.Question{{Name: "media.licdn.com.", Qtype: 1, Qclass: 1}},
	Answer: []dns.RR{
        &dns.CNAME{Hdr: dns.RR_Header{Name: "media.licdn.com.", Rrtype: 5, Class: 1, Ttl: 184, Rdlength: 32}, Target: "2-01-2c3e-005c.cdx.cedexis.net."}, 
        &dns.CNAME{Hdr: dns.RR_Header{Name: "2-01-2c3e-005c.cdx.cedexis.net.", Rrtype: 5, Class: 1, Ttl: 213, Rdlength: 24}, Target: "od.linkedin.edgesuite.net."}, 
        &dns.CNAME{Hdr: dns.RR_Header{Name: "od.linkedin.edgesuite.net.", Rrtype: 5, Class: 1, Ttl: 3315, Rdlength: 21}, Target: "a1916.dscg2.akamai.net."}, 
        &dns.A{Hdr: dns.RR_Header{Name: "a1916.dscg2.akamai.net.", Rrtype: 1, Class: 1, Ttl: 20, Rdlength: 4}, A: []uint8{206, 248, 168, 145}}, 
        &dns.A{Hdr: dns.RR_Header{Name: "a1916.dscg2.akamai.net.", Rrtype: 1, Class: 1, Ttl: 20, Rdlength: 4}, A: []uint8{206, 248, 168, 162}}}, Extra: []dns.RR{&dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: 41, Class: 512}}
    }
}

```
## BPF Map Periodic Cleanup

When nethadone runs for more than a couple of days, an accumulation
of (saddr, daddr) pairs begin to slow things down though only
a relatively small number of IPs within a 10-20m window are really 
needed. A restart fixes the issue but this should be handled.

## 

## Other devices

### Orange PI R1

While a lower-end device, it has 2 100MBit ethernet ports plus more 
functional wifi and could be a cheaper and easier alternative than 
a R1+, especially for testing and trial purposes.

The sunxi-current kernel provided by [daeuniverse](https://github.com/daeuniverse/armbian-btf-kernel) doesn't seem to have 
`CONFIG_BPFFILTER` enabled, which prevents the BPF programs from 
being added.

This issue has more details on the error: https://github.com/armbian/build/pull/2277/files


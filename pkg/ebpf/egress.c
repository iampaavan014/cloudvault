// +build ignore

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>

struct egress_stats {
    __u64 bytes;
    __u64 packets;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32); // Destination IP
    __type(value, struct egress_stats);
    __uint(max_entries, 10240);
} egress_map SEC(".maps");

SEC("socket")
int count_egress(struct __sk_buff *skb) {
    // Only interested in egress
    // In a socket filter, we can't easily tell ingress vs egress 
    // without more context, but we'll assume this is attached to a 
    // container's veth egress or similar.
    
    struct egress_stats *stats;
    __u32 dest_ip;
    
    // Read IP header
    // Offset for ETH_HLEN (14)
    // Dest IP is at 16 bytes into IP header
    // Total offset = 14 + 16 = 30
    bpf_skb_load_bytes(skb, 30, &dest_ip, 4);

    stats = bpf_map_lookup_elem(&egress_map, &dest_ip);
    if (stats) {
        __sync_fetch_and_add(&stats->bytes, skb->len);
        __sync_fetch_and_add(&stats->packets, 1);
    } else {
        struct egress_stats new_stats = {skb->len, 1};
        bpf_map_update_elem(&egress_map, &dest_ip, &new_stats, BPF_ANY);
    }

    return -1; // Pass packet through
}

char _license[] SEC("license") = "GPL";

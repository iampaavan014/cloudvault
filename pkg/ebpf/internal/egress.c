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

struct egress_key {
    __u32 src_ip;
    __u32 dst_ip;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct egress_key);
    __type(value, struct egress_stats);
    __uint(max_entries, 65536);
} egress_map SEC(".maps");

SEC("socket")
int count_egress(struct __sk_buff *skb) {
    struct egress_key key = {};
    if (bpf_skb_load_bytes(skb, 26, &key.src_ip, 4) < 0)
        return 0;
    if (bpf_skb_load_bytes(skb, 30, &key.dst_ip, 4) < 0)
        return 0;

    struct egress_stats *stats;
    stats = bpf_map_lookup_elem(&egress_map, &key);
    if (stats) {
        __sync_fetch_and_add(&stats->bytes, skb->len);
        __sync_fetch_and_add(&stats->packets, 1);
    } else {
        struct egress_stats new_stats = {skb->len, 1};
        bpf_map_update_elem(&egress_map, &key, &new_stats, BPF_ANY);
    }

    return -1;
}

char _license[] SEC("license") = "GPL";

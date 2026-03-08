package ebpfgen

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpfeb,bpfel Egress egress.c

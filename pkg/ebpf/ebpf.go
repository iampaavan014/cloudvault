//go:build linux

package ebpf

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target bpf egress egress.c -- -I../headers

import (
	"fmt"
	"net"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
)

// Agent handles the lifecycle of the eBPF egress monitor.
// NOTE: The 'egressObjects' and 'loadEgressObjects' symbols are generated
// by 'bpf2go' (go:generate) and will be undefined until 'make generate'
// is run on a Linux machine with clang/llvm/libbpf installed.
type Agent struct {
	objs egressObjects
}

// NewAgent initializes and loads the eBPF programs
func NewAgent() (*Agent, error) {
	// Allow the current process to lock memory for eBPF resources
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, fmt.Errorf("failed to remove memlock: %w", err)
	}

	// Load pre-compiled programs and maps into the kernel
	var objs egressObjects
	if err := loadEgressObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("failed to load objects: %w", err)
	}

	return &Agent{objs: objs}, nil
}

// Close cleans up eBPF resources
func (a *Agent) Close() error {
	return a.objs.Close()
}

// GetEgressStats retrieves the latest stats from the eBPF map
func (a *Agent) GetEgressStats() (map[string]uint64, error) {
	stats := make(map[string]uint64)
	var key uint32
	var val egressEgressStats

	iter := a.objs.EgressMap.Iterate()
	for iter.Next(&key, &val) {
		ip := make(net.IP, 4)
		ip[0] = byte(key)
		ip[1] = byte(key >> 8)
		ip[2] = byte(key >> 16)
		ip[3] = byte(key >> 24)
		stats[ip.String()] = val.Bytes
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate map: %w", err)
	}

	return stats, nil
}

// AttachToInterface attaches the socket filter to a network interface
func (a *Agent) AttachToInterface(ifaceName string) (link.Link, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", ifaceName, err)
	}

	l, err := AttachSocket(SocketOptions{
		Program: a.objs.EgressFilter,
		Target:  iface.Index,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach socket: %w", err)
	}

	return l, nil
}

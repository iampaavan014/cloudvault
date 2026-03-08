//go:build linux

package ebpf

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
	ebpfgen "github.com/cloudvault-io/cloudvault/pkg/ebpf/internal"
)

// Agent handles the lifecycle of the eBPF egress monitor.
type Agent struct {
	objs ebpfgen.EgressObjects
}

// NewAgent initializes and loads the eBPF programs
func NewAgent() (*Agent, error) {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  memlock rlimit removal failed (ok on kernel ≥5.11): %v\n", err)
	}

	// Load pre-compiled programs and maps into the kernel
	var objs ebpfgen.EgressObjects
	if err := ebpfgen.LoadEgressObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("failed to load objects: %w", err)
	}

	return &Agent{objs: objs}, nil
}

// SeedSelfTest inserts a dummy entry into the map to verify the aggregation pipeline
func (a *Agent) SeedSelfTest() error {
	if a == nil || a.objs.EgressMap == nil {
		return fmt.Errorf("map not initialized")
	}

	key := ebpfgen.EgressEgressKey{
		SrcIp: binary.BigEndian.Uint32(net.ParseIP("127.0.0.1").To4()),
		DstIp: binary.BigEndian.Uint32(net.ParseIP("8.8.8.8").To4()),
	}
	val := ebpfgen.EgressEgressStats{
		Bytes:   1024,
		Packets: 1,
	}

	return a.objs.EgressMap.Update(&key, &val, ebpf.UpdateAny)
}

func (a *Agent) Close() error {
	if a == nil {
		return nil
	}
	return a.objs.Close()
}

// GetEgressStats retrieves the latest stats from the eBPF map
func (a *Agent) GetEgressStats() (map[string]map[string]uint64, error) {
	if a == nil || a.objs.EgressMap == nil {
		return nil, fmt.Errorf("eBPF map not initialized")
	}

	stats := make(map[string]map[string]uint64)
	var key ebpfgen.EgressEgressKey
	var val ebpfgen.EgressEgressStats

	iter := a.objs.EgressMap.Iterate()
	for iter.Next(&key, &val) {
		srcIP := intToIP(key.SrcIp).String()
		dstIP := intToIP(key.DstIp).String()

		if _, ok := stats[srcIP]; !ok {
			stats[srcIP] = make(map[string]uint64)
		}
		stats[srcIP][dstIP] = val.Bytes
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate map: %w", err)
	}

	return stats, nil
}

func intToIP(val uint32) net.IP {
	ip := make(net.IP, 4)
	// eBPF map stored it exactly as bytes from the header, meaning on x86 little-endian
	// reading it out needs LittleEndian to preserve the original 1st-to-4th byte ordering
	binary.LittleEndian.PutUint32(ip, val)
	return ip
}

// AttachToInterface attaches the socket filter to a network interface
func (a *Agent) AttachToInterface(ifaceName string) (io.Closer, error) {
	if _, err := os.Stat("/sys/fs/bpf"); os.IsNotExist(err) {
		return nil, fmt.Errorf("eBPF not supported: /sys/fs/bpf not found")
	}

	// Robustness check for nil receiver or stub mode
	if a == nil || a.objs.CountEgress == nil {
		return nil, fmt.Errorf("eBPF filter not loaded (nil agent or stub mode)")
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", ifaceName, err)
	}

	l, err := ebpfgen.AttachSocket(ebpfgen.SocketOptions{
		Program: a.objs.CountEgress,
		Target:  iface.Index,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach socket: %w", err)
	}

	return l, nil
}

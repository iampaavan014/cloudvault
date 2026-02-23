//go:build linux

package ebpf

import (
	"fmt"
	"io"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

// socketLink implements the link.Link interface for a socket filter
type socketLink struct {
	fd int
}

func (l *socketLink) Close() error {
	// Note: Closing the link doesn't necessarily stop the filter,
	// but for our purposes, we manage the lifecycle via Agent
	return unix.Close(l.fd)
}

func (l *socketLink) Detach() error {
	return l.Close()
}

func (l *socketLink) Update(_ *ebpf.Program) error {
	return fmt.Errorf("update not supported for socket link")
}

func (l *socketLink) Pin(_ string) error {
	return fmt.Errorf("pinning not supported for socket link")
}

func (l *socketLink) Unpin() error {
	return fmt.Errorf("unpinning not supported for socket link")
}

func (l *socketLink) Info() (*link.Info, error) {
	return nil, fmt.Errorf("info not supported for socket link")
}

// AttachSocket attaches a BPF program to a socket for a specific interface index.
// This is used by our Agent to hook 'count_egress' into networking.
func AttachSocket(opts SocketOptions) (io.Closer, error) {
	if opts.Program == nil {
		return nil, fmt.Errorf("nil program")
	}

	// Create a RAW socket to attach the filter to
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(unix.ETH_P_ALL))
	if err != nil {
		return nil, fmt.Errorf("failed to create raw socket: %w", err)
	}

	// Bind to the specific interface
	sll := &unix.SockaddrLinklayer{
		Ifindex:  opts.Target,
		Protocol: uint16(unix.ETH_P_ALL),
	}
	if err := unix.Bind(fd, sll); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to bind to interface index %d: %w", opts.Target, err)
	}

	// Attach the BPF program
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ATTACH_BPF, opts.Program.FD()); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to attach BPF program: %w", err)
	}

	return &socketLink{fd: fd}, nil
}

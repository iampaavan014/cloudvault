//go:build linux

package ebpfgen

import (
	"fmt"
	"io"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

// SocketOptions specifies parameters for attaching a BPF program to a socket
type SocketOptions struct {
	Program *ebpf.Program
	Target  int
}

// socketLink implements the link.Link interface for a socket filter
type socketLink struct {
	fd int
}

func (l *socketLink) Close() error {
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
func AttachSocket(opts SocketOptions) (io.Closer, error) {
	if opts.Program == nil {
		return nil, fmt.Errorf("nil program")
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(unix.ETH_P_ALL))
	if err != nil {
		return nil, fmt.Errorf("failed to create raw socket: %w", err)
	}

	// htons converts host byte order to network byte order
	var htons = func(i uint16) uint16 {
		return (i << 8) | (i >> 8)
	}

	sll := &unix.SockaddrLinklayer{
		Ifindex:  opts.Target,
		Protocol: htons(unix.ETH_P_ALL),
	}
	if err := unix.Bind(fd, sll); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to bind to interface index %d: %w", opts.Target, err)
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ATTACH_BPF, opts.Program.FD()); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to attach BPF program: %w", err)
	}

	return &socketLink{fd: fd}, nil
}

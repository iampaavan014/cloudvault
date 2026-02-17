//go:build !linux

package ebpf

import (
	"errors"
)

// Agent handles the lifecycle of the eBPF egress monitor (Mock)
type Agent struct{}

func NewAgent() (*Agent, error) {
	return &Agent{}, nil
}

func (a *Agent) Close() error {
	return nil
}

func (a *Agent) GetEgressStats() (map[string]uint64, error) {
	return map[string]uint64{
		"10.0.1.5": 157286400, // Simulated data
	}, nil
}

func (a *Agent) AttachToInterface(ifaceName string) (interface{}, error) {
	return nil, errors.New("eBPF not supported on this platform")
}

// Dummy types to satisfy Linux build components if they are not guarded
type egressObjects struct {
	EgressMap interface {
		Iterate() interface {
			Next(interface{}, interface{}) bool
			Err() error
		}
		Close() error
	}
}

func (o *egressObjects) Close() error { return nil }

type egressEgressStats struct {
	Bytes uint64
}

func loadEgressObjects(obj interface{}, opts interface{}) error {
	return nil
}

// Code generated by bpf2go; DO NOT EDIT.
//go:build mips || mips64 || ppc64 || s390x

package handlers

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type dnspktPayloadT struct {
	Len  uint32
	Data [508]uint8
}

// loadDnspkt returns the embedded CollectionSpec for dnspkt.
func loadDnspkt() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_DnspktBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load dnspkt: %w", err)
	}

	return spec, err
}

// loadDnspktObjects loads dnspkt and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*dnspktObjects
//	*dnspktPrograms
//	*dnspktMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func loadDnspktObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := loadDnspkt()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// dnspktSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type dnspktSpecs struct {
	dnspktProgramSpecs
	dnspktMapSpecs
}

// dnspktSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type dnspktProgramSpecs struct {
	UdpDnsSniff *ebpf.ProgramSpec `ebpf:"udp_dns_sniff"`
}

// dnspktMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type dnspktMapSpecs struct {
	DnsArr *ebpf.MapSpec `ebpf:"dns_arr"`
	TmpMap *ebpf.MapSpec `ebpf:"tmp_map"`
}

// dnspktObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to loadDnspktObjects or ebpf.CollectionSpec.LoadAndAssign.
type dnspktObjects struct {
	dnspktPrograms
	dnspktMaps
}

func (o *dnspktObjects) Close() error {
	return _DnspktClose(
		&o.dnspktPrograms,
		&o.dnspktMaps,
	)
}

// dnspktMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to loadDnspktObjects or ebpf.CollectionSpec.LoadAndAssign.
type dnspktMaps struct {
	DnsArr *ebpf.Map `ebpf:"dns_arr"`
	TmpMap *ebpf.Map `ebpf:"tmp_map"`
}

func (m *dnspktMaps) Close() error {
	return _DnspktClose(
		m.DnsArr,
		m.TmpMap,
	)
}

// dnspktPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to loadDnspktObjects or ebpf.CollectionSpec.LoadAndAssign.
type dnspktPrograms struct {
	UdpDnsSniff *ebpf.Program `ebpf:"udp_dns_sniff"`
}

func (p *dnspktPrograms) Close() error {
	return _DnspktClose(
		p.UdpDnsSniff,
	)
}

func _DnspktClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed dnspkt_bpfeb.o
var _DnspktBytes []byte

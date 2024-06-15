package handlers

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/alecthomas/repr"
	"github.com/florianl/go-tc"
)

// Not really test cases but testing grounds

func TestEbpfRebuild(t *testing.T) {
	// Testing

	fmt.Println(os.Getwd())
	tplfile := "../ebpf/throttle.bpf.c.tpl"
	cfile := "../ebpf/throttle.bpf.c"

	fparams := make([]FiltParams, 0)
	fparams = append(fparams, FiltParams{
		SrcIpAddr:  "127,0,0,1",
		DestIpAddr: "10,0,0,1",
		DelayMs:    10,
	})
	rebuildBpf(tplfile, cfile, &fparams)
}

func TestTcExisting(t *testing.T) {

	tcnl := getTcnl()
	iface, err := net.InterfaceByName("eth0")
	if err != nil {
		t.Fatal(err)
	}

	msg := tc.Msg{Ifindex: uint32(iface.Index), Parent: 4294967283, Info: 768}
	tfilt, err := tcnl.Filter().Get(&msg)
	if err != nil {
		t.Fatal(err)
	}
	for _, tobj := range tfilt {
		if tobj.BPF != nil && tobj.BPF.Name != nil && *tobj.BPF.Name == "throttle" {
			repr.Println(tobj)
			tcnl.Filter().Delete(&tobj)
		}
	}

}

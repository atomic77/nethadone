package views

import "testing"

func TestFmtBytes(t *testing.T) {
	for n, want := range map[uint64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KiB", 5 << 30: "5.0 GiB"} {
		if got := fmtBytes(n); got != want {
			t.Errorf("fmtBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFmtBits(t *testing.T) {
	for n, want := range map[float64]string{0: "0 bit/s", 999: "999 bit/s", 50e3: "50 kbit/s", 1.5e6: "1.5 Mbit/s", 2e9: "2 Gbit/s"} {
		if got := fmtBits(n); got != want {
			t.Errorf("fmtBits(%v) = %q, want %q", n, got, want)
		}
	}
}

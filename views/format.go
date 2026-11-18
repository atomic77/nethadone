package views

import "fmt"

// fmtBytes renders n with a binary unit, e.g. 1536 -> "1.5 KiB".
func fmtBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// htmxConfig lets a 422 (form validation error) swap in like a 200.
const htmxConfig = `{"responseHandling":[{"code":"204","swap":false},{"code":"422","swap":true},{"code":"[23]..","swap":true},{"code":"[45]..","swap":false,"error":true}]}`

// fmtBits renders a rate in bits/s with an SI unit, e.g. 1500000 -> "1.5 Mbit/s".
func fmtBits(bps float64) string {
	for _, u := range []string{"bit/s", "kbit/s", "Mbit/s"} {
		if bps < 1000 {
			return fmt.Sprintf("%.3g %s", bps, u)
		}
		bps /= 1000
	}
	return fmt.Sprintf("%.3g Gbit/s", bps)
}

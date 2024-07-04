package handlers

var BaseConfig = Config{
	WanInterface: "eth0",
	LanInterface: "lan0",
	WanMbps:      100,
	// Start degrading from 1mbit down to 10kbits, over 5 bandwidth classes
	NumQdiscClasses: 5,
	StartRateKbs:    1000,
	MaxDelayMs:      50,
	MinRateKbs:      50,
}

type Config struct {
	WanInterface string
	LanInterface string
	// Parameters for the htb / netem qdiscs and classes we'll create on the fly
	// WanMbs represents the max bandwidth of your downstream internet connection
	// The MinRateKbs and MaxDelayMs represent the maximum bandwidth degradation ;
	// classes will be created linearly between them based on `NumQdiscClasses`,
	// and the Start and Min RateKbs
	NumQdiscClasses int
	WanMbps         int
	StartRateKbs    int
	MaxDelayMs      int
	MinRateKbs      int
}

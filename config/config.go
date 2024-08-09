package config

import (
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

var baseConfig = Config{
	WanInterface: "eth0",
	LanInterface: "lan0",
	WanMbps:      100,

	CfgDb:         "/root/cfg.db",
	DnsDb:         "/root/dns.db",
	PrometheusUrl: "http://localhost:9090",

	MapCollectionInterval: time.Second * 30,
	PolicyCheckInterval:   time.Minute * 1,

	// Start degrading from 1mbit down to 10kbits, over 5 bandwidth classes
	NumQdiscClasses: 5,
	StartRateKbs:    1000,
	MaxDelayMs:      50,
	MinRateKbs:      50,

	ActivityThresholdBytes: 100,
	PolicyBackoffInterval:  time.Minute * 3,
}

var Cfg = baseConfig

type Config struct {
	// The WAN interface should have access to the internet
	// The LAN interface is where your local devices
	WanInterface string `yaml:"wan_interface"`
	LanInterface string `yaml:"lan_interface"`

	// Database locations
	CfgDb         string `yaml:"cfg_db"`
	DnsDb         string `yaml:"dns_db"`
	PrometheusUrl string `yaml:"prometheus_url"`

	// Operational parameters for tuning use on SBCs with different
	// capabilities
	MapCollectionInterval time.Duration `yaml:"map_collection_interval"`
	PolicyCheckInterval   time.Duration `yaml:"policy_check_interval"`

	// Parameters for the htb / netem qdiscs and classes we'll create on the fly
	// WanMbs represents the max bandwidth of your downstream internet connection
	// The MinRateKbs and MaxDelayMs represent the maximum bandwidth degradation ;
	// classes will be created linearly between them based on `NumQdiscClasses`,
	// and the Start and Min RateKbs
	NumQdiscClasses int `yaml:"num_qdisc_classes"`
	WanMbps         int `yaml:"wan_mbps"`
	StartRateKbs    int `yaml:"start_rate_kbs"`
	MaxDelayMs      int `yaml:"max_delay_ms"`
	MinRateKbs      int `yaml:"min_rate_kbs"`

	////////////////////
	// Parameters for the Simple Load average policy
	// See policy/README.md for details

	// Activity threshold is represented as `k` in policy readme,
	// the number of bytes considered "active" for the prom rate
	// queries
	ActivityThresholdBytes int `yaml:"activity_threshold_bytes"`
	// How long before we'll consider a change to policy;
	// setting this lower will cause clients to be moved up and
	// down bandwidth classes faster
	PolicyBackoffInterval time.Duration `yaml:"policy_backoff_interval"`
}

func ParseConfig(file string) {
	f, err := os.Open(file)
	if err != nil {
		log.Println("configuration file could not be parsed; proceeding with defaults")
		return
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&Cfg)
	if err != nil {
		log.Fatalln("failed to parse configuration file ", err)
	}
}

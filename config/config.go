package config

import (
	"io"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

var baseConfig = Config{
	// Empty means "the interface holding the default route", resolved at startup
	WanInterface: "",
	LanInterface: "lan0",
	WanMbps:      100,

	CfgDb: "/var/lib/nethadone/cfg.db",
	DnsDb: "/var/lib/nethadone/dns.db",

	MapCollectionInterval: time.Second * 30,
	PolicyCheckInterval:   time.Minute * 1,

	// Start degrading from 1mbit down to 10kbits, over 5 bandwidth classes
	NumQdiscClasses: 5,
	StartRateKbs:    1000,
	MaxDelayMs:      50,
	MinRateKbs:      50,

	ActivityThresholdBytes: 100,
	PolicyBackoffInterval:  time.Minute * 3,
	PolicyTimeUnit:         time.Minute,
}

var Cfg = baseConfig

type Config struct {
	// The WAN interface should have access to the internet
	// The LAN interface is where your local devices
	WanInterface string `yaml:"wan_interface"`
	LanInterface string `yaml:"lan_interface"`

	// Database locations
	CfgDb string `yaml:"cfg_db"`
	DnsDb string `yaml:"dns_db"`

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
	// the bytes per second over which a client counts as "active"
	// on a glob group
	ActivityThresholdBytes int `yaml:"activity_threshold_bytes"`
	// How long before we'll consider a change to policy;
	// setting this lower will cause clients to be moved up and
	// down bandwidth classes faster
	PolicyBackoffInterval time.Duration `yaml:"policy_backoff_interval"`
	// Unit of the policy's rate windows (2 units now, 2 units ending 5 units
	// ago, 5 units now). Only shorten it for testing.
	PolicyTimeUnit time.Duration `yaml:"policy_time_unit"`
}

// DefaultFile is where install.sh puts the config and the systemd unit reads it.
const DefaultFile = "/etc/nethadone/nethadone.yml"

// ParseConfig loads file over the defaults. A missing file is fatal when
// required (the user named it), otherwise nethadone proceeds with defaults.
func ParseConfig(file string, required bool) {
	f, err := os.Open(file)
	if err != nil {
		if required {
			log.Fatalln("could not open configuration file: ", err)
		}
		log.Println("no configuration file at ", file, "; proceeding with defaults")
		return
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	// Catch typos in keys instead of silently ignoring them
	decoder.KnownFields(true)
	err = decoder.Decode(&Cfg)
	if err != nil && err != io.EOF {
		log.Fatalln("failed to parse configuration file ", file, ": ", err)
	}
}

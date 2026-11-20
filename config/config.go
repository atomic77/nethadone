package config

import (
	"errors"
	"io"
	"log"
	"os"
	"sync"
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

	DhcpLeasesFile: "/var/lib/NetworkManager/dnsmasq-*.leases",

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
	ThrottlePerDevice:      true,
}

var Cfg = baseConfig

// Loaded is the config file that was read, empty when running on defaults.
var Loaded string

// fileTunables are the tunables from defaults + file, before any web UI override.
var fileTunables = Tunables{baseConfig.ActivityThresholdBytes, baseConfig.PolicyBackoffInterval}

// mu guards Cfg once the web server runs: SetTunables writes while the policy loop reads.
var mu sync.RWMutex

// Tunables are the settings the web UI can change while running. Overrides are
// stored in cfg_db (database.SaveTunables); everything else comes from the file only.
type Tunables struct {
	ActivityThresholdBytes int
	PolicyBackoffInterval  time.Duration
}

func (t Tunables) Validate() error {
	if t.ActivityThresholdBytes < 0 || t.ActivityThresholdBytes > 100_000_000 {
		return errors.New("activity threshold must be between 0 and 100000000 bytes/s")
	}
	if t.PolicyBackoffInterval < time.Second || t.PolicyBackoffInterval > 24*time.Hour {
		return errors.New("backoff interval must be between 1s and 24h")
	}
	return nil
}

// Current returns a copy of Cfg that is safe to read while SetTunables may run.
func Current() Config {
	mu.RLock()
	defer mu.RUnlock()
	return Cfg
}

func GetTunables() Tunables {
	mu.RLock()
	defer mu.RUnlock()
	return Tunables{Cfg.ActivityThresholdBytes, Cfg.PolicyBackoffInterval}
}

func SetTunables(t Tunables) error {
	if err := t.Validate(); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	Cfg.ActivityThresholdBytes, Cfg.PolicyBackoffInterval = t.ActivityThresholdBytes, t.PolicyBackoffInterval
	return nil
}

// FileTunables are the tunables as set by the defaults and the config file.
func FileTunables() Tunables { return fileTunables }

type Config struct {
	// The WAN interface should have access to the internet
	// The LAN interface is where your local devices
	WanInterface string `yaml:"wan_interface"`
	LanInterface string `yaml:"lan_interface"`

	// Database locations
	CfgDb string `yaml:"cfg_db"`
	DnsDb string `yaml:"dns_db"`

	// dnsmasq leases file(s), a glob, read for client hostnames on /clients. Empty disables.
	DhcpLeasesFile string `yaml:"dhcp_leases_file"`

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
	// true: each LAN device is measured and throttled on its own (several people).
	// false: usage of all devices is summed per glob group and all are throttled
	// together (one person with a phone and a laptop).
	ThrottlePerDevice bool `yaml:"throttle_per_device"`
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
	Loaded = file
	fileTunables = Tunables{Cfg.ActivityThresholdBytes, Cfg.PolicyBackoffInterval}
}

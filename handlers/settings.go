package handlers

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/models"
	"github.com/atomic77/nethadone/views"
)

// paramHelp explains every config.Config field, keyed by YAML name (TestParamHelpComplete checks it).
var paramHelp = map[string]string{
	"activity_threshold_bytes": "How strict. A device counts as actively using a glob group when it receives more than this many bytes per second from it, in all three usage windows: the last 2 and 5 time units, and the 2 units ending 5 units ago (a unit is policy_time_unit, 1 minute by default). Lower values start throttling sooner and catch lighter use.",
	"policy_backoff_interval":  "How fast. The minimum time between moving a device one step slower (or one step faster once it stops). Shorter reacts quicker; longer is gentler. Policies are only checked every policy_check_interval, so shorter values act like that interval.",

	"wan_interface":           "Network interface connected to the internet. DNS answers are read here to learn which IPs belong to which sites. Left empty in the file, the interface with the default route is used.",
	"lan_interface":           "Network interface your home devices are on. Traffic is counted and slowed down here.",
	"cfg_db":                  "SQLite database holding glob groups, throttling policies and the settings changed on this page.",
	"dns_db":                  "SQLite database of the domain names seen in DNS answers and their IP addresses. Entries older than about 5 days are removed.",
	"dhcp_leases_file":        "dnsmasq lease file(s), a glob, read to show device hostnames on the Clients page. Empty turns this off.",
	"map_collection_interval": "How often per-device traffic counters are read from the kernel. Shorter gives fresher Bandwidth data and faster reactions, but costs CPU on small boards.",
	"policy_check_interval":   "How often nethadone decides whether each device should be slowed down or sped up for each glob group.",
	"num_qdisc_classes":       "How many throttling steps exist between full speed and the slowest step. A device moves one step per policy change. Keep it below 9.",
	"wan_mbps":                "Your internet download speed in Mbit/s. Traffic that is not throttled may use up to this rate.",
	"start_rate_kbs":          "Bandwidth of the first (mildest) throttling step, in kbit/s.",
	"min_rate_kbs":            "Bandwidth of the slowest throttling step, in kbit/s. Steps in between are spaced evenly from start_rate_kbs down to this.",
	"max_delay_ms":            "Extra delay added to every packet at the slowest step, in milliseconds. Milder steps get a proportional share. Delay makes video and scrolling feel sluggish without blocking anything.",
	"throttle_per_device":     "true: each device is measured and throttled on its own, for several people sharing the router. false: the traffic of all devices is added up per glob group and every device is slowed down together (policies show the device as \"all\"), for one person with a phone and a laptop. The activity threshold then applies to the combined traffic.",
	"policy_time_unit":        "Length of the time unit behind the usage windows above (2 and 5 units) and of the usage history kept in memory (10 units). Only shorten it for testing.",
}

// fmtDuration drops zero tails, e.g. 3m0s -> 3m, 1h0m0s -> 1h.
func fmtDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

// fileParams lists every config field except the tunables, in struct order.
func fileParams(c config.Config) []models.Param {
	var out []models.Param
	v := reflect.ValueOf(c)
	for i := range v.NumField() {
		name := strings.Split(v.Type().Field(i).Tag.Get("yaml"), ",")[0]
		if name == "activity_threshold_bytes" || name == "policy_backoff_interval" {
			continue
		}
		val := fmt.Sprint(v.Field(i).Interface())
		if d, ok := v.Field(i).Interface().(time.Duration); ok {
			val = fmtDuration(d)
		}
		out = append(out, models.Param{Name: name, Value: val, Help: paramHelp[name]})
	}
	return out
}

// tunableParams pairs the form values with the config file's, for the live settings form.
func tunableParams(threshold, backoff string) []models.Param {
	file := config.FileTunables()
	p := []models.Param{
		{Name: "activity_threshold_bytes", Value: threshold, File: strconv.Itoa(file.ActivityThresholdBytes)},
		{Name: "policy_backoff_interval", Value: backoff, File: fmtDuration(file.PolicyBackoffInterval)},
	}
	for i := range p {
		p[i].Help = paramHelp[p[i].Name]
		if p[i].File == p[i].Value {
			p[i].File = ""
		}
	}
	return p
}

func renderSettings(w http.ResponseWriter, r *http.Request, threshold, backoff, errMsg string) {
	render(w, r, views.Settings(tunableParams(threshold, backoff), fileParams(config.Current()), config.Loaded, errMsg))
}

func Settings(w http.ResponseWriter, r *http.Request) {
	t := config.GetTunables()
	renderSettings(w, r, strconv.Itoa(t.ActivityThresholdBytes), fmtDuration(t.PolicyBackoffInterval), "")
}

// SettingsSave stores and applies the live tunables. They take effect at the next policy check.
func SettingsSave(w http.ResponseWriter, r *http.Request) {
	threshold := strings.TrimSpace(r.FormValue("activity_threshold_bytes"))
	backoff := strings.TrimSpace(r.FormValue("policy_backoff_interval"))
	var t config.Tunables
	var err error
	if t.ActivityThresholdBytes, err = strconv.Atoi(threshold); err != nil {
		err = fmt.Errorf("activity threshold %q is not a whole number", threshold)
	} else if t.PolicyBackoffInterval, err = time.ParseDuration(backoff); err != nil {
		err = fmt.Errorf("backoff interval %q is not a duration like 90s, 3m or 1h", backoff)
	} else {
		err = saveTunables(t)
	}
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		renderSettings(w, r, threshold, backoff, err.Error())
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// SettingsReset drops the web UI overrides and goes back to the config file's values.
func SettingsReset(w http.ResponseWriter, r *http.Request) {
	if err := saveTunables(config.FileTunables()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// saveTunables validates, stores, then applies, so a failed write never leaves a live value that a restart would lose.
func saveTunables(t config.Tunables) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := database.SaveTunables(t); err != nil {
		log.Println("could not save settings: ", err)
		return fmt.Errorf("could not save settings")
	}
	log.Printf("settings changed in web UI: %+v", t)
	return config.SetTunables(t)
}

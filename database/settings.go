package database

import (
	"log"
	"strconv"
	"time"

	"github.com/atomic77/nethadone/config"
)

// Rows in the setting table override the config file for config.Tunables, keyed by YAML name.
// Only values that differ from the file are stored, so editing the file still works for the rest.

// LoadTunables applies stored overrides; call it after Connect. Bad rows are logged and ignored.
func LoadTunables() {
	rows := []struct{ Name, Value string }{}
	if err := cfgDb.Select(&rows, "SELECT name, value FROM setting"); err != nil {
		log.Println("could not read settings: ", err)
		return
	}
	t := config.GetTunables()
	for _, r := range rows {
		var err error
		switch r.Name {
		case "activity_threshold_bytes":
			var n int
			if n, err = strconv.Atoi(r.Value); err == nil {
				t.ActivityThresholdBytes = n
			}
		case "policy_backoff_interval":
			var d time.Duration
			if d, err = time.ParseDuration(r.Value); err == nil {
				t.PolicyBackoffInterval = d
			}
		default:
			continue
		}
		if err != nil {
			log.Println("ignoring stored setting ", r.Name, ": ", err)
			continue
		}
		log.Println("web UI setting overrides config file: ", r.Name, " = ", r.Value)
	}
	if err := config.SetTunables(t); err != nil {
		log.Println("ignoring stored settings: ", err)
	}
}

// SaveTunables stores the fields of t that differ from the config file and deletes the rest.
func SaveTunables(t config.Tunables) error {
	file := config.FileTunables()
	tx, err := cfgDb.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit
	for _, f := range []struct {
		name, value string
		fromFile    bool
	}{
		{"activity_threshold_bytes", strconv.Itoa(t.ActivityThresholdBytes), t.ActivityThresholdBytes == file.ActivityThresholdBytes},
		{"policy_backoff_interval", t.PolicyBackoffInterval.String(), t.PolicyBackoffInterval == file.PolicyBackoffInterval},
	} {
		if f.fromFile {
			_, err = tx.Exec("DELETE FROM setting WHERE name = $1", f.name)
		} else {
			_, err = tx.Exec("INSERT INTO setting VALUES ($1, $2) ON CONFLICT (name) DO UPDATE SET value = excluded.value", f.name, f.value)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

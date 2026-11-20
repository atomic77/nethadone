package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
)

func TestParamHelpComplete(t *testing.T) {
	typ := reflect.TypeOf(config.Config{})
	for i := range typ.NumField() {
		if name := typ.Field(i).Tag.Get("yaml"); paramHelp[name] == "" {
			t.Errorf("no help text for %s on the settings page", name)
		}
	}
	if n := len(fileParams(config.Config{})); n != typ.NumField()-2 {
		t.Errorf("read-only list has %d settings, want all but the 2 tunables", n)
	}
}

func TestFmtDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "0s", 30 * time.Second: "30s", 3 * time.Minute: "3m",
		time.Hour: "1h", 90 * time.Minute: "1h30m", 61 * time.Second: "1m1s"} {
		if got := fmtDuration(d); got != want {
			t.Errorf("fmtDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

// Save, reject, survive a restart, reset: the whole life of a web UI override.
func TestSettingsForm(t *testing.T) {
	connectTempDb(t)
	file := config.FileTunables()
	t.Cleanup(func() { config.SetTunables(file) })
	do := func(method, path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		map[string]http.HandlerFunc{"GET /settings": Settings, "POST /settings": SettingsSave, "POST /settings/reset": SettingsReset}[method+" "+path](rec, req)
		return rec
	}
	form := func(k, b string) url.Values {
		return url.Values{"activity_threshold_bytes": {k}, "policy_backoff_interval": {b}}
	}

	for _, bad := range []url.Values{form("abc", "3m"), form("-1", "3m"), form("100", "soon"), form("100", "0s")} {
		if rec := do("POST", "/settings", bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `role="alert"`) {
			t.Errorf("%v: got %d, want 422 with an error", bad, rec.Code)
		}
	}
	if got := config.GetTunables(); got != file {
		t.Fatalf("rejected input changed tunables to %+v", got)
	}

	// Threshold equal to the file value is not an override; only the backoff is stored.
	want := config.Tunables{ActivityThresholdBytes: file.ActivityThresholdBytes, PolicyBackoffInterval: 90 * time.Second}
	if rec := do("POST", "/settings", form("100", "90s")); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: got %d: %s", rec.Code, rec.Body)
	}
	if got := config.GetTunables(); got != want {
		t.Errorf("after save got %+v, want %+v", got, want)
	}
	page := do("GET", "/settings", nil).Body.String()
	if !strings.Contains(page, "configuration file value of 3m") || strings.Contains(page, "configuration file value of 100") {
		t.Errorf("page should flag only the backoff as overridden")
	}
	if !strings.Contains(page, "lan_interface") {
		t.Errorf("page is missing the read-only settings")
	}

	config.SetTunables(file) // restart: the file values come back, then the stored overrides
	database.LoadTunables()
	if got := config.GetTunables(); got != want {
		t.Errorf("after restart got %+v, want %+v", got, want)
	}

	if rec := do("POST", "/settings/reset", nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("reset: got %d", rec.Code)
	}
	config.SetTunables(want)
	database.LoadTunables()
	if got := config.GetTunables(); got != want {
		t.Errorf("reset should leave no stored overrides; LoadTunables changed %+v to %+v", want, got)
	}
}

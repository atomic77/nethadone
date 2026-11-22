package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Only paths that leave the running (test) binary in place.
func TestDownloadUpdate(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("not an executable")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/same" {
			http.ServeFile(w, r, self)
			return
		}
		w.Write(body)
	}))
	defer srv.Close()
	defer func(u string) { releaseURL = u }(releaseURL)

	releaseURL = srv.URL + "/same"
	if _, changed, err := downloadUpdate(); err != nil || changed {
		t.Errorf("identical release: changed=%v err=%v", changed, err)
	}
	releaseURL = srv.URL + "/junk"
	if _, changed, err := downloadUpdate(); err == nil || changed {
		t.Errorf("broken release: changed=%v err=%v, want an error", changed, err)
	}
	if b, _ := os.ReadFile(self); string(b) == string(body) {
		t.Fatal("test binary was replaced")
	}
}

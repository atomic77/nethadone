package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
)

// Same asset install.sh downloads; the Makefile names builds after GOARCH.
var releaseURL = "https://github.com/atomic77/nethadone/releases/latest/download/nethadone-" + runtime.GOARCH + "-linux"

var updating sync.Mutex

// Update replaces the running binary with the latest GitHub release and re-execs it.
func Update(w http.ResponseWriter, r *http.Request) {
	if !updating.TryLock() {
		renderStatus(w, r, "An update is already running.")
		return
	}
	exe, changed, err := downloadUpdate()
	if err != nil {
		updating.Unlock()
		log.Println("update: ", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		renderStatus(w, r, "Update failed: "+err.Error())
		return
	}
	if !changed {
		updating.Unlock()
		renderStatus(w, r, "Already running the latest release.")
		return
	}
	log.Println("update: installed new binary, restarting")
	renderStatus(w, r, "Updated to the latest release. Restarting, reload this page in a few seconds.")
	go func() {
		time.Sleep(time.Second) // let the response reach the browser
		// exec keeps the PID, so this works with or without systemd; Initialize replaces the old qdiscs and filters
		log.Fatal("update: exec failed: ", syscall.Exec(exe, os.Args, os.Environ()))
	}()
}

// downloadUpdate fetches the latest release next to the running binary, checks that it
// runs here (`nethadone check` with the current flags) and renames it over the binary.
// changed is false when the release is byte-identical to the running binary.
func downloadUpdate() (exe string, changed bool, err error) {
	if exe, err = os.Executable(); err != nil {
		return "", false, err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", false, err
	}
	// Same directory, so the final rename is atomic
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".nethadone-update-*")
	if err != nil {
		return "", false, err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename
	defer tmp.Close()

	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(releaseURL)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("download %s: %s", releaseURL, resp.Status)
	}
	h := sha256.New()
	if _, err = io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		return "", false, fmt.Errorf("download: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return "", false, err
	}
	if same, err := sameHash(exe, h.Sum(nil)); err != nil || same {
		return "", false, err
	}
	if err = os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", false, err
	}
	if out, err := exec.Command(tmp.Name(), append([]string{"check"}, os.Args[1:]...)...).CombinedOutput(); err != nil {
		return "", false, fmt.Errorf("new release failed `nethadone check` (%v), keeping the current binary:\n%s", err, out)
	}
	if err = os.Rename(tmp.Name(), exe); err != nil {
		return "", false, err
	}
	return exe, true, nil
}

func sameHash(path string, sum []byte) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return string(h.Sum(nil)) == string(sum), nil
}

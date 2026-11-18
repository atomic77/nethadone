package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/handlers"
	"github.com/atomic77/nethadone/policy"
	"github.com/atomic77/nethadone/views"
)

func main() {

	// `nethadone check [flags]` runs the preflight checks and exits
	checkOnly := len(os.Args) > 1 && os.Args[1] == "check"
	if checkOnly {
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	wanIf := flag.String("wan-interface", "", "Interface connecting out to internet (default: the one with the default route)")
	lanIf := flag.String("lan-interface", "", "Interface connected to local network")
	configFile := flag.String("config-file", config.DefaultFile, "Configuration file")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [check] [flags]\n\n  check\tverify this device can run nethadone, change nothing\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	explicitConfig := false
	flag.Visit(func(f *flag.Flag) { explicitConfig = explicitConfig || f.Name == "config-file" })
	config.ParseConfig(*configFile, explicitConfig)
	// Command line parameters override anything that might be in the config
	if *lanIf != "" {
		config.Cfg.LanInterface = *lanIf
	}
	if *wanIf != "" {
		config.Cfg.WanInterface = *wanIf
	}
	if config.Cfg.WanInterface == "" {
		if wan, err := handlers.DefaultRouteInterface(); err == nil {
			log.Println("Detected WAN interface ", wan, " from the default route")
			config.Cfg.WanInterface = wan
		}
	}

	if checkOnly {
		log.SetOutput(io.Discard) // keep the checklist readable; compile logs are noise here
		if !handlers.Check(os.Stdout) {
			os.Exit(1)
		}
		return
	}

	log.Println("Configuration: ", repr.String(config.Cfg, repr.Indent("  ")))
	if !handlers.Check(log.Writer()) {
		log.Fatal("preflight checks failed, see above; run `nethadone check` after fixing")
	}
	database.Connect()
	handlers.Initialize()

	handlers.InitMetrics()
	policy.InitPolicy()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handlers.Index)
	mux.HandleFunc("GET /policies", handlers.Policies)
	mux.HandleFunc("GET /bandwidth", handlers.Bandwidth)
	mux.HandleFunc("GET /clients", handlers.Clients)
	mux.HandleFunc("GET /shaping", handlers.Shaping)
	mux.HandleFunc("GET /globs", handlers.Globs)
	mux.HandleFunc("POST /globs", handlers.GlobAdd)
	mux.HandleFunc("POST /globs/delete", handlers.GlobDelete)
	mux.HandleFunc("GET /metrics", handlers.MetricsHandleFunc)
	mux.Handle("GET /static/", http.FileServerFS(views.Static))

	// The UI has no login, so at least refuse cross-site form posts from other pages the user visits.
	log.Fatal(http.ListenAndServe(":3000", http.NewCrossOriginProtection().Handler(mux)))
}

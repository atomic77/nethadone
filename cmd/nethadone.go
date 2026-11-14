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
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
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

	// Use embedded templates
	engine := html.NewFileSystem(http.FS(views.EmbedTemplates), ".tpl")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Use(logger.New())

	app.Get("/", handlers.Index)
	app.Get("/interfaces", handlers.Interfaces)
	app.Get("/devices", handlers.Devices)
	app.Get("/policies", handlers.Policies)
	app.Get("/bandwidth", handlers.Bandwidth)
	app.Get("/globs", handlers.Globs)
	app.Post("/globs/add", handlers.GlobAdd)

	handlers.InitMetrics()
	policy.InitPolicy()

	app.Get("/metrics", adaptor.HTTPHandlerFunc(handlers.MetricsHandleFunc))

	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendFile("static/laptop.svg")
	})

	log.Fatal(app.Listen(":3000"))
}

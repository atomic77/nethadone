package main

import (
	"flag"
	"log"

	"github.com/alecthomas/repr"
	"github.com/atomic77/nethadone/config"
	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/handlers"
	"github.com/atomic77/nethadone/policy"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

func main() {

	wanIf := flag.String("wan-interface", "eth0", "Interface connecting out to internet")
	lanIf := flag.String("lan-interface", "eth1", "Interface connected to local network")
	configFile := flag.String("config-file", "nethadone.yml", "Configuration file")

	flag.Parse()

	config.ParseConfig(*configFile)
	// Command line parameters override anything that might be in the config
	config.Cfg.LanInterface = *lanIf
	config.Cfg.WanInterface = *wanIf

	log.Println("Configuration: ", repr.String(config.Cfg, repr.Indent("  ")))
	database.Connect()
	handlers.Initialize()

	// Pass the engine to the Views
	engine := html.New("./views", ".tpl")
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

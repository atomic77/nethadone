package main

import (
	"flag"
	"log"

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

	flag.Parse()

	// Pass the engine to the Views
	engine := html.New("./views", ".tpl")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	database.Connect()
	cfg := handlers.BaseConfig
	cfg.LanInterface = *lanIf
	cfg.WanInterface = *wanIf
	handlers.Initialize(&cfg)

	// Middleware
	// app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/", handlers.Index)
	app.Get("/interfaces", handlers.Interfaces)
	app.Get("/devices", handlers.Devices)
	app.Get("/rulesets", handlers.Rules)
	app.Post("/rulesets/change", handlers.RuleChange)
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

package main

import (
	"log"

	"github.com/atomic77/nethadone/database"
	"github.com/atomic77/nethadone/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
)

func main() {

	// ifname := flag.String("interface", "eth0", "Interface to attach to")
	// dir := flag.String("direction", "egress", "Ingress or Egress")

	// flag.Parse()

	// Pass the engine to the Views
	engine := html.New("./views", ".tpl")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	database.Connect()
	handlers.InitializeBpf("eth0")

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
	// app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	app.Get("/metrics", adaptor.HTTPHandlerFunc(handlers.MetricsHandleFunc))

	/*
		Can eventually create groups like so
		// Create a /api/v1 endpoint
		v1 := app.Group("/api/v1")

		// Bind handlers
		v1.Get("/users", handlers.UserList)
		v1.Post("/users", handlers.UserCreate)
	*/

	app.Get("/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendFile("static/laptop.svg")
	})

	log.Fatal(app.Listen(":3000"))
}

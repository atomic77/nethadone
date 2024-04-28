package main

import (
	"log"

	"github.com/atomic77/chiron/handlers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
)

func main() {
	// Create a new engine

	// Or from an embedded system
	// See github.com/gofiber/embed for examples
	// engine := html.NewFileSystem(http.Dir("./views", ".html"))

	// Pass the engine to the Views
	engine := html.New("./views", ".tpl")
	app := fiber.New(fiber.Config{
		Views: engine,
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/", handlers.Index)
	app.Get("/interfaces", handlers.Interfaces)
	app.Get("/devices", handlers.Devices)
	app.Get("/rulesets", handlers.Rules)
	app.Post("/rulesets/change", handlers.RuleChange)

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

// Command api serves Kardee promotions over HTTP.
package main

import (
	"log"

	"github.com/gofiber/fiber/v3"

	"github.com/saharak/kardee-go/app/api"
	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/database"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	app := fiber.New(fiber.Config{AppName: "kardee-api"})
	(&api.Server{DB: db}).Routes(app)
	log.Fatal(app.Listen(":" + cfg.Port))
}

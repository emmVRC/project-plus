package main

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"io"
	"log"
	"os"
)

func init() {
	open, err := os.Open("service_conf.json")
	if err != nil {
		log.Printf("failed to open config: %s", err)
		writeDefaultConfig()
		os.Exit(0)
	}

	data, err := io.ReadAll(open)
	if err != nil {
		log.Fatalf("failed to read config: %s", err)
	}

	err = json.Unmarshal(data, &ServiceConfig)
	if err != nil {
		log.Fatalf("failed to unmarshal json data: %s", err)
	}

}

func writeDefaultConfig() {
	defaultData, err := json.MarshalIndent(&ServiceConfig, "", "    ")
	if err != nil {
		log.Fatalf("failed to marshal json data: %s", err)
	}

	err = os.WriteFile("service_conf.json", defaultData, 660)
	if err != nil {
		log.Fatalf("failed to write config: %s", err)
	}
}

func main() {
	SetupDatabaseConnection()
	SetupRedisConnection()

	app := fiber.New(fiber.Config{
		Prefork: true,
		//JSONEncoder: sonic.Marshal,
		//JSONDecoder: sonic.Unmarshal,
	})

	app.Use(recover.New())
	app.Use(logger.New())

	appGroup := app.Group("/api/v2")
	authRoutes(appGroup)
	favoriteRoutes(appGroup)
	adminRoutes(appGroup)

	InitCheckService()

	log.Fatal(app.Listen(":3002"))
}

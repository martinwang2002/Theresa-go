package httpserver

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

type (
	SubdomainFiber struct {
		Fiber *fiber.App
	}
)

type AppS3 struct {
	*fiber.App
}

type AppStatic struct {
	*fiber.App
}

// func CreateHttpServer() *fiber.App{
func CreateHttpServer() (*fiber.App, *AppS3, *AppStatic) {
	// Hosts
	SubdomainFibers := map[string]*SubdomainFiber{}

	//---------
	// Website
	//---------

	appS3 := fiber.New()

	SubdomainFibers["s3"] = &SubdomainFiber{appS3}

	appS3.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendString("This is internal site for s3.")
	})

	appStatic := fiber.New()

	SubdomainFibers["static"] = &SubdomainFiber{appStatic}

	appStatic.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	// Server
	app := fiber.New()

	// Logging middleware
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} ${latency} ${method} ${host} ${path}\n",
		TimeFormat: "2006-01-02T15:04:05-0700",
	}))

	// subdomain middleware
	app.Use(func(ctx *fiber.Ctx) error {
		subdomain := strings.Split(ctx.Hostname(), ".")[0]

		subdomainFiber := SubdomainFibers[subdomain]
		if subdomainFiber == nil {
			return ctx.SendStatus(fiber.StatusNotFound)
		} else {
			handler := subdomainFiber.Fiber.Handler()
			handler(ctx.Context())
			return nil
		}
	})

	// return app
	return app, &AppS3{appS3}, &AppStatic{appStatic}
}

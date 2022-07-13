package httpserver

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"theresa-go/internal/config"
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

func CreateHttpServer(conf *config.Config) (*fiber.App, *AppS3, *AppStatic) {
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

	appStatic := fiber.New(fiber.Config{
		CaseSensitive: true,
	})

	SubdomainFibers["static"] = &SubdomainFiber{appStatic}

	appStatic.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	// Server
	app := fiber.New(fiber.Config{
		// Override default error handler
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {

			fmt.Printf("Error Handler")
			// Status code defaults to 500
			code := fiber.StatusInternalServerError

			// Retrieve the custom status code if it's an fiber.*Error
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			

			return ctx.Status(code).SendString("Internal Server Error")
		},
	})

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

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(ctx *fiber.Ctx, e any) {
			buf := make([]byte, 4096)
			buf = buf[:runtime.Stack(buf, false)]
			_, _ = os.Stderr.WriteString(fmt.Sprintf("panic: %v\n%s\n", e, buf))
		},
	}))

	if conf.DevMode {
		appS3.Use(pprof.New())
		appStatic.Use(pprof.New())
	}

	// return app
	return app, &AppS3{appS3}, &AppStatic{appStatic}
}

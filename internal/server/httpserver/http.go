package httpserver

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"

	"theresa-go/internal/config"
	"theresa-go/internal/middlewares/logger"

	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/storage/redis"
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
	log := zerolog.New(os.Stdout)

	var fiberConfig fiber.Config = fiber.Config{
		// Override default error handler
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			// Status code defaults to 500
			code := fiber.StatusInternalServerError

			// Retrieve the custom status code if it's an fiber.*Error
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}

			log.Error().
				Err(err).
				Dict("http", zerolog.Dict().
					Dict("request", zerolog.Dict().
						Str("method", ctx.Method()).
						Str("path", string(ctx.Request().URI().RequestURI())).
						Str("host", string(ctx.Context().Host())),
					).
					Dict("response", zerolog.Dict().
						Int("statusCode", ctx.Response().StatusCode()).
						Dict("body", zerolog.Dict().
							Int("bytes", ctx.Response().Header.ContentLength()).
							Int("bytesSent", len(ctx.Response().Body())),
						),
					),
				).
				Dict("error", zerolog.Dict().
					Str("message", err.Error()),
				).
				Msgf("%s %s", ctx.Method(), ctx.Path())

			return ctx.Status(code).JSON(fiber.Map{
				"http":  fiber.Map{"statusCode": code, "message": "Internal Server Error"},
				"error": fiber.Map{"message": err.Error()},
			})
		},
		DisableKeepalive: true,
		ReadTimeout:      1 * time.Minute,
		WriteTimeout:     1 * time.Minute,
	}

	// Hosts
	SubdomainFibers := map[string]*SubdomainFiber{}

	//---------
	// Website
	//---------

	appS3 := fiber.New(fiberConfig)

	SubdomainFibers["s3"] = &SubdomainFiber{appS3}

	appS3.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendString("This is internal site for s3.")
	})

	fiberConfigS3 := fiberConfig
	fiberConfigS3.CaseSensitive = true
	appStatic := fiber.New(fiberConfigS3)

	SubdomainFibers["static"] = &SubdomainFiber{appStatic}

	appStatic.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.SendStatus(fiber.StatusNoContent)
	})

	// Server
	app := fiber.New(fiberConfig)

	app.Use(logger.Logger(&log))

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
		// dev mode enable pprof
		appS3.Use(pprof.New())
		appStatic.Use(pprof.New())
	} else {
		// prod mode enable cache
		// disable when envoy supports cache
		appStatic.Use(cache.New(cache.Config{
			Expiration: 7 * 24 * time.Hour,
			ExpirationGenerator: func(ctx *fiber.Ctx, config *cache.Config) time.Duration {
				if ctx.Response().StatusCode() == fiber.StatusOK {
					return config.Expiration
				} else {
					return 0
				}
			},
			CacheControl:         true,
			Storage:              redis.New(redis.Config{URL: conf.RedisDsn}),
			StoreResponseHeaders: true,
		}))
	}

	// return app
	return app, &AppS3{appS3}, &AppStatic{appStatic}
}

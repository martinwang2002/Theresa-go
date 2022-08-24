package logger

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

func Logger(l *zerolog.Logger) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		sub := l.With().Logger()
		WithLogger(c, &sub)

		err := c.Next()
		logCompleted(c, start)

		return err
	}
}

const loggerKey = "json-logger"

func ReqLogger(c *fiber.Ctx) *zerolog.Logger {
	if logger, ok := c.Locals(loggerKey).(*zerolog.Logger); ok {
		return logger
	}
	return &zerolog.Logger{}
}

func WithLogger(c *fiber.Ctx, l *zerolog.Logger) {
	c.Locals(loggerKey, l)
}

func logCompleted(c *fiber.Ctx, start time.Time) {
	ReqLogger(c).Info().
		Dict("http", zerolog.Dict().
			Dict("request", zerolog.Dict().
				Str("method", c.Method()),
			).
			Dict("response", zerolog.Dict().
				Int("statusCode", c.Response().StatusCode()).
				Dict("body", zerolog.Dict().
					Int("bytes", c.Response().Header.ContentLength()),
				),
			),
		).
		Dict("url", zerolog.Dict().
			Str("path", string(c.Request().URI().RequestURI())).
			Str("host", string(c.Context().Host())),
		).
		Float64("responseTime", float64(time.Since(start).Nanoseconds())/1e6).
		Msg("Request completed")
}

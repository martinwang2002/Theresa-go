package service

import (
	"context"
	"net"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
)

func run(app *fiber.App, lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			app.Server().Logger.Printf("Starting server...")
			ln, err := net.Listen("tcp", ":8000")
			if err != nil {
				return err
			}

			go func() {
				err := app.Listener(ln)
				if err != nil {
					app.Server().Logger.Printf("Server stopped: %s", err.Error())
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			return app.Shutdown()
			// return async.WaitAll(
			// 	async.Errable(app.Shutdown),
			// 	async.Errable(func() error {
			// 		flushed := sentry.Flush(time.Second * 30)
			// 		if !flushed {
			// 			return errors.New("sentry flush timeout, some events may be lost")
			// 		}
			// 		return nil
			// 	}),
			// )
		},
	})
}

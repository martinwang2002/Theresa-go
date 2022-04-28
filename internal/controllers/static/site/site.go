package staticSiteController

import (
	"go.uber.org/fx"

	"theresa-go/internal/server/httpserver"
)

type StaticSiteController struct {
	fx.In
}

func RegisterStaticSiteController(appStatic *httpserver.AppStatic, c StaticSiteController) error {
	appStatic.Get("/robots.txt", c.robotsTxt)
	return nil
}
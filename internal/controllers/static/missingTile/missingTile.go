package staticMissingTileController

import (
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/controllers/static/notFound"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
)

type StaticMissingTileController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func RegisterStaticMissingTileController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticMissingTileController) error {
	appStaticApiV0AK.Get("/missingTile", c.MissingTile)
	return nil
}

func (c *StaticMissingTileController) MissingTile(ctx *fiber.Ctx) error {
	staticNotFoundController := staticNotFoundController.StaticNotFoundController{
		AkAbFs:               c.AkAbFs,
		StaticVersionService: c.StaticVersionService,
	}
	return staticNotFoundController.NotFoundSqaure(ctx)
}

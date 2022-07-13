package appentry

import (
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/controllers/s3"
	"theresa-go/internal/controllers/static/audio"
	"theresa-go/internal/controllers/static/item"
	"theresa-go/internal/controllers/static/map3d"
	"theresa-go/internal/controllers/static/mapPreview"
	"theresa-go/internal/controllers/static/missingTile"
	"theresa-go/internal/controllers/static/site"
	"theresa-go/internal/config"
	"theresa-go/internal/server/httpserver"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/akVersionService"
	"theresa-go/internal/service/staticVersionService"
)

func ProvideOptions(includeSwagger bool) []fx.Option {
	opts := []fx.Option{
		fx.Provide(
			// configs
			config.Parse,
			// fiber.App
			httpserver.CreateHttpServer,
			versioning.CreateS3VersioningEndpoints,
			versioning.CreateStaticVersioningEndpoints,
			// akAbFs
			akAbFs.NewAkAbFs,
			// service
			akVersionService.NewAkVersionService,
			staticVersionService.NewStaticVersionService,
		),
		fx.Invoke(
			// s3
			s3AkAbController.RegisterS3AkController,
			// static
			staticAudioController.RegisterAudioController,
			staticItemController.RegisterStaticItemController,
			staticMap3DController.RegisterstaticMap3DController,
			staticMapPreviewController.RegisterStaticMapPreviewController,
			staticMissingTileController.RegisterStaticMissingTileController,
			staticSiteController.RegisterStaticSiteController,
		),
	}

	return opts
}

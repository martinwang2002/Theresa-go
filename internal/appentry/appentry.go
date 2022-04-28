package appentry

import (
	"go.uber.org/fx"

	"theresa-go/internal/controllers/s3"
	"theresa-go/internal/controllers/static/item"
	"theresa-go/internal/controllers/static/mapPreview"
	"theresa-go/internal/controllers/static/missingTile"
	"theresa-go/internal/controllers/static/site"
	"theresa-go/internal/server/httpserver"
	"theresa-go/internal/server/versioning"
)

func ProvideOptions(includeSwagger bool) []fx.Option {
	opts := []fx.Option{
		fx.Provide(
			httpserver.CreateHttpServer,
			versioning.CreateS3VersioningEndpoints,
			versioning.CreateStaticVersioningEndpoints,
		),
		fx.Invoke(
			// s3
			s3AkAbController.RegisterS3AkController,
			// static
			staticItemController.RegisterStaticItemController,
			staticMapPreviewController.RegisterStaticMapPreviewController,
			staticMissingTileController.RegisterStaticMissingTileController,
			staticSiteController.RegisterStaticSiteController,
		),
	}

	return opts
}

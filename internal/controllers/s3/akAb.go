package s3AkAbController

import (
	"context"
	"fmt"
	"sort"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"

	"theresa-go/internal/fs"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/akVersionService"
)

type S3AkController struct {
	fx.In
}

func RegisterS3AkController(appS3ApiV0AK *versioning.AppS3ApiV0AK, c S3AkController) error {
	appS3ApiV0AK.Get("/latest", c.LatestVersion)
	appS3ApiV0AK.Get("/current", c.LatestVersion)
	appS3ApiV0AK.Get("/version", c.LatestVersion)
	appS3ApiV0AK.Get("/versions", c.Versions)
	appS3ApiV0AK.Get("/assets/:resVersion/*", c.DirectoryHandler)
	return nil
}

func (c *S3AkController) DirectoryHandler(ctx *fiber.Ctx) error {
	// get file path
	path := fmt.Sprintf(
		"%s/%s",
		akVersionService.RealLatestVersionPath(ctx.Params("server"), ctx.Params("platform"), ctx.Params("resVersion")),
		ctx.Params("*"),
	)

	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// try list directory first
	entries, err := akAbFs.List(path)

	if err == nil {
		return ctx.JSON(entries)
	} else {
		// respond with file
		newObject, err := akAbFs.NewObject(path)
		if err != nil {
			return ctx.SendStatus(fiber.StatusNotFound)
		}
		newObjectIoReader, err := newObject.Open(context.Background())
		if err != nil {
			return err
		}
		return ctx.SendStream(newObjectIoReader)
	}
}

func (c *S3AkController) LatestVersion(ctx *fiber.Ctx) error {
	versionFileJson, err := akVersionService.LatestVersion(ctx.Params("server"), ctx.Params("platform"))

	if err != nil {
		return err
	}

	// return json response
	return ctx.JSON(versionFileJson)
}

func (c *S3AkController) Versions(ctx *fiber.Ctx) error {
	entries, err := akAbFs.List(fmt.Sprintf("AK/%s/%s/assets", ctx.Params("server"), ctx.Params("platform")))
	if err != nil {
		return err
	}

	var versions []string
	for _, entry := range entries {
		versions = append(versions, entry.Name)
	}

	sort.Strings(versions)

	return ctx.JSON(versions)
}
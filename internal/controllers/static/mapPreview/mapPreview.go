package staticMapPreviewController

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
)

type StaticMapPreviewController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func RegisterStaticMapPreviewController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticMapPreviewController) error {
	appStaticApiV0AK.Get("/mappreview/:mapId/:width/:quality", c.MapPreview)
	return nil
}

func (c *StaticMapPreviewController) MapPreview(ctx *fiber.Ctx) error {

	allowedImageWidth := map[int]bool{
		16:   true,
		32:   true,
		48:   true,
		64:   true,
		128:  true,
		256:  true,
		384:  true,
		640:  true,
		750:  true,
		828:  true,
		1080: true,
		1200: true,
		1920: true,
		2048: true,
		3840: true,
	}

	allowedImageQuality := map[int]bool{
		75: true,
	}

	width, err := strconv.Atoi(ctx.Params("width"))
	if err != nil && !allowedImageWidth[width] {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	quality, err := strconv.Atoi(ctx.Params("quality"))
	if err != nil && !allowedImageQuality[quality] {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	mapPreviewPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/arts/ui/stage/mappreviews/%s.png", ctx.Params("mapId"))

	mapPreviewObject, err := c.AkAbFs.NewObject(mapPreviewPath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	mapPreviewObjectIoReader, err := mapPreviewObject.Open(context.Background())
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(mapPreviewObjectIoReader)

	// resize image to 16:9 ratio
	resizedImage, err := bimg.NewImage(buf.Bytes()).Process(bimg.Options{
		Width:   width,
		Height:  (width * 9 / 16),
		Quality: quality,
		Type:    bimg.WEBP,
	})

	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	return ctx.SendStream(bytes.NewReader(resizedImage))
}

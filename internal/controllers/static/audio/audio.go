package staticAudioController

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/server/versioning"
)

type StaticAudioController struct {
	fx.In
	AkAbFs *akAbFs.AkAbFs
}

func RegisterAudioController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticAudioController) error {
	appStaticApiV0AK.Get("/audio/*", c.Audio)
	return nil
}

func (c *StaticAudioController) Audio(ctx *fiber.Ctx) error {
	audioPath := ctx.Params("*")
	if !strings.HasSuffix(audioPath, ".wav") {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	audioPath = strings.ToLower(audioPath)

	audioObject, err := c.AkAbFs.NewObjectSmart(ctx.Params("server"), ctx.Params("platform"), "/unpacked_assetbundle/assets/torappu/dynamicassets/audio/"+audioPath)

	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	audioObjectIoReader, err := audioObject.Open(context.Background())

	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "audio/wav")

	return ctx.SendStream(audioObjectIoReader)
}

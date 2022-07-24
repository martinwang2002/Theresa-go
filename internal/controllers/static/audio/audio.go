package staticAudioController

import (
	"bytes"
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	ffmpeg "github.com/u2takey/ffmpeg-go"
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
	if !strings.HasSuffix(audioPath, ".ogg") {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	audioPath = strings.Replace(strings.ToLower(audioPath), ".ogg", ".wav", -1)

	audioObject, err := c.AkAbFs.NewObjectSmart(ctx.Params("server"), ctx.Params("platform"), "/unpacked_assetbundle/assets/torappu/dynamicassets/audio/"+audioPath)

	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	audioObjectIoReader, err := audioObject.Open(context.Background())

	if err != nil {
		return err
	}

	flacBuf := bytes.NewBuffer(nil)
	err = ffmpeg.
		Input("pipe:", ffmpeg.KwArgs{
			"loglevel": "quiet",
		}).
		Output("pipe:", ffmpeg.KwArgs{
			// audio bitrate
			"ab": "64k",
			// audio format to Opus Interactive Audio Codec (igg)
			"format": "ogg",
		}).
		WithInput(audioObjectIoReader).
		WithOutput(flacBuf).
		ErrorToStdOut().
		Run()
	if err != nil {
		panic(err)
	}

	ctx.Set("Content-Type", "audio/ogg")

	return ctx.SendStream(flacBuf)
}

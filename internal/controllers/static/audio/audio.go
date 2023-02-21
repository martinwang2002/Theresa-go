package staticAudioController

import (
	"bytes"
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
	audioPath := strings.ToLower(ctx.Params("*"))
	if !(strings.HasSuffix(audioPath, ".ogg") || strings.HasSuffix(audioPath, ".mp3")) {
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	indexOfDot := strings.LastIndex(audioPath, ".")
	audioFilePath := audioPath[:indexOfDot] + ".wav"
	audioFileExtension := audioPath[indexOfDot+1:]

	audioObject, err := c.AkAbFs.NewObjectSmart(ctx.UserContext(), ctx.Params("server"), ctx.Params("platform"), "/unpacked_assetbundle/assets/torappu/dynamicassets/audio/"+audioFilePath)

	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	audioObjectIoReader, err := audioObject.Open(ctx.UserContext())

	if err != nil {
		return err
	}

	ffmpegBuffer := bytes.NewBuffer(nil)
	err = ffmpeg.
		Input("pipe:", ffmpeg.KwArgs{
			"loglevel": "quiet",
		}).
		Output("pipe:", ffmpeg.KwArgs{
			// audio bitrate
			"ab": "64k",
			// audio format to Opus Interactive Audio Codec (igg)
			"format": audioFileExtension,
		}).
		WithInput(audioObjectIoReader).
		WithOutput(ffmpegBuffer).
		ErrorToStdOut().
		Run()
	if err != nil {
		panic(err)
	}

	ctx.Set("Content-Type", "audio/"+audioFileExtension)

	return ctx.SendStream(ffmpegBuffer)
}

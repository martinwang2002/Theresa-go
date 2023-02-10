package staticNotFoundController

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/service/staticVersionService"
	"theresa-go/internal/service/webpService"
)

type StaticNotFoundController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func (c *StaticNotFoundController) NotFoundSqaure(ctx *fiber.Ctx) error {
	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	missingTilePath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/arts/[pack]common/missing.png")

	newObject, err := c.AkAbFs.NewObject(missingTilePath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	newObjectIoReader, err := newObject.Open(cancelContext)
	if err != nil {
		return err
	}
	defer newObjectIoReader.Close()

	buf := new(bytes.Buffer)
	defer buf.Reset()
	buf.ReadFrom(newObjectIoReader)

	encodedWebpBuffer, err := webpService.EncodeWebp(buf.Bytes(), 100)

	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	return ctx.Send(encodedWebpBuffer)
}

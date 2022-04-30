package staticItemController

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"github.com/tidwall/gjson"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
	"theresa-go/internal/service/webpService"
)

type StaticItemController struct {
	fx.In
	AkAbFs *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func RegisterStaticItemController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticItemController) error {
	appStaticApiV0AK.Get("/item/:itemId", c.ItemImage)
	return nil
}

func (c *StaticItemController) ItemImage(ctx *fiber.Ctx) error {

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	// get item info starts
	itemTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/item_table.json")

	itemTableJsonObject, err := c.AkAbFs.NewObject(itemTableJsonPath)

	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	itemTableJsonObjectIoReader, err := itemTableJsonObject.Open(context.Background())

	if err != nil {
		return err
	}

	itemTableJsonBytes, err := ioutil.ReadAll(itemTableJsonObjectIoReader)
	if err != nil {
		return err
	}

	itemTableJson := gjson.ParseBytes(itemTableJsonBytes).Map()

	if !itemTableJson["items"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	items := itemTableJson["items"].Map()
	if !items[ctx.Params("itemId")].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	item := items[ctx.Params("itemId")].Map()

	if !item["rarity"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	rarity := item["rarity"].Int()
	iconId := item["iconId"].String()
	// get item info ends

	// get rarity image
	rarityWithOffset := rarity + 1
	rarityString := strconv.Itoa(int(rarityWithOffset))

	spriteItemRXImageBytes, err := os.ReadFile(fmt.Sprintf("./item/sprite_item_r%s.png",rarityString ))
	if err != nil {
		return err
	}

	// get item image
	itemPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/arts/items/icons/%s.png", strings.ToLower(iconId))

	itemObject, err := c.AkAbFs.NewObject(itemPath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	itemObjectIoReader, err := itemObject.Open(context.Background())
	if err != nil {
		return err
	}

	itemImageBuf := new(bytes.Buffer)
	itemImageBuf.ReadFrom(itemObjectIoReader)

	// Add item image with rarity
	spriteItemRXImage := bimg.NewImage(spriteItemRXImageBytes)
	itemImage := bimg.NewImage(itemImageBuf.Bytes())

	// get sizes of images for center of watermark
	spriteItemRXImageSize, err := spriteItemRXImage.Size()
	if err != nil {
		return err
	}

	itemImageSize, err := itemImage.Size()
	if err != nil {
		return err
	}

	// Add watermark
	itemImageWithBackGround, err := spriteItemRXImage.Process(bimg.Options{
		WatermarkImage: bimg.WatermarkImage{
			// offset image to center
			Left:    (spriteItemRXImageSize.Width - itemImageSize.Width) / 2,
			Top:     (spriteItemRXImageSize.Height - itemImageSize.Height) / 2,
			Buf:     itemImageBuf.Bytes(),
			Opacity: 1,
		},
	})
	if err != nil {
		return err
	}

	encodedWebpBuffer, err := webpService.EncodeWebp(itemImageWithBackGround, 100)
	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	return ctx.SendStream(bytes.NewReader(encodedWebpBuffer))
}

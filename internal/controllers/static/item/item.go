package staticItemController

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
)

type StaticItemController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
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

	itemTableJsonResult, err := c.AkAbFs.NewJsonObject(itemTableJsonPath)

	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	itemTableJson := itemTableJsonResult.Map()

	if !itemTableJson["items"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	items := itemTableJson["items"].Map()

	itemId := ctx.Params("itemId")

	itemSpriteBackgroundName := ""

	if items[itemId].Exists() {
		itemSpriteBackgroundName = "sprite_item_r"
	} else {
		return ctx.SendStatus(fiber.StatusNotFound)
	}
	item := items[itemId].Map()

	if !item["rarity"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	rarity := item["rarity"].Int()
	iconId := item["iconId"].String()
	// get item info ends

	// get rarity image
	rarityWithOffset := rarity + 1
	rarityString := strconv.Itoa(int(rarityWithOffset))

	spriteItemRXImagePath := fmt.Sprintf("./item/%s%s.png", itemSpriteBackgroundName, rarityString)

	spriteItemRXImageBytes, err := os.ReadFile(spriteItemRXImagePath)
	if err != nil {
		return err
	}

	// get item image
	// load mapping from icon hub
	iconHubAbJsonPath := staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/arts/items/icons/icon_hub.ab.json"

	iconHubAbJson, err := c.AkAbFs.NewJsonObject(iconHubAbJsonPath)
	if err != nil {
		return err
	}

	iconHubKeys := iconHubAbJson.Get("icon_hub._keys").Array()
	iconHubIndex := -1
	for index, result := range iconHubKeys {
		if result.Str == strings.ToLower(iconId) {
			iconHubIndex = index
			break
		}
	}

	iconHubItemPath := iconHubAbJson.Get("icon_hub._values." + strconv.Itoa(iconHubIndex)).Str
	itemPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/%s.png", strings.ToLower(iconHubItemPath))

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
		Quality: 100,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	return ctx.SendStream(bytes.NewReader(itemImageWithBackGround))
}

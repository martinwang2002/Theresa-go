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

func (c *StaticItemController) getItemFromItemTable(itemId string, staticProdVersionPath string) (string, int64, int, []byte, error) {
	// get item info starts
	itemTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/item_table.json")

	itemTableJsonResult, err := c.AkAbFs.NewJsonObject(itemTableJsonPath)
	if err != nil {
		return "", 0, 0, nil, err
	}

	itemTableJson := itemTableJsonResult.Map()

	if !itemTableJson["items"].Exists() {
		return "", 0, 0, nil, fmt.Errorf("item table json does not contain item %s", itemId)
	}

	items := itemTableJson["items"].Map()

	itemSpriteBackgroundName := ""

	if items[itemId].Exists() {
		itemSpriteBackgroundName = "sprite_item_r"
	} else {
		return "", 0, 0, nil, fmt.Errorf("item table json does not contain item %s", itemId)
	}
	item := items[itemId].Map()

	if !item["rarity"].Exists() {
		return "", 0, 0, nil, fmt.Errorf("item table json does not contain item %s rarity", itemId)
	}

	rarity := item["rarity"].Int() + 1 // rarity in item has offset of 1
	iconId := item["iconId"].String()
	itemType := item["itemType"].String()
	// get item info ends

	// get item image
	// load mapping from icon hub
	var iconHubAbJsonPath string
	var iconHubKey string
	var verticalOffset int

	if itemType == "ACTIVITY_ITEM" {
		iconHubAbJsonPath = staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/activity/commonassets.ab.json"
		iconHubKey = "act_item_hub"
		verticalOffset = 17
	} else {
		iconHubAbJsonPath = staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/arts/items/icons/icon_hub.ab.json"
		iconHubKey = "icon_hub"
		verticalOffset = 0
	}
	iconHubAbJson, err := c.AkAbFs.NewJsonObject(iconHubAbJsonPath)
	if err != nil {
		return "", 0, 0, nil, err
	}

	iconHubKeys := iconHubAbJson.Get(iconHubKey + "._keys").Array()
	iconHubIndex := -1
	for index, result := range iconHubKeys {
		if result.Str == strings.ToLower(iconId) {
			iconHubIndex = index
			break
		}
	}

	iconHubItemPath := iconHubAbJson.Get(iconHubKey + "._values." + strconv.Itoa(iconHubIndex)).Str
	itemPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/%s.png", strings.ToLower(iconHubItemPath))

	itemObject, err := c.AkAbFs.NewObject(itemPath)
	if err != nil {
		return "", 0, 0, nil, err
	}

	itemObjectIoReader, err := itemObject.Open(context.Background())
	if err != nil {
		return "", 0, 0, nil, err
	}

	itemImageBuf := new(bytes.Buffer)
	itemImageBuf.ReadFrom(itemObjectIoReader)
	defer itemObjectIoReader.Close()

	return itemSpriteBackgroundName, rarity, verticalOffset, itemImageBuf.Bytes(), nil
}

func (c *StaticItemController) getFurniFromBuildingData(itemId string, staticProdVersionPath string) (string, int64, int, []byte, error) {
	// get building data
	buildingDataJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/building_data.json")

	buildingDataJsonResult, err := c.AkAbFs.NewJsonObject(buildingDataJsonPath)

	if err != nil {
		return "", 0, 0, nil, err
	}

	furnitures := buildingDataJsonResult.Get("customData.furnitures").Map()

	itemSpriteBackgroundName := ""

	if furnitures[itemId].Exists() {
		itemSpriteBackgroundName = "sprite_furni_r"
	} else {
		return "", 0, 0, nil, err
	}

	item := furnitures[itemId].Map()

	if !item["rarity"].Exists() {
		return "", 0, 0, nil, err
	}

	rarity := item["rarity"].Int()
	iconId := item["iconId"].String()
	// get item info ends

	// get item image
	// load mapping from furni icon hub
	furniHubAbJsonPath := staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/arts/ui/furnitureicons/furni_icon_hub.ab.json"

	furniHubAbJson, err := c.AkAbFs.NewJsonObject(furniHubAbJsonPath)
	if err != nil {
		return "", 0, 0, nil, err
	}

	iconHubKeys := furniHubAbJson.Get("furni_icon_hub._keys").Array()
	iconHubIndex := -1
	for index, result := range iconHubKeys {
		if result.Str == strings.ToLower(iconId) {
			iconHubIndex = index
			break
		}
	}

	iconHubItemPath := furniHubAbJson.Get("furni_icon_hub._values." + strconv.Itoa(iconHubIndex)).Str
	itemPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/%s.png", strings.ToLower(iconHubItemPath))

	itemObject, err := c.AkAbFs.NewObject(itemPath)
	if err != nil {
		return "", 0, 0, nil, err
	}

	itemObjectIoReader, err := itemObject.Open(context.Background())
	if err != nil {
		return "", 0, 0, nil, err
	}

	itemImageBuf := new(bytes.Buffer)
	itemImageBuf.ReadFrom(itemObjectIoReader)
	defer itemObjectIoReader.Close()

	itemImage := bimg.NewImage(itemImageBuf.Bytes())

	itemImageZoomed, err := itemImage.Process(bimg.Options{
		// Width: 153,
		Width:  150,
		Height: 112,
	})
	if err != nil {
		return "", 0, 0, nil, err
	}
	return itemSpriteBackgroundName, rarity, 0, itemImageZoomed, nil
}

func (c *StaticItemController) ItemImage(ctx *fiber.Ctx) error {

	itemId := ctx.Params("itemId")

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	var itemSpriteBackgroundName string
	var rarity int64
	var verticalOffset int
	var itemImageBytes []byte
	var err error

	if strings.HasPrefix(itemId, "furni_") {
		itemSpriteBackgroundName, rarity, verticalOffset, itemImageBytes, err = c.getFurniFromBuildingData(itemId, staticProdVersionPath)
	} else {
		itemSpriteBackgroundName, rarity, verticalOffset, itemImageBytes, err = c.getItemFromItemTable(itemId, staticProdVersionPath)
	}

	if err != nil {
		return err
	}

	// get rarity image
	rarityString := strconv.Itoa(int(rarity))

	spriteItemRXImagePath := fmt.Sprintf("./item/%s%s.png", itemSpriteBackgroundName, rarityString)

	spriteItemRXImageBytes, err := os.ReadFile(spriteItemRXImagePath)
	if err != nil {
		return err
	}
	// Add item image with rarity
	spriteItemRXImage := bimg.NewImage(spriteItemRXImageBytes)
	itemImage := bimg.NewImage(itemImageBytes)

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
			Left: (spriteItemRXImageSize.Width - itemImageSize.Width + 1) / 2,
			Top:  verticalOffset + (spriteItemRXImageSize.Height - verticalOffset - itemImageSize.Height + 1) / 2,
			Buf:     itemImageBytes,
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

package staticItemController

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	"image/png"
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
	appStaticApiV0AK.Get("/item/id/:itemId", c.ItemImage)
	appStaticApiV0AK.Get("/item/sprite", c.Sprite)
	return nil
}

type Offset struct {
	X int
	Y int
}

func (c *StaticItemController) getItemOffsetByRootPackingTag(iconId string, rootPackingTag string, staticProdVersionPath string) (Offset, error) {
	var offset Offset

	// find in sprite folder
	spritesFolderItems, err := c.AkAbFs.List(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/spritepack")
	if err != nil {
		return offset, err
	}

	for _, spriteFolderItem := range spritesFolderItems {
		if !spriteFolderItem.IsDir && strings.HasPrefix(spriteFolderItem.Name, strings.ToLower(rootPackingTag)) {

			abJson, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/spritepack/" + spriteFolderItem.Name)

			if err != nil {
				continue
			}
			if abJson.Get("*" + iconId + ".m_RD.textureRectOffset").Exists() {
				offset = Offset{
					X: int(abJson.Get("*" + iconId + ".m_RD.textureRectOffset.x").Int()),
					Y: int(abJson.Get("*" + iconId + ".m_RD.textureRectOffset.y").Int()),
				}
				return offset, nil
			}
		}
	}

	// find in acitivity
	activityFolderItems, err := c.AkAbFs.List(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/activity")
	if err != nil {
		return offset, err
	}

	for _, spriteFolderItem := range activityFolderItems {
		if !spriteFolderItem.IsDir && strings.HasPrefix(spriteFolderItem.Name, "commonassets") {
			abJson, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/activity/" + spriteFolderItem.Name)
			if err != nil {
				continue
			}
			if abJson.Get("*" + iconId + ".m_RD.textureRectOffset").Exists() {
				offset = Offset{
					X: int(abJson.Get("*" + iconId + ".m_RD.textureRectOffset.x").Int()),
					Y: int(abJson.Get("*" + iconId + ".m_RD.textureRectOffset.y").Int()),
				}
				return offset, nil
			}
		}
	}

	// to compoensate hypergryph's bugs?
	gachaFolderItems, err := c.AkAbFs.List(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/ui/gacha")
	if err != nil {
		return offset, err
	}

	for _, gachaFolderItem := range gachaFolderItems {
		if !gachaFolderItem.IsDir {
			abJson, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/ui/gacha/" + gachaFolderItem.Name)
			if err != nil {
				continue
			}
			if abJson.Get("*" + iconId + ".m_RD.textureRectOffset").Exists() {
				offset = Offset{
					X: int(abJson.Get("*" + iconId + ".m_RD.textureRectOffset.x").Int()),
					Y: int(abJson.Get("*" + iconId + ".m_RD.textureRectOffset.y").Int()),
				}
				return offset, nil
			}
		}
	}

	return offset, fmt.Errorf("no offset found for %s", iconId)
}

type IconInfo struct {
	ItemSpriteBackgroundName string
	Rarity                   int
	Offset                   Offset
	ItemImage                *image.Image
}

func (c *StaticItemController) getItemFromItemTable(itemId string, staticProdVersionPath string) (IconInfo, error) {
	// get item info starts
	itemTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/item_table.json")

	itemTableJsonResult, err := c.AkAbFs.NewJsonObject(itemTableJsonPath)
	if err != nil {
		return IconInfo{}, err
	}

	itemTableJson := itemTableJsonResult.Map()

	if !itemTableJson["items"].Exists() {
		return IconInfo{}, fmt.Errorf("item table json does not contain items")
	}

	items := itemTableJson["items"].Map()

	itemSpriteBackgroundName := ""

	if items[itemId].Exists() {
		itemSpriteBackgroundName = "sprite_item_r"
	} else {
		return IconInfo{}, fmt.Errorf("item table json does not contain item %s", itemId)
	}
	item := items[itemId].Map()

	if !item["rarity"].Exists() {
		return IconInfo{}, fmt.Errorf("item table json does not contain item %s rarity", itemId)
	}

	rarity := item["rarity"].Int() + 1 // rarity in item has offset of 1
	iconId := item["iconId"].String()
	itemType := item["itemType"].String()
	// get item info ends

	// get item image
	// load mapping from icon hub
	var iconHubAbJsonPath string
	var iconHubKey string

	if itemType == "ACTIVITY_ITEM" {
		iconHubAbJsonPath = staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/activity/commonassets.ab.json"
		iconHubKey = "act_item_hub"
	} else {
		iconHubAbJsonPath = staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/arts/items/icons/icon_hub.ab.json"
		iconHubKey = "icon_hub"
	}
	iconHubAbJson, err := c.AkAbFs.NewJsonObject(iconHubAbJsonPath)
	if err != nil {
		return IconInfo{}, err
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
		return IconInfo{}, err
	}

	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	itemObjectIoReader, err := itemObject.Open(cancelContext)
	if err != nil {
		return IconInfo{}, err
	}
	defer itemObjectIoReader.Close()

	// get offset defined in sprite?
	rootPackingTag := iconHubAbJson.Get("icon_hub._rootPackingTag").Str

	offset, err := c.getItemOffsetByRootPackingTag(iconId, rootPackingTag, staticProdVersionPath)

	if err != nil {
		return IconInfo{}, err
	}

	itemImage, _, err := image.Decode(itemObjectIoReader)
	if err != nil {
		return IconInfo{}, err
	}

	offset.Y = 181 - itemImage.Bounds().Max.Y - offset.Y

	return IconInfo{
		ItemSpriteBackgroundName: itemSpriteBackgroundName,
		Rarity:                   int(rarity),
		Offset:                   offset,
		ItemImage:                &itemImage,
	}, nil
}

func (c *StaticItemController) getFurniFromBuildingData(itemId string, staticProdVersionPath string) (IconInfo, error) {
	// get building data
	buildingDataJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/building_data.json")

	buildingDataJsonResult, err := c.AkAbFs.NewJsonObject(buildingDataJsonPath)

	if err != nil {
		return IconInfo{}, err
	}

	furnitures := buildingDataJsonResult.Get("customData.furnitures").Map()

	var itemSpriteBackgroundName string

	if furnitures[itemId].Exists() {
		itemSpriteBackgroundName = "sprite_furni_r"
	} else {
		return IconInfo{}, err
	}

	item := furnitures[itemId].Map()

	if !item["rarity"].Exists() {
		return IconInfo{}, err
	}

	rarity := item["rarity"].Int()
	iconId := item["iconId"].String()
	// get item info ends

	// get item image
	// load mapping from furni icon hub
	furniHubAbJsonPath := staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/arts/ui/furnitureicons/furni_icon_hub.ab.json"

	furniHubAbJson, err := c.AkAbFs.NewJsonObject(furniHubAbJsonPath)
	if err != nil {
		return IconInfo{}, err
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
		return IconInfo{}, err
	}

	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	itemObjectIoReader, err := itemObject.Open(cancelContext)
	if err != nil {
		return IconInfo{}, err
	}
	defer itemObjectIoReader.Close()

	itemImageBuf := new(bytes.Buffer)
	defer itemImageBuf.Reset()
	itemImageBuf.ReadFrom(itemObjectIoReader)
	itemImage := bimg.NewImage(itemImageBuf.Bytes())
	itemImageZoomed, err := itemImage.Process(bimg.Options{
		Width:  151, // 226/1.5
		Height: 113, // 169/1.5
	})
	if err != nil {
		return IconInfo{}, err
	}

	itemImageZoomedImage, _, err := image.Decode(bytes.NewReader(itemImageZoomed))
	if err != nil {
		return IconInfo{}, err
	}

	return IconInfo{
		ItemSpriteBackgroundName: itemSpriteBackgroundName,
		Rarity:                   int(rarity),
		Offset: Offset{
			X: 13, // (181[image width]-153[sprite width])/2[center] - 1[offset]
			Y: 30, // (181[image width]-85[sprite width])/2[center] - 1[offset] - 3[extra]
		},
		ItemImage: &itemImageZoomedImage,
	}, nil
}

func (c *StaticItemController) itemImage(itemId string, staticProdVersionPath string) (*image.RGBA, error) {
	var err error
	var iconInfo IconInfo

	if strings.HasPrefix(itemId, "furni_") {
		iconInfo, err = c.getFurniFromBuildingData(itemId, staticProdVersionPath)
	} else {
		iconInfo, err = c.getItemFromItemTable(itemId, staticProdVersionPath)
	}

	if err != nil {
		return nil, err
	}

	// get rarity image
	rarityString := strconv.Itoa(int(iconInfo.Rarity))

	spriteItemRXImagePath := fmt.Sprintf("./resources/item/%s%s.png", iconInfo.ItemSpriteBackgroundName, rarityString)

	spriteItemRXImageIoReader, err := os.Open(spriteItemRXImagePath)
	if err != nil {
		return nil, err
	}

	// Add item image with rarity
	spriteItemImage := image.NewRGBA(image.Rect(0, 0, 181, 181))
	spriteItemRXImage, _, err := image.Decode(spriteItemRXImageIoReader)
	if err != nil {
		return nil, err
	}
	draw.Draw(spriteItemImage, image.Rect(0, 0, 181, 181), spriteItemRXImage, image.Point{0, 0}, draw.Over)
	draw.Draw(spriteItemImage,
		image.Rect(1+iconInfo.Offset.X, 1+iconInfo.Offset.Y, 181, 181),
		*iconInfo.ItemImage,
		image.Point{0, 0},
		draw.Over,
	)

	return spriteItemImage, err
}

func (c *StaticItemController) ItemImage(ctx *fiber.Ctx) error {

	itemId := ctx.Params("itemId")

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	// get png image
	itemImageWithBackGround, err := c.itemImage(itemId, staticProdVersionPath)
	if err != nil {
		return err
	}
	imageBuffer := new(bytes.Buffer)
	defer imageBuffer.Reset()
	png.Encode(imageBuffer, itemImageWithBackGround)
	itemImageWithBackGround = nil

	// convert to webp
	itemWebpImage := bimg.NewImage(imageBuffer.Bytes())
	itemWebpImageBytes, err := itemWebpImage.Process(bimg.Options{
		Quality: 75,
		Type:    bimg.WEBP,
	})
	ctx.Set("Content-Type", "image/webp")
	if err != nil {
		return err
	}

	return ctx.SendStream(bytes.NewReader(itemWebpImageBytes))
}

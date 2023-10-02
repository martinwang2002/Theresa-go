package staticEnemyAvatarController

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/controllers/static/notFound"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
)

type StaticItemController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func RegisterStaticEnemyAvatarController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticItemController) error {
	appStaticApiV0AK.Get("/enemy/avatar/id/:enemyId", c.EnemyImage)
	appStaticApiV0AK.Get("/enemy/avatar/sprite", c.Sprite)
	return nil
}

func (c *StaticItemController) enemyImage(ctx context.Context, enemyId string, staticProdVersionPath string) (image.Image, error) {
	enemyHandbookTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/enemy_handbook_table.json")
	enemyHandbookTableJsonResult, err := c.AkAbFs.NewJsonObject(ctx, enemyHandbookTableJsonPath)
	if err != nil {
		return nil, err
	}

	if !enemyHandbookTableJsonResult.Get("enemyData." + enemyId).Exists() {
		return nil, fmt.Errorf("enemyId %s not found", enemyId)
	}

	if enemyHandbookTableJsonResult.Get("enemyData." + enemyId + ".hideInHandbook").Bool() {
		return nil, fmt.Errorf("enemyId %s is hidden in handbook", enemyId)
	}

	enemyIconsAbPath := staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/arts/enemies/ahub_enemy_icons.ab.json"

	enemyIconsAbJson, err := c.AkAbFs.NewJsonObject(ctx, enemyIconsAbPath)
	if err != nil {
		return nil, err
	}

	iconHubIndex := -1
	for index, result := range enemyIconsAbJson.Get("ahub_enemy_icons._keys").Array() {
		if result.Str == strings.ToLower(enemyId) {
			iconHubIndex = index
			break
		}
	}

	iconHubItemPath := enemyIconsAbJson.Get("ahub_enemy_icons._values." + strconv.Itoa(iconHubIndex)).Str
	enemyIconPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/%s.png", strings.ToLower(iconHubItemPath))

	enemyIconObject, err := c.AkAbFs.NewObject(ctx, enemyIconPath)
	if err != nil {
		fmt.Println(enemyId, iconHubIndex)
		fmt.Println(iconHubItemPath)
		fmt.Println(err)
		return nil, err
	}

	enemyIconIoReader, err := enemyIconObject.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer enemyIconIoReader.Close()

	enemyIcon, _, err := image.Decode(enemyIconIoReader)
	if err != nil {
		return nil, err
	}

	return enemyIcon, err
}

func (c *StaticItemController) EnemyImage(ctx *fiber.Ctx) error {
	enemyId := ctx.Params("enemyId")

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.UserContext(), ctx.Params("server"), ctx.Params("platform"))

	// get png image
	enemyImage, err := c.enemyImage(ctx.UserContext(), enemyId, staticProdVersionPath)
	if err != nil {
		// 404 if hidden in handbook, instead of raising internal server error
		if strings.Contains(err.Error(), "hidden in handbook") {
			staticNotFoundController := staticNotFoundController.StaticNotFoundController{
				AkAbFs:               c.AkAbFs,
				StaticVersionService: c.StaticVersionService,
			}
			return staticNotFoundController.NotFoundSqaure(ctx)
		}
		return err
	}
	var imageBuffer bytes.Buffer

	if err := png.Encode(&imageBuffer, enemyImage); err != nil {
		return err
	}

	// convert to webp
	itemWebpImage := bimg.NewImage(imageBuffer.Bytes())
	imageBuffer.Reset()

	itemWebpImageBytes, err := itemWebpImage.Process(bimg.Options{
		Quality: 75,
		Type:    bimg.WEBP,
	})
	ctx.Set("Content-Type", "image/webp")
	if err != nil {
		return err
	}

	return ctx.Send(itemWebpImageBytes)
}

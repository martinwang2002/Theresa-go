package staticEnemyAvatarController

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
)

func (c *StaticItemController) Sprite(ctx *fiber.Ctx) error {
	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	enemyHandbookTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/enemy_handbook_table.json")
	enemyHandbookTableJsonResult, err := c.AkAbFs.NewJsonObject(enemyHandbookTableJsonPath)
	if err != nil {
		return err
	}

	enemyIdsInGjson := enemyHandbookTableJsonResult.Get("@values").Array()
	enemyIds := make([]string, 0)
	for _, enemyIdInGjson := range enemyIdsInGjson {
		// if hide in handbook is true,
		// then there is no avatar image
		if !enemyIdInGjson.Get("hideInHandbook").Bool() {
			enemyIds = append(enemyIds, enemyIdInGjson.Get("enemyId").Str)
		}
	}

	numOfItems := len(enemyIds)
	numOfRowsAndCols := int(math.Sqrt(float64(numOfItems))) + 1

	// generate all enemy avatars
	var wg sync.WaitGroup
	max := 10 // wait group concurrency limit
	semaphore := make(chan struct{}, max)
	wg.Add(len(enemyIds))

	enemyAvatarImageChannel := make([]*image.Image, numOfItems)
	enemyAvatarErrorChannel := make([]error, numOfItems)

	for index, enemyId := range enemyIds {
		go func(index int, enemyId string) {
			defer wg.Done()
			semaphore <- struct{}{} // acquire semaphore
			enemyImage, err := c.enemyImage(enemyId, staticProdVersionPath)
			enemyAvatarImageChannel[index] = enemyImage
			enemyAvatarErrorChannel[index] = err
			<-semaphore // release semaphore
		}(index, enemyId)
	}
	wg.Wait()

	spriteImageDimension := 158
	spriteEmptyImageRGBA := image.NewRGBA(image.Rect(0, 0, numOfRowsAndCols*spriteImageDimension, (int(numOfItems/numOfRowsAndCols)+1)*spriteImageDimension))

	for index := range enemyIds {
		row := index / numOfRowsAndCols
		col := index % numOfRowsAndCols

		if enemyAvatarErrorChannel[index] != nil {
			return enemyAvatarErrorChannel[index]
		}

		draw.Draw(
			spriteEmptyImageRGBA,
			image.Rect(col*spriteImageDimension, row*spriteImageDimension, (col+1)*spriteImageDimension, (row+1)*spriteImageDimension),
			*enemyAvatarImageChannel[index],
			image.Point{0, 0},
			draw.Src,
		)
	}
	enemyAvatarImageChannel = nil
	enemyAvatarErrorChannel = nil

	spritePngImageBuffer := new(bytes.Buffer)
	encoder := png.Encoder{
		CompressionLevel: png.BestSpeed,
	}
	err = encoder.Encode(spritePngImageBuffer, spriteEmptyImageRGBA)
	spriteEmptyImageRGBA = nil

	if err != nil {
		return err
	}

	// convert to webp
	spriteItemWebpImage := bimg.NewImage(spritePngImageBuffer.Bytes())
	spritePngImageBuffer = nil
	spriteItemWebpImageBytes, err := spriteItemWebpImage.Process(bimg.Options{
		Quality: 25,
		Type:    bimg.WEBP,
	})
	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	// set metadata header
	itemIdsJson, err := json.Marshal(enemyIds)
	if err != nil {
		return err
	}
	ctx.Set("X-Dimension", strconv.Itoa(spriteImageDimension))
	ctx.Set("X-Cols", strconv.Itoa(numOfRowsAndCols))
	ctx.Set("X-Rows", strconv.Itoa(numOfItems/numOfRowsAndCols+1))
	ctx.Set("X-Item-Ids", string(itemIdsJson))
	ctx.Set("Access-Control-Expose-Headers", "X-Dimension,X-Cols,X-Rows,X-Item-Ids")

	return ctx.Send(spriteItemWebpImageBytes)
}

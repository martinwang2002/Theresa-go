package staticEnemyAvatarController

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"runtime"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
)

func (c *StaticItemController) Sprite(ctx *fiber.Ctx) error {
	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.UserContext(), ctx.Params("server"), ctx.Params("platform"))

	enemyHandbookTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/enemy_handbook_table.json")
	enemyHandbookTableJsonResult, err := c.AkAbFs.NewJsonObject(ctx.UserContext(), enemyHandbookTableJsonPath)
	if err != nil {
		return err
	}

	enemyIdsInGjson := enemyHandbookTableJsonResult.Get("enemyData|@values").Array()
	enemyIds := make([]string, 0)
	for _, enemyIdInGjson := range enemyIdsInGjson {
		// if hide in handbook is true,
		// then there is no avatar image
		if !enemyIdInGjson.Get("hideInHandbook").Bool() {
			enemyIds = append(enemyIds, enemyIdInGjson.Get("enemyId").Str)
		}
	}
	numOfItems := len(enemyIds)
	numOfRowsAndCols := int(math.Ceil(math.Sqrt(float64(numOfItems))))

	// get enemy avatar image in parallel
	type ImageChannel struct {
		Image image.Image
		Index int
		Err   error
	}

	enemyAvatarImageChannel := make(chan ImageChannel, 3)
	var wg sync.WaitGroup
	wg.Add(numOfItems)

	for index := range enemyIds {
		go func(index int) {
			defer wg.Done()
			enemyAvatarImage, err := c.enemyImage(ctx.UserContext(), enemyIds[index], staticProdVersionPath)
			enemyAvatarImageChannel <- ImageChannel{
				Image: enemyAvatarImage,
				Index: index,
				Err:   err,
			}
		}(index)
	}

	spriteImageDimension := 158
	spriteEmptyImageRGBA := image.NewRGBA(image.Rect(0, 0, numOfRowsAndCols*spriteImageDimension, (int(numOfItems/numOfRowsAndCols)+1)*spriteImageDimension))

	for range enemyIds {
		imageChannel := <-enemyAvatarImageChannel
		index := imageChannel.Index
		row := index / numOfRowsAndCols
		col := index % numOfRowsAndCols

		if imageChannel.Err != nil {
			return imageChannel.Err
		}

		draw.Draw(
			spriteEmptyImageRGBA,
			image.Rect(col*spriteImageDimension, row*spriteImageDimension, (col+1)*spriteImageDimension, (row+1)*spriteImageDimension),
			imageChannel.Image,
			image.Point{0, 0},
			draw.Src,
		)
	}
	wg.Wait()
	close(enemyAvatarImageChannel)

	spritePngImageBuffer := new(bytes.Buffer)
	encoder := png.Encoder{
		CompressionLevel: png.BestSpeed,
	}
	err = encoder.Encode(spritePngImageBuffer, spriteEmptyImageRGBA)

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

	defer runtime.GC()

	return ctx.Send(spriteItemWebpImageBytes)
}

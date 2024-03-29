package staticItemController

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"regexp"
	"runtime"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"github.com/tidwall/gjson"
)

func (c *StaticItemController) Sprite(ctx *fiber.Ctx) error {
	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.UserContext(), ctx.Params("server"), ctx.Params("platform"))

	itemTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/item_table.json")

	itemTableJsonResult, err := c.AkAbFs.NewJsonObject(ctx.UserContext(), itemTableJsonPath)
	if err != nil {
		return err
	}

	items := itemTableJsonResult.Get("items")

	pureNumberKeysRegex, err := regexp.Compile("^[0-9]*$")
	if err != nil {
		return err
	}

	filtereditemIds := []string{}
	items.ForEach(func(key, value gjson.Result) bool {
		itemId := key.String()
		if pureNumberKeysRegex.MatchString(itemId) || itemId == "AP_GAMEPLAY" {
			filtereditemIds = append(filtereditemIds, itemId)
		}
		return true // keep iterating
	})

	numOfItems := len(filtereditemIds)
	numOfRowsAndCols := int(math.Sqrt(float64(numOfItems))) + 1

	spriteImageDimension := 181
	spriteEmptyImageRGBA := image.NewRGBA(image.Rect(0, 0, numOfRowsAndCols*spriteImageDimension, (int(numOfItems/numOfRowsAndCols)+1)*spriteImageDimension))

	var wg sync.WaitGroup
	max := 10 // wait group concurrency limit
	semaphore := make(chan struct{}, max)
	wg.Add(len(filtereditemIds))

	itemImageChannel := make([]*image.RGBA, len(filtereditemIds))
	itemImageErrorChannel := make([]error, len(filtereditemIds))
	for index, itemId := range filtereditemIds {
		go func(index int, itemId string) {
			defer wg.Done()
			semaphore <- struct{}{} // acquire semaphore
			itemImage, err := c.itemImage(ctx.UserContext(), itemId, staticProdVersionPath)
			itemImageChannel[index] = itemImage
			itemImageErrorChannel[index] = err
			<-semaphore // release semaphore
		}(index, itemId)
	}
	wg.Wait()

	for index := range filtereditemIds {
		row := index / numOfRowsAndCols
		col := index % numOfRowsAndCols

		if itemImageErrorChannel[index] != nil {
			return fmt.Errorf("error when processing item id:%s item image: %w", filtereditemIds[index], itemImageErrorChannel[index])
		}

		draw.Draw(
			spriteEmptyImageRGBA,
			image.Rect(col*spriteImageDimension, row*spriteImageDimension, (col+1)*spriteImageDimension, (row+1)*spriteImageDimension),
			itemImageChannel[index],
			image.Point{0, 0},
			draw.Src,
		)
	}
	itemImageChannel = nil
	itemImageErrorChannel = nil

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
	itemIdsJson, err := json.Marshal(filtereditemIds)
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

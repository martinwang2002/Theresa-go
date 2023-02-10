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
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"github.com/tidwall/gjson"
)

func (c *StaticItemController) Sprite(ctx *fiber.Ctx) error {
	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	itemTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/item_table.json")

	itemTableJsonResult, err := c.AkAbFs.NewJsonObject(itemTableJsonPath)
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
	wg.Add(len(filtereditemIds))

	itemImageChannel := make([]*image.RGBA, len(filtereditemIds))
	itemImageErrorChannel := make([]error, len(filtereditemIds))
	for index, itemId := range filtereditemIds {
		go func(index int, itemId string) {
			defer wg.Done()
			itemImage, err := c.itemImage(itemId, staticProdVersionPath)
			itemImageChannel[index] = itemImage
			itemImageErrorChannel[index] = err
		}(index, itemId)
	}
	wg.Wait()

	for index := range filtereditemIds {
		row := index / numOfRowsAndCols
		col := index % numOfRowsAndCols

		if itemImageErrorChannel[index] != nil {
			return itemImageErrorChannel[index]
		}

		draw.Draw(
			spriteEmptyImageRGBA,
			image.Rect(col*spriteImageDimension, row*spriteImageDimension, (col+1)*spriteImageDimension, (row+1)*spriteImageDimension),
			itemImageChannel[index],
			image.Point{0, 0},
			draw.Src,
		)
	}

	spritePngImageBuffer := new(bytes.Buffer)
	defer spritePngImageBuffer.Reset()
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

	return ctx.SendStream(bytes.NewReader(spriteItemWebpImageBytes))
}

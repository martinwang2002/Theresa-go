package webpService

import (
	"github.com/h2non/bimg"
)

func EncodeWebp(image []byte, quality int) ([]byte, error) {
	if quality < 0 || quality > 100 {
		quality = 100
	}

	webpWithQuality, err := bimg.NewImage(image).Process(bimg.Options{
		Quality: quality,
		Type: bimg.WEBP,
	})

	if err != nil {
		return nil, err
	}

	return webpWithQuality, nil
}

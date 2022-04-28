package webpService

import (
	"github.com/h2non/bimg"
)

// func EncodeWebpFromStream(stream io.Reader, quality int) (io.Reader, error) {

// 	img, _, err := image.Decode(stream)

// 	if err != nil {
// 		return nil, err
// 	}
// 	encodedWebpIoReader, err := EncodeWebp(img, quality)

// 	if err != nil {
// 		return nil, err
// 	}
// 	return encodedWebpIoReader, err

// }

func EncodeWebp(image []byte, quality int) ([]byte, error) {
	if quality < 0 || quality > 100 {
		quality = 100
	}

	// var buf bytes.Buffer

	// err := webp.Encode(&buf, image, &webp.Options{
	// 	Lossless: true,
	// 	Quality:  float32(quality),
	// })

	imageWithQuality, err := bimg.NewImage(image).Process(bimg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	
	webpWithQuality, err := bimg.NewImage(imageWithQuality).Convert(bimg.WEBP)

	if err != nil {
		return nil, err
	}

	return webpWithQuality, nil
	// encodedWebpIoReader := bytes.NewReader(buf.Bytes())

	// return encodedWebpIoReader, nil
}

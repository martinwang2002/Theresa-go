package versioning

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"

	"theresa-go/internal/server/httpserver"
)

type AppS3ApiV0 struct {
	fiber.Router
}

type AppS3ApiV0AK struct {
	fiber.Router
}

type AppStaticApiV0 struct {
	fiber.Router
}

type AppStaticApiV0AK struct {
	fiber.Router
}

func CreateS3VersioningEndpoints(appS3 *httpserver.AppS3) (*AppS3ApiV0, *AppS3ApiV0AK) {
	appS3ApiV0 := appS3.Group("/api/v0")

	appS3ApiV0AK := appS3ApiV0.Group("/AK/:server/:platform")

	return &AppS3ApiV0{appS3ApiV0}, &AppS3ApiV0AK{appS3ApiV0AK}
}

func CreateStaticVersioningEndpoints(appStatic *httpserver.AppStatic) (*AppStaticApiV0, *AppStaticApiV0AK) {
	appStaticApiV0 := appStatic.Group("/api/v0")

	appStaticApiV0.Use(cors.New())

	appStaticApiV0AK := appStaticApiV0.Group("/AK/:server/:platform")

	return &AppStaticApiV0{appStaticApiV0}, &AppStaticApiV0AK{appStaticApiV0AK}
}

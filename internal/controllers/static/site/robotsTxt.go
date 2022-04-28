package staticSiteController

import (
	"github.com/gofiber/fiber/v2"
)

func (c *StaticSiteController) robotsTxt(ctx *fiber.Ctx) error {
	// disallow all User Agent
	return ctx.SendString("User-agent: *\nDisallow: /\n")
}

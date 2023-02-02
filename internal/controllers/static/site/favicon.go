package staticSiteController

import (
	"github.com/gofiber/fiber/v2"
)

func (c *StaticSiteController) favicon(ctx *fiber.Ctx) error {
	return ctx.Redirect("https://theresa.wiki/favicon.svg", fiber.StatusMovedPermanently)
}

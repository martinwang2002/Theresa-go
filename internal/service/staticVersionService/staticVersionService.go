package staticVersionService

import (
	"fmt"

	"theresa-go/internal/service/akVersionService"
)

func StaticProdVersion(server string, platform string) string {
	latestVersion, err := akVersionService.LatestVersion(server, platform)
	if err != nil {
		panic(err)
	}
	return latestVersion.ResVersion

}

func StaticProdVersionPath(server string, platform string) string {
	resVersion := StaticProdVersion(server, platform)

	return fmt.Sprintf("AK/%s/%s/assets/%s", server, platform, resVersion)
}

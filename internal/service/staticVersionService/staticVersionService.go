package staticVersionService

import (
	"fmt"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/service/akVersionService"
)

type StaticVersionService struct {
	AkAbFs           *akAbFs.AkAbFs
	AkVersionService *akVersionService.AkVersionService
}

func NewStaticVersionService(akAbFs *akAbFs.AkAbFs, akVersionService *akVersionService.AkVersionService) *StaticVersionService {
	return &StaticVersionService{
		AkAbFs:           akAbFs,
		AkVersionService: akVersionService,
	}
}

func (s *StaticVersionService) StaticProdVersion(server string, platform string) string {
	latestVersion, err := s.AkVersionService.LatestVersion(server, platform)
	if err != nil {
		panic(err)
	}
	return latestVersion.ResVersion

}

func (s *StaticVersionService) StaticProdVersionPath(server string, platform string) string {
	resVersion := s.StaticProdVersion(server, platform)

	return fmt.Sprintf("AK/%s/%s/assets/%s", server, platform, resVersion)
}

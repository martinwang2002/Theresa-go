package staticVersionService

import (
	"context"
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

func (s *StaticVersionService) StaticProdVersion(ctx context.Context, server string, platform string) string {
	latestVersion, err := s.AkVersionService.LatestVersion(ctx, server, platform)
	if err != nil {
		panic(err)
	}
	return latestVersion.ResVersion

}

func (s *StaticVersionService) StaticProdVersionPath(ctx context.Context, server string, platform string) string {
	resVersion := s.StaticProdVersion(ctx, server, platform)

	return fmt.Sprintf("AK/%s/%s/assets/%s", server, platform, resVersion)
}

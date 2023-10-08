package akVersionService

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog/log"
	"theresa-go/internal/akAbFs"
)

type AkVersionService struct {
	AkAbFs *akAbFs.AkAbFs
}

func NewAkVersionService(akAbFs *akAbFs.AkAbFs) *AkVersionService {
	return &AkVersionService{
		AkAbFs: akAbFs,
	}
}

type VersionFileJson struct {
	ResVersion    string `json:"resVersion"`
	ClientVersion string `json:"clientVersion"`
	AkAbHash      string `json:"_AK_AB_HASH"`
}

func (s *AkVersionService) LatestVersion(ctx context.Context, server string, platform string) (VersionFileJson, error) {
	var versionFileJson VersionFileJson

	// load version file
	versionFile, err := s.AkAbFs.NewObject(ctx, fmt.Sprintf("AK/%s/%s/version.json", server, platform))
	if err != nil {
		return versionFileJson, err
	}
	// convert version file to json
	versionFileIoReader, err := versionFile.Open(ctx)
	if err != nil {
		return versionFileJson, err
	}
	versionFileBytes, err := io.ReadAll(versionFileIoReader)
	if err != nil {
		return versionFileJson, err
	}
	defer versionFileIoReader.Close()

	json.Unmarshal(versionFileBytes, &versionFileJson)
	prevVersionFileBytes, err := s.AkAbFs.CacheClient.GetBytes(ctx, "LatestVersion"+server+platform)

	setCache := func() {
		s.AkAbFs.CacheClient.SetBytesWithTimeout(ctx, "LatestVersion"+server+platform, versionFileBytes, 5*time.Minute)
	}

	if err == nil {
		if !bytes.Equal(prevVersionFileBytes, versionFileBytes) {
			s.AkAbFs.CacheClient.Flush(ctx)
			log.Info().Msg("Flush cache")

			setCache()
		}
	} else {
		setCache()
	}

	return versionFileJson, nil
}

func (s *AkVersionService) RealLatestVersion(ctx context.Context, server string, platform string, resVersion string) string {
	if resVersion == "latest" {
		latestVersion, err := s.LatestVersion(ctx, server, platform)
		if err != nil {
			panic(err)
		}
		return latestVersion.ResVersion
	}
	return resVersion
}

func (s *AkVersionService) RealLatestVersionPath(ctx context.Context, server string, platform string, resVersion string) string {
	if resVersion == "latest" {
		latestVersion, err := s.LatestVersion(ctx, server, platform)
		if err != nil {
			panic(err)
		}
		resVersion = latestVersion.ResVersion
	}

	return fmt.Sprintf("AK/%s/%s/assets/%s", server, platform, resVersion)
}

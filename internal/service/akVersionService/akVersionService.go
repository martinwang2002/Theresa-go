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

func (s *AkVersionService) LatestVersion(server string, platform string) (VersionFileJson, error) {
	var versionFileJson VersionFileJson

	// load version file
	versionFile, err := s.AkAbFs.NewObject(fmt.Sprintf("AK/%s/%s/version.json", server, platform))
	if err != nil {
		return versionFileJson, err
	}
	// convert version file to json
	versionFileIoReader, err := versionFile.Open(context.Background())
	if err != nil {
		return versionFileJson, err
	}
	versionFileBytes, err := io.ReadAll(versionFileIoReader)
	if err != nil {
		return versionFileJson, err
	}
	defer versionFileIoReader.Close()

	json.Unmarshal(versionFileBytes, &versionFileJson)

	prevVersionFileBytes, err := s.AkAbFs.RedisClient.Get(s.AkAbFs.AkAbFsContext, "LatestVersion"+server+platform).Bytes()

	setCache := func() {
		err = s.AkAbFs.RedisClient.Set(s.AkAbFs.AkAbFsContext, "LatestVersion"+server+platform, versionFileBytes, 5*time.Minute).Err()
		if err != nil {
			log.Error().Err(err).Msg("Failed to set cache LatestVersion")
		}
	}

	if err == nil {
		if !bytes.Equal(prevVersionFileBytes, versionFileBytes) {
			err := s.AkAbFs.RedisClient.FlushDB(s.AkAbFs.AkAbFsContext).Err()
			if err != nil {
				log.Error().Err(err).Msg("Failed to clear cache")
			} else {
				log.Info().Msg("In memory cache purged")
			}
			setCache()
		}
	} else {
		setCache()
	}

	return versionFileJson, nil
}

func (s *AkVersionService) RealLatestVersion(server string, platform string, resVersion string) string {
	if resVersion == "latest" {
		latestVersion, err := s.LatestVersion(server, platform)
		if err != nil {
			panic(err)
		}
		return latestVersion.ResVersion
	}
	return resVersion
}

func (s *AkVersionService) RealLatestVersionPath(server string, platform string, resVersion string) string {
	if resVersion == "latest" {
		latestVersion, err := s.LatestVersion(server, platform)
		if err != nil {
			panic(err)
		}
		resVersion = latestVersion.ResVersion
	}

	return fmt.Sprintf("AK/%s/%s/assets/%s", server, platform, resVersion)
}

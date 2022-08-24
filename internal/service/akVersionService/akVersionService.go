package akVersionService

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

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

	prevVersionFileBytes, err := s.AkAbFs.CacheManager.Get(context.Background(), "LatestVersion"+server+platform)
	if err == nil {
		if !bytes.Equal(prevVersionFileBytes, versionFileBytes) {
			err := s.AkAbFs.CacheManager.GetCodec().Clear(context.Background())
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("big cache purged")
			}

			err = s.AkAbFs.CacheManager.Set(context.Background(), "LatestVersion"+server+platform, versionFileBytes)
			if err != nil {
				fmt.Println(err)
			}
		}
	} else {
		err := s.AkAbFs.CacheManager.Set(context.Background(), "LatestVersion"+server+platform, versionFileBytes)
		if err != nil {
			fmt.Println(err)
		}
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

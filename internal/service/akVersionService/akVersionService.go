package akVersionService

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

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
	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	versionFileIoReader, err := versionFile.Open(cancelContext)
	if err != nil {
		return versionFileJson, err
	}
	versionFileBytes, err := io.ReadAll(versionFileIoReader)
	if err != nil {
		return versionFileJson, err
	}
	defer versionFileIoReader.Close()

	json.Unmarshal(versionFileBytes, &versionFileJson)

	prevVersionFileBytes, err := s.AkAbFs.CacheClient.GetBytes("LatestVersion" + server + platform)

	setCache := func() {
		s.AkAbFs.CacheClient.SetBytesWithTimeout("LatestVersion"+server+platform, versionFileBytes, 5*time.Minute)
	}

	if err == nil {
		if !bytes.Equal(prevVersionFileBytes, versionFileBytes) {
			s.AkAbFs.CacheClient.Flush()
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

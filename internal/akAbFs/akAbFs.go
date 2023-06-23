package akAbFs

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"math/rand"
	pathLib "path"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs"
	"github.com/tidwall/gjson"

	"theresa-go/internal/config"
)

type AkAbFs struct {
	akAbFsContext context.Context
	useGithub     bool
	githubClient  *GithubClient
	remoteFs      fs.Fs
	localFs       fs.Fs
	CacheClient   *CacheClient // this is used by other packages for flushing cache
	mu            sync.Mutex
}

func NewAkAbFs(conf *config.Config) *AkAbFs {
	cacheClient := NewCacheClient(conf)

	akAbFsContext := GetBackgroundContext()

	githubClient := GetGithubClient(akAbFsContext, conf)

	remoteFs, err := GetRemoteFs(akAbFsContext, conf)
	if err != nil {
		panic(err)
	}

	localFs, err := GetLocalFs(akAbFsContext)
	if err != nil {
		panic(err)
	}

	return &AkAbFs{
		akAbFsContext: akAbFsContext,
		CacheClient:   cacheClient,
		githubClient:  githubClient,
		localFs:       localFs,
		mu:            sync.Mutex{},
		remoteFs:      remoteFs,
	}
}

func GetBackgroundContext() context.Context {
	return context.Background()
}

func (akAbFs *AkAbFs) list(path string) (fs.DirEntries, error) {
	localEntries, localErr := akAbFs.localFs.List(akAbFs.akAbFsContext, path)
	remoteEntries, remoteError := akAbFs.remoteFs.List(akAbFs.akAbFsContext, path)

	// Raise error if both errors are not nil for listing in local drive and remote drive
	if localErr != nil && remoteError != nil {
		// return remoteError since I guarentee that remote drive is the backup file service
		return nil, remoteError
	}

	allEntries := append(localEntries, remoteEntries...)

	// unique entries
	directories := make(map[string]bool)
	objects := make(map[string]bool)

	entries := fs.DirEntries{}

	for _, entry := range allEntries {
		name := pathLib.Base(entry.Remote())
		switch entry.(type) {
		case fs.Directory:
			if _, value := directories[name]; !value {
				directories[name] = true
				entries = append(entries, entry)
			}
		case fs.Object:
			if _, value := objects[name]; !value {
				objects[name] = true
				entries = append(entries, entry)
			}
		default:
		}
	}
	return entries, nil
}

type JsonDirEntries = []JsonDirEntry

type JsonDirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

func (akAbFs *AkAbFs) List(ctx context.Context, path string) (JsonDirEntries, error) {
	akAbFs.mu.Lock()
	defer akAbFs.mu.Unlock()

	// use cache if available
	cachedEntriesBytes, err := akAbFs.CacheClient.GetBytes(ctx, "List"+path)
	if err == nil {
		var buffer bytes.Buffer
		buffer.Write(cachedEntriesBytes)
		var cachedEntries JsonDirEntries
		err = gob.NewDecoder(&buffer).Decode(&cachedEntries)
		if err == nil {
			return cachedEntries, err
		}
	}

	// load entries
	entries, err := akAbFs.list(path)
	if err != nil {
		return nil, err
	}

	var jsonEntries = make(JsonDirEntries, len(entries))
	for i, entry := range entries {
		switch entry.(type) {
		case fs.Directory:
			jsonEntries[i] = JsonDirEntry{
				Name:  pathLib.Base(entry.Remote()),
				IsDir: true,
			}
		case fs.Object:
			jsonEntries[i] = JsonDirEntry{
				Name:  pathLib.Base(entry.Remote()),
				IsDir: false,
			}
		default:
		}
	}

	// set cache
	var buffer bytes.Buffer
	err = gob.NewEncoder(&buffer).Encode(jsonEntries)
	if err == nil {
		akAbFs.CacheClient.SetBytes(ctx, "List"+path, buffer.Bytes())
	}
	return jsonEntries, nil
}

func (akAbFs *AkAbFs) NewObject(ctx context.Context, path string) (fs.Object, error) {
	localNewObject, err := akAbFs.localNewObject(ctx, path)

	if err == nil {
		return localNewObject, nil
	}

	if akAbFs.githubClient.useGithubGamedata && strings.Contains(path, "gamedata") {
		githubNewObject, err := akAbFs.githubNewObject(ctx, path)
		if err == nil {
			return githubNewObject, nil
		}
	}

	remoteNewObject, err := akAbFs.remoteNewObject(ctx, path)
	if err != nil {
		return nil, err
	}

	// 5% probability of clearing directory cache
	if rand.Intn(100) < 5 {
		defer akAbFs.remoteFs.Features().DirCacheFlush()
		defer runtime.GC()
	}

	return remoteNewObject, nil
}

func (akAbFs *AkAbFs) NewJsonObject(ctx context.Context, path string) (*gjson.Result, error) {
	akAbFs.mu.Lock()
	defer akAbFs.mu.Unlock()

	// use cache if available
	cachedNewJsonObjectGjsonResult, err := akAbFs.CacheClient.GetGjsonResult(ctx, "NewJsonObject"+path)
	if err == nil {
		return cachedNewJsonObjectGjsonResult, nil
	}

	Object, err := akAbFs.NewObject(ctx, path)

	if err != nil {
		return nil, err
	}

	ObjectIoReader, err := Object.Open(ctx)

	if err != nil {
		return nil, err
	}

	ObjectIoReaderBytes, err := io.ReadAll(ObjectIoReader)
	ObjectIoReader.Close()
	if err != nil {
		return nil, err
	}

	gjsonResult := gjson.ParseBytes(ObjectIoReaderBytes)
	akAbFs.CacheClient.SetGjsonResult(ctx, "NewJsonObject"+path, ObjectIoReaderBytes, &gjsonResult)

	return &gjsonResult, nil
}

func (akAbFs *AkAbFs) getAssetFolders(ctx context.Context, server string, platform string, resVersion string) ([]string, error) {
	data, err := akAbFs.CacheClient.GetBytes(ctx, "getAssetFolders"+server+platform+resVersion)
	if err == nil {
		var buffer bytes.Buffer
		buffer.Write(data)
		var folders []string
		err = gob.NewDecoder(&buffer).Decode(&folders)
		if err == nil {
			return folders, nil
		}
	}

	// list dirs
	dirEntries, err := akAbFs.List(ctx, fmt.Sprintf("AK/%s/%s/assets", server, platform))

	if err != nil {
		return nil, err
	}

	folders := make([]string, 0)
	for index, dirEntry := range dirEntries {
		if dirEntry.IsDir && dirEntry.Name != resVersion && dirEntry.Name[:5] != "_next" {
			// do sampling, otherwise it takes too long to respond
			if index%5 == 0 && index < 25 {
				folders = append(folders, dirEntry.Name)
			}
		}
	}

	// sort folders in decending order
	sort.Sort(sort.Reverse(sort.StringSlice(folders)))

	// set cache
	var buffer bytes.Buffer
	err = gob.NewEncoder(&buffer).Encode(folders)
	if err == nil {
		akAbFs.CacheClient.SetBytes(ctx, "getAssetFolders"+server+platform+resVersion, buffer.Bytes())
	}
	return folders, nil
}

func (akAbFs *AkAbFs) NewObjectSmart(ctx context.Context, server string, platform string, path string) (fs.Object, error) {
	path = strings.ReplaceAll(path, "//", "/")
	// remove starting /
	path = strings.TrimPrefix(path, "/")

	// try load object file first

	// load version file
	versionFileJson, err := akAbFs.NewJsonObject(ctx, fmt.Sprintf("AK/%s/%s/version.json", server, platform))
	if err != nil {
		return nil, err
	}

	resVersion := versionFileJson.Map()["resVersion"].Str

	localObjectPath := fmt.Sprintf("AK/%s/%s/assets/%s/%s", server, platform, resVersion, path)

	localNewObject, err := akAbFs.localNewObject(ctx, localObjectPath)

	if err == nil {
		return localNewObject, nil
	}

	if akAbFs.githubClient.useGithubGamedata && strings.Contains(path, "gamedata") {
		githubNewObject, err := akAbFs.githubNewObject(ctx, path)
		if err == nil {
			return githubNewObject, nil
		}
	}

	// load from remote drive
	folders, err := akAbFs.getAssetFolders(ctx, server, platform, resVersion)
	if err != nil {
		return nil, err
	}

	for _, resVersion := range folders {
		remoteObjectPath := fmt.Sprintf("AK/%s/%s/assets/%s/%s", server, platform, resVersion, path)

		remoteNewObject, err := akAbFs.remoteNewObject(ctx, remoteObjectPath)
		if err == nil {
			return remoteNewObject, nil
		}
	}
	return nil, fmt.Errorf("object not found")
}

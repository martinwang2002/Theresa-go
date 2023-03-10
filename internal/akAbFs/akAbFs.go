package akAbFs

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	pathLib "path"
	"sort"
	"strings"
	"sync"

	backendDrive "github.com/rclone/rclone/backend/drive"
	localDrive "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/tidwall/gjson"

	"theresa-go/internal/config"
)

type AkAbFs struct {
	akAbFsContext context.Context
	googleDriveFs fs.Fs
	localFs       fs.Fs
	CacheClient   *CacheClient
	mu            sync.Mutex
}

func NewAkAbFs(conf *config.Config) *AkAbFs {
	cacheClient := NewCacheClient(conf)

	akAbFsContext := GetBackgroundContext()
	googleDriveFs, err := GetGoogleDriveFs(akAbFsContext)
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
		googleDriveFs: googleDriveFs,
		localFs:       localFs,
		mu:            sync.Mutex{},
	}
}

func GetBackgroundContext() context.Context {
	return context.Background()
}

func GetGoogleDriveFs(backgroundContext context.Context) (fs.Fs, error) {
	config := configmap.Simple{}
	driveOptions := new(backendDrive.Options)
	configstruct.Set(config, driveOptions)
	config.Set("client_id", os.Getenv("GOOGLE_DRIVE_CLIENT_ID"))
	config.Set("client_secret", os.Getenv("GOOGLE_DRIVE_CLIENT_SECRET"))
	config.Set("chunk_size", (8 * fs.Mebi).String())
	configToken := map[string]interface{}{
		"access_token":  "1",
		"token_type":    "Bearer",
		"refresh_token": os.Getenv("GOOGLE_DRIVE_REFRESH_TOKEN"),
		"expiry":        "2000-01-01T01:01:01.000000Z",
	}

	out, err := json.Marshal(configToken)
	if err != nil {
		panic(err)
	}
	config.Set("token", string(out))
	config.Set("root_folder_id", os.Getenv("GOOGLE_DRIVE_ROOT_FOLDER_ID"))

	fs, err := backendDrive.NewFs(backgroundContext, "GoogleDrive", "DATA/AK_AB_DATA", config)

	if err != nil {
		return nil, err
	}

	return fs, nil
}

func GetLocalFs(backgroundContext context.Context) (fs.Fs, error) {
	config := configmap.Simple{}

	fs, err := localDrive.NewFs(backgroundContext, "Local", "./AK_AB_DATA/", config)

	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (akAbFs *AkAbFs) list(path string) (fs.DirEntries, error) {
	googleDriveFs := akAbFs.googleDriveFs
	localFs := akAbFs.localFs

	localEntries, localErr := localFs.List(akAbFs.akAbFsContext, path)
	googleDriveEntries, googleDriveErr := googleDriveFs.List(akAbFs.akAbFsContext, path)

	// Raise error if both errors are not nil for listing in local drive and google drive
	if localErr != nil && googleDriveErr != nil {
		// return googleDriveErr since I guarentee that google drive is the backup file service
		return nil, googleDriveErr
	}

	allEntries := append(localEntries, googleDriveEntries...)

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

func (akAbFs *AkAbFs) List(path string) (JsonDirEntries, error) {
	akAbFs.mu.Lock()
	defer akAbFs.mu.Unlock()

	// use cache if available
	cachedEntriesBytes, err := akAbFs.CacheClient.GetBytes("List" + path)
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
		akAbFs.CacheClient.SetBytes("List"+path, buffer.Bytes())
	}
	return jsonEntries, nil
}

func (akAbFs *AkAbFs) localNewObject(path string) (fs.Object, error) {
	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	localNewObject, err := akAbFs.localFs.NewObject(cancelContext, path)
	if err != nil {
		return nil, err
	}
	return localNewObject, nil
}

func (akAbFs *AkAbFs) googleDriveNewObject(path string) (fs.Object, error) {
	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	googleDriveNewObject, err := akAbFs.googleDriveFs.NewObject(cancelContext, path)
	if err != nil {
		return nil, err
	}
	return googleDriveNewObject, nil
}

func (akAbFs *AkAbFs) NewObject(path string) (fs.Object, error) {
	localNewObject, err := akAbFs.localNewObject(path)

	if err == nil {
		return localNewObject, nil
	}

	googleDriveNewObject, err := akAbFs.googleDriveNewObject(path)
	if err != nil {
		return nil, err
	}
	return googleDriveNewObject, nil
}

func (akAbFs *AkAbFs) NewJsonObject(path string) (*gjson.Result, error) {
	akAbFs.mu.Lock()
	defer akAbFs.mu.Unlock()

	// use cache if available
	cachedNewJsonObjectGjsonResult, err := akAbFs.CacheClient.GetGjsonResult("NewJsonObject" + path)
	if err == nil {
		return cachedNewJsonObjectGjsonResult, nil
	}

	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	Object, err := akAbFs.NewObject(path)

	if err != nil {
		return nil, err
	}

	ObjectIoReader, err := Object.Open(cancelContext)

	if err != nil {
		return nil, err
	}

	ObjectIoReaderBytes, err := io.ReadAll(ObjectIoReader)
	if err != nil {
		return nil, err
	}
	defer ObjectIoReader.Close()

	gjsonResult := gjson.ParseBytes(ObjectIoReaderBytes)
	akAbFs.CacheClient.SetGjsonResult("NewJsonObject"+path, ObjectIoReaderBytes, &gjsonResult)

	return &gjsonResult, nil
}

func (akAbFs *AkAbFs) NewObjectSmart(server string, platform string, path string) (fs.Object, error) {
	path = strings.ReplaceAll(path, "//", "/")
	// remove starting /
	path = strings.TrimPrefix(path, "/")

	// try load object file first

	// load version file
	versionFileJson, err := akAbFs.NewJsonObject(fmt.Sprintf("AK/%s/%s/version.json", server, platform))
	if err != nil {
		return nil, err
	}

	resVersion := versionFileJson.Map()["resVersion"].Str

	localObjectPath := fmt.Sprintf("AK/%s/%s/assets/%s/%s", server, platform, resVersion, path)

	localNewObject, err := akAbFs.localNewObject(localObjectPath)

	if err == nil {
		return localNewObject, nil
	}

	// load from google drive

	// list dirs
	dirEntries, err := akAbFs.List(fmt.Sprintf("AK/%s/%s/assets", server, platform))

	if err != nil {
		return nil, err
	}

	folders := make([]string, 0)
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir && dirEntry.Name != resVersion && dirEntry.Name[:5] != "_next" {
			folders = append(folders, dirEntry.Name)
		}
	}

	// sort folders in decending order
	sort.Sort(sort.Reverse(sort.StringSlice(folders)))

	for _, resVersion := range folders {
		googleDriveObjectPath := fmt.Sprintf("AK/%s/%s/assets/%s/%s", server, platform, resVersion, path)

		googleDriveNewObject, err := akAbFs.googleDriveNewObject(googleDriveObjectPath)
		if err == nil {
			return googleDriveNewObject, nil
		}
	}
	return nil, fmt.Errorf("object not found")
}

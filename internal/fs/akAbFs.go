package akAbFs

import (
	"context"
	"encoding/json"
	"os"
	pathLib "path"

	backendDrive "github.com/rclone/rclone/backend/drive"
	localDrive "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
)

func getBackgroundContext() context.Context {
	return context.Background()
}

func getGoogleDriveFs() (fs.Fs, error) {
	backgroundContext := getBackgroundContext()

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

func getLocalFs() (fs.Fs, error) {
	config := configmap.Simple{}

	backgroundContext := getBackgroundContext()
	fs, err := localDrive.NewFs(backgroundContext, "Local", "./AK_AB_DATA/", config)

	if err != nil {
		return nil, err
	}

	return fs, nil
}

func list(path string) (fs.DirEntries, error) {
	googleDriveFs, err := getGoogleDriveFs()
	if err != nil {
		return nil, err
	}
	localFs, err := getLocalFs()
	if err != nil {
		return nil, err
	}

	localEntries, localErr := localFs.List(getBackgroundContext(), path)
	googleDriveEntries, googleDriveErr := googleDriveFs.List(getBackgroundContext(), path)

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

func List(path string) (JsonDirEntries, error) {
	entries, err := list(path)
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
	return jsonEntries, nil
}

func localNewObject(path string) (fs.Object, error) {
	localFs, err := getLocalFs()
	if err != nil {
		return nil, err
	}

	localNewObject, err := localFs.NewObject(getBackgroundContext(), path)
	if err != nil {
		return nil, err
	}
	return localNewObject, nil
}

func googleDriveNewObject(path string) (fs.Object, error) {
	googleDriveFs, err := getGoogleDriveFs()
	if err != nil {
		return nil, err
	}
	googleDriveNewObject, err := googleDriveFs.NewObject(getBackgroundContext(), path)
	if err != nil {
		return nil, err
	}
	return googleDriveNewObject, nil
}

func NewObject(path string) (fs.Object, error) {
	localNewObject, err := localNewObject(path)

	if err == nil {
		return localNewObject, nil
	}

	googleDriveNewObject, err := googleDriveNewObject(path)
	if err != nil {
		return nil, err
	}
	return googleDriveNewObject, nil
}

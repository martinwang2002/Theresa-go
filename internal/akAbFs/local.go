package akAbFs

import (
	"context"

	"github.com/rclone/rclone/fs/config/configmap"

	"github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
)

func GetLocalFs(backgroundContext context.Context) (fs.Fs, error) {
	config := configmap.Simple{}

	fs, err := local.NewFs(backgroundContext, "Local", "./AK_AB_DATA/", config)

	if err != nil {
		return nil, err
	}

	return fs, nil
}

func (akAbFs *AkAbFs) localNewObject(ctx context.Context, path string) (fs.Object, error) {
	localNewObject, err := akAbFs.localFs.NewObject(ctx, path)
	if err != nil {
		return nil, err
	}
	return localNewObject, nil
}

package akAbFs

import (
	"context"

	_ "github.com/rclone/rclone/backend/all"
	"github.com/rclone/rclone/fs"
	rcloneConfig "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"

	"theresa-go/internal/config"
)

func GetRemoteFs(backgroundContext context.Context, conf *config.Config) (fs.Fs, error) {
	rcloneConfig.SetConfigPath("./configs/rclone.conf")
	configfile.Install()
	rcloneFs, err := fs.NewFs(backgroundContext, conf.AkAbFsRemoteName)
	if err != nil {
		panic(err)
	}

	return rcloneFs, nil
}

func (akAbFs *AkAbFs) remoteNewObject(ctx context.Context, path string) (fs.Object, error) {
	remoteNewObject, err := akAbFs.remoteFs.NewObject(ctx, path)
	if err != nil {
		return nil, err
	}
	return remoteNewObject, nil
}

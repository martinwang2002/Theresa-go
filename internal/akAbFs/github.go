package akAbFs

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-github/v50/github"
	"github.com/rclone/rclone/fs"

	"theresa-go/internal/config"
)

type GithubGamedataRepo struct {
	owner    string
	repo     string
	basePath string
}

type GithubClient struct {
	client             *github.Client
	useGithubGamedata  bool
	githubGamedataRepo GithubGamedataRepo
}

type GithubObject struct {
	fs.Object
	content io.ReadCloser
}

func GetGithubClient(backgroundContext context.Context, conf *config.Config) *GithubClient {
	if !conf.UseGithubGamedata {
		return &GithubClient{
			client:             nil,
			useGithubGamedata:  conf.UseGithubGamedata,
			githubGamedataRepo: GithubGamedataRepo{},
		}
	}

	client := github.NewClient(nil)
	if conf.GithubToken != "" {
		client = github.NewTokenClient(backgroundContext, conf.GithubToken)
	}

	githubGamedataRepoSlices := strings.Split(strings.Replace(conf.GithubGamedataRepo, "https://github.com/", "", 1), "/")
	if len(githubGamedataRepoSlices) < 2 {
		panic("invalid github gamedata repo")
	}

	owner := githubGamedataRepoSlices[0]
	repo := githubGamedataRepoSlices[1]
	ref := githubGamedataRepoSlices[2] + "/" + githubGamedataRepoSlices[3]
	basePath := strings.Replace(conf.GithubGamedataRepo, "https://github.com/"+owner+"/"+repo+"/"+ref+"/", "", 1)

	return &GithubClient{
		client:            client,
		useGithubGamedata: conf.UseGithubGamedata,
		githubGamedataRepo: GithubGamedataRepo{
			owner:    owner,
			repo:     repo,
			basePath: basePath,
		},
	}
}

func (o GithubObject) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	return o.content, nil
}

func (akAbFs *AkAbFs) githubNewObject(ctx context.Context, path string) (fs.Object, error) {
	// get gamedata file from Kengxxiao
	gitPath := strings.Replace(path, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata", "", 1)
	fmt.Println(gitPath, "git")

	fileContent, _, _, err := akAbFs.githubClient.client.Repositories.DownloadContentsWithMeta(
		ctx,
		akAbFs.githubClient.githubGamedataRepo.owner,
		akAbFs.githubClient.githubGamedataRepo.repo,
		akAbFs.githubClient.githubGamedataRepo.basePath+gitPath,
		nil)

	if err != nil {
		fmt.Println(fileContent, err)
		return nil, err
	}

	return GithubObject{
		content: fileContent,
	}, nil
}

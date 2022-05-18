package staticMap3DController

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/tidwall/gjson"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
)

type StaticMap3DController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func RegisterstaticMap3DController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticMap3DController) error {
	appStaticApiV0AK.Get("/map3d/:stageId/config", c.Map3DConfig)
	appStaticApiV0AK.Get("/map3d/:stageId/root_scene/obj", c.Map3DRootSceneObj).Name("map3d.rootScene.obj")
	appStaticApiV0AK.Get("/map3d/:stageId/root_scene/lightmap", c.Map3DRootSceneLightmap).Name("map3d.rootScene.lightmap")
	return nil
}

type Obj struct {
	Obj      string `json:"obj"`
	Lightmap string `json:"lightmap"`
	LightmapConfigs []LightmapConfig `json:"lightmapConfigs"`
}

type Map3DConfig struct {
	RootScene Obj `json:"rootScene"`
}

type LightmapConfig struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
}

func (c *StaticMap3DController) lightmapConfig(staticProdVersionPath string, lowerLevelId string) ([]LightmapConfig, error) {

	sceneAbDirectoryPath := staticProdVersionPath + fmt.Sprintf("/assetbundle/scenes/%s", lowerLevelId)

	sceneAbDirectoryFiles, err := c.AkAbFs.List(sceneAbDirectoryPath)
	if err != nil {
		return nil, err
	}

	sceneAbLockFileName := ""
	for _, sceneAbDirectoryFile := range sceneAbDirectoryFiles {
		if sceneAbDirectoryFile.Name[len(sceneAbDirectoryFile.Name)-5:] == ".lock" {
			sceneAbLockFileName = sceneAbDirectoryFile.Name
		}
	}

	if sceneAbLockFileName == "" {
		return nil, fmt.Errorf("no sceneAbLockPath found")
	}

	sceneAbLockObject, err := c.AkAbFs.NewObject(sceneAbDirectoryPath + "/" + sceneAbLockFileName)

	if err != nil {
		return nil, err
	}

	sceneAbLockObjectIoReader, err := sceneAbLockObject.Open(context.Background())

	if err != nil {
		return nil, err
	}

	sceneAbLockJsonBytes, err := ioutil.ReadAll(sceneAbLockObjectIoReader)
	if err != nil {
		return nil, err
	}

	sceneAbLockJson := gjson.ParseBytes(sceneAbLockJsonBytes).Map()


	if !sceneAbLockJson["files"].Exists() {
		return nil, fmt.Errorf("no files field in .lock file found")
	}

	meshRendererFiles := []string{}

	for _, sceneAbLockJsonFile := range sceneAbLockJson["files"].Array() {
		splittedPath := strings.Split(sceneAbLockJsonFile.Str, "/")
		fileName := splittedPath[len(splittedPath)-1]

		if strings.Contains(fileName, "MeshRenderer") {
			meshRendererFiles = append(meshRendererFiles, sceneAbLockJsonFile.Str)
		}
	}


	lightmapConfigs := make([]LightmapConfig, len(meshRendererFiles) -1)

	for _, meshRendererFile := range meshRendererFiles {


		meshRendererFileObject, err := c.AkAbFs.NewObject(staticProdVersionPath + "/" + meshRendererFile)

		if err != nil {
			return nil, err
		}

		meshRendererFileObjectIoReader, err := meshRendererFileObject.Open(context.Background())

		if err != nil {
			return nil, err
		}

		meshRendererFileJsonBytes, err := ioutil.ReadAll(meshRendererFileObjectIoReader)
		if err != nil {
			return nil, err
		}

		meshRendererFileJson := gjson.ParseBytes(meshRendererFileJsonBytes).Map()




		if !meshRendererFileJson["m_StaticBatchInfo"].Exists()  || !meshRendererFileJson["m_StaticBatchInfo"].Map()["firstSubMesh"].Exists()  {
			return nil, fmt.Errorf("cannot find firstSubMesh")
		}

		firstSubMesh := meshRendererFileJson["m_StaticBatchInfo"].Map()["firstSubMesh"].Int() 

		if !meshRendererFileJson["m_LightmapTilingOffset"].Exists()		{
			return nil, fmt.Errorf("cannot find m_LightmapTilingOffset")
		}

		lightmapConfigs[firstSubMesh] = LightmapConfig{
			X: meshRendererFileJson["m_LightmapTilingOffset"].Map()["x"].Float(),
			Y: meshRendererFileJson["m_LightmapTilingOffset"].Map()["y"].Float(),
			Z: meshRendererFileJson["m_LightmapTilingOffset"].Map()["z"].Float(),
			W: meshRendererFileJson["m_LightmapTilingOffset"].Map()["w"].Float(),
		}
	}

	return lightmapConfigs, nil
}

func (c *StaticMap3DController) Map3DConfig(ctx *fiber.Ctx) error {

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	stageTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/stage_table.json")

	stageTableJsonObject, err := c.AkAbFs.NewObject(stageTableJsonPath)
	if err != nil {
		return err
	}

	stageTableJsonObjectIoReader, err := stageTableJsonObject.Open(context.Background())

	if err != nil {
		return err
	}

	stageTableJsonBytes, err := ioutil.ReadAll(stageTableJsonObjectIoReader)
	if err != nil {
		return err
	}

	stageTableJson := gjson.ParseBytes(stageTableJsonBytes).Map()
	if !stageTableJson["stages"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	stages := stageTableJson["stages"].Map()
	if !stages[ctx.Params("stageId")].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	// stageInfo := stages[ctx.Params("stageId")].Map()
	rootSceneObjPath, err := ctx.GetRouteURL("map3d.rootScene.obj", fiber.Map{
		"server":   ctx.Params("server"),
		"platform": ctx.Params("platform"),
		// TODO: https://github.com/gofiber/fiber/issues/1907
		"stageid": ctx.Params("stageId"),
	})
	if err != nil {
		return err
	}
	rootSceneLightmapPath, err := ctx.GetRouteURL("map3d.rootScene.lightmap", fiber.Map{
		"server":   ctx.Params("server"),
		"platform": ctx.Params("platform"),
		// TODO: https://github.com/gofiber/fiber/issues/1907
		"stageid": ctx.Params("stageId"),
	})
	if err != nil {
		return err
	}
	rootSceneObjUrl := ctx.BaseURL() + rootSceneObjPath
	rootSceneLightmapUrl := ctx.BaseURL() + rootSceneLightmapPath

	stageInfo := stages[ctx.Params("stageId")].Map()
	levelId := stageInfo["levelId"].Str

	lowerLevelId := strings.ToLower(levelId)

	lightmapConfigs, err := c.lightmapConfig(staticProdVersionPath, lowerLevelId)
	if err != nil {
		return err
	} 
	
	rootSceneObj := Obj{
		Obj:      rootSceneObjUrl,
		Lightmap: rootSceneLightmapUrl,
		LightmapConfigs: lightmapConfigs,
	}

	map3DConfig := Map3DConfig{
		RootScene: rootSceneObj,
	}
	return ctx.JSON(map3DConfig)
}

func (c *StaticMap3DController) Map3DRootSceneObj(ctx *fiber.Ctx) error {

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	stageTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/stage_table.json")

	stageTableJsonObject, err := c.AkAbFs.NewObject(stageTableJsonPath)
	if err != nil {
		return err
	}

	stageTableJsonObjectIoReader, err := stageTableJsonObject.Open(context.Background())

	if err != nil {
		return err
	}

	stageTableJsonBytes, err := ioutil.ReadAll(stageTableJsonObjectIoReader)
	if err != nil {
		return err
	}

	stageTableJson := gjson.ParseBytes(stageTableJsonBytes).Map()
	if !stageTableJson["stages"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	stages := stageTableJson["stages"].Map()
	if !stages[ctx.Params("stageId")].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	stageInfo := stages[ctx.Params("stageId")].Map()

	levelId := stageInfo["levelId"].Str

	lowerLevelId := strings.ToLower(levelId)

	splittedLowerLevelId := strings.Split(lowerLevelId, "/")

	mapPreviewPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/scenes/%s/%s.ab/1_Mesh_Combined Mesh (root_ scene).obj", lowerLevelId, splittedLowerLevelId[len(splittedLowerLevelId)-1])

	newObject, err := c.AkAbFs.NewObject(mapPreviewPath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}
	newObjectIoReader, err := newObject.Open(context.Background())
	if err != nil {
		return err
	}
	return ctx.SendStream(newObjectIoReader)
}

func (c *StaticMap3DController) Map3DRootSceneLightmap(ctx *fiber.Ctx) error {
	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	stageTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/stage_table.json")

	stageTableJsonObject, err := c.AkAbFs.NewObject(stageTableJsonPath)
	if err != nil {
		return err
	}

	stageTableJsonObjectIoReader, err := stageTableJsonObject.Open(context.Background())

	if err != nil {
		return err
	}

	stageTableJsonBytes, err := ioutil.ReadAll(stageTableJsonObjectIoReader)
	if err != nil {
		return err
	}

	stageTableJson := gjson.ParseBytes(stageTableJsonBytes).Map()
	if !stageTableJson["stages"].Exists() {
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	stages := stageTableJson["stages"].Map()
	if !stages[ctx.Params("stageId")].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	stageInfo := stages[ctx.Params("stageId")].Map()

	levelId := stageInfo["levelId"].Str

	lowerLevelId := strings.ToLower(levelId)

	splittedLowerLevelId := strings.Split(lowerLevelId, "/")

	mapPreviewPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/scenes/%s/%s/lightmap-0_comp_light.png", lowerLevelId, splittedLowerLevelId[len(splittedLowerLevelId)-1])

	newObject, err := c.AkAbFs.NewObject(mapPreviewPath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}
	newObjectIoReader, err := newObject.Open(context.Background())
	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/png")

	return ctx.SendStream(newObjectIoReader)
}

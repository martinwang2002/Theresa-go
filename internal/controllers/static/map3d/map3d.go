package staticMap3DController

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"

	"theresa-go/internal/akAbFs"
	"theresa-go/internal/server/versioning"
	"theresa-go/internal/service/staticVersionService"
	"theresa-go/internal/service/webpService"
)

type StaticMap3DController struct {
	fx.In
	AkAbFs               *akAbFs.AkAbFs
	StaticVersionService *staticVersionService.StaticVersionService
}

func RegisterstaticMap3DController(appStaticApiV0AK *versioning.AppStaticApiV0AK, c StaticMap3DController) error {
	appStaticApiV0AK.Get("/map3d/stage/:stageId/config", c.Map3DConfig)
	appStaticApiV0AK.Get("/map3d/stage/:stageId/root_scene/obj", c.Map3DRootSceneObj).Name("map3d.rootScene.obj")
	appStaticApiV0AK.Get("/map3d/stage/:stageId/root_scene/lightmap", c.Map3DRootSceneLightmap).Name("map3d.rootScene.lightmap")
	appStaticApiV0AK.Get("/map3d/material/*", c.Map3DTextureMap).Name("map3d.material")
	return nil
}

type Obj struct {
	Obj         string                    `json:"obj"`
	MeshConfigs []MeshConfig              `json:"meshConfigs"`
	Materials   map[string]MaterialConfig `json:"materials"`
}

type MaterialConfig struct {
	Texture       string  `json:"texture"`
	EmissionMap   *string `json:"emissionMap"`
	Color         *Color  `json:"color"`
	EmissionColor *Color  `json:"emissionColor"`
}

type Map3DConfig struct {
	RootScene Obj `json:"rootScene"`
}

type MeshConfig struct {
	Material       string         `json:"material"`
	LightmapConfig LightmapConfig `json:"lightmapConfig"`
}

type LightmapConfig struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
}

type Color struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
	A float64 `json:"a"`
}

func (c *StaticMap3DController) meshConfig(ctx *fiber.Ctx, staticProdVersionPath string, lowerLevelId string) ([]MeshConfig, map[string]MaterialConfig, error) {

	sceneAbDirectoryPath := staticProdVersionPath + fmt.Sprintf("/assetbundle/scenes/%s", lowerLevelId)

	sceneAbDirectoryFiles, err := c.AkAbFs.List(sceneAbDirectoryPath)
	if err != nil {
		return nil, nil, err
	}

	sceneAbLockFileName := ""
	for _, sceneAbDirectoryFile := range sceneAbDirectoryFiles {
		if sceneAbDirectoryFile.Name[len(sceneAbDirectoryFile.Name)-5:] == ".lock" {
			sceneAbLockFileName = sceneAbDirectoryFile.Name
		}
	}

	if sceneAbLockFileName == "" {
		return nil, nil, fmt.Errorf("no sceneAbLockPath found")
	}

	sceneAbLockPath := sceneAbDirectoryPath + "/" + sceneAbLockFileName

	sceneAbLockJsonResult, err := c.AkAbFs.NewJsonObject(sceneAbLockPath)

	if err != nil {
		return nil, nil, err
	}

	sceneAbLockJson := sceneAbLockJsonResult.Map()

	if !sceneAbLockJson["files"].Exists() {
		return nil, nil, fmt.Errorf("no files field in .lock file found")
	}

	preloadDataFile := ""

	meshRendererFiles := []string{}

	for _, sceneAbLockJsonFile := range sceneAbLockJson["files"].Array() {
		splittedPath := strings.Split(sceneAbLockJsonFile.Str, "/")
		fileName := splittedPath[len(splittedPath)-1]

		if strings.Contains(fileName, "MeshRenderer") {
			meshRendererFiles = append(meshRendererFiles, sceneAbLockJsonFile.Str)
		} else if strings.Contains(fileName, "PreloadData") {
			preloadDataFile = sceneAbLockJsonFile.Str
		}
	}

	// load preloadDataFile
	if preloadDataFile == "" {
		return nil, nil, fmt.Errorf("no preloadDataFile found")
	}
	preloadDataFileJsonResult, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/" + preloadDataFile)
	if err != nil {
		return nil, nil, err
	}

	// load all resource references in preloadDataFile
	resourcesInPreloadDataFile := []string{}

	preloadDataFileJsonDependencies := preloadDataFileJsonResult.Map()["m_Dependencies"].Array()

	for _, preloadDataFileJsonDependency := range preloadDataFileJsonDependencies {
		// get files in the lock file
		preloadDataFileJsonDependencyLockFile := strings.Replace(preloadDataFileJsonDependency.Str, ".ab", ".lock", 1)
		preloadDataFileJsonDependencyLockFileJsonResult, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/assetbundle/" + preloadDataFileJsonDependencyLockFile)

		if err != nil {
			return nil, nil, err
		}

		for _, preloadDataFileJsonDependencyLockFileJsonFile := range preloadDataFileJsonDependencyLockFileJsonResult.Map()["files"].Array() {
			resourcesInPreloadDataFile = append(resourcesInPreloadDataFile, preloadDataFileJsonDependencyLockFileJsonFile.Str)
		}
	}

	// Generate meshConfigs
	meshConfigs := make([]MeshConfig, len(meshRendererFiles))
	materials := map[string]MaterialConfig{}

	for _, meshRendererFile := range meshRendererFiles {
		meshRendererPath := staticProdVersionPath + "/" + meshRendererFile

		meshRendererFileJsonResult, err := c.AkAbFs.NewJsonObject(meshRendererPath)

		if err != nil {
			return nil, nil, err
		}

		meshRendererFileJson := meshRendererFileJsonResult.Map()

		if !meshRendererFileJson["m_StaticBatchInfo"].Exists() || !meshRendererFileJson["m_StaticBatchInfo"].Map()["firstSubMesh"].Exists() {
			return nil, nil, fmt.Errorf("cannot find firstSubMesh")
		}

		firstSubMesh := meshRendererFileJson["m_StaticBatchInfo"].Map()["firstSubMesh"].Int()

		if !meshRendererFileJson["m_LightmapTilingOffset"].Exists() {
			return nil, nil, fmt.Errorf("cannot find m_LightmapTilingOffset")
		}

		meshConfigs[firstSubMesh].LightmapConfig = LightmapConfig{
			X: meshRendererFileJson["m_LightmapTilingOffset"].Map()["x"].Float(),
			Y: meshRendererFileJson["m_LightmapTilingOffset"].Map()["y"].Float(),
			Z: meshRendererFileJson["m_LightmapTilingOffset"].Map()["z"].Float(),
			W: meshRendererFileJson["m_LightmapTilingOffset"].Map()["w"].Float(),
		}

		// Material path id
		if !meshRendererFileJson["m_Materials"].Exists() {
			return nil, nil, fmt.Errorf("cannot find m_Materials")
		}

		m_Materials := meshRendererFileJson["m_Materials"].Array()

		if !m_Materials[0].Exists() || !m_Materials[0].Map()["m_PathID"].Exists() {
			return nil, nil, fmt.Errorf("cannot find m_PathID")
		}

		materialPathId := m_Materials[0].Map()["m_PathID"].String()

		meshConfigs[firstSubMesh].Material = materialPathId

		// texture
		if _, exists := materials[materialPathId]; !exists {
			texturePath := ""
			emissionMapPath := ""
			color := &Color{}
			emissionColor := &Color{}

			for _, resourceInPreloadDataFile := range resourcesInPreloadDataFile {
				if strings.Contains(resourceInPreloadDataFile, "/"+materialPathId+"_Material") {
					resourceInPreloadDataFileJsonResult, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/" + resourceInPreloadDataFile)
					if err != nil {
						return nil, nil, err
					}

					if !resourceInPreloadDataFileJsonResult.Map()["m_SavedProperties"].Exists() {
						return nil, nil, fmt.Errorf("cannot find m_SavedProperties")
					}

					materialSavedProperties := resourceInPreloadDataFileJsonResult.Map()["m_SavedProperties"].Map()

					if !materialSavedProperties["m_TexEnvs"].Exists() {
						return nil, nil, fmt.Errorf("cannot find m_TexEnvs")
					}

					materialSavedPropertiesTexEnvs := materialSavedProperties["m_TexEnvs"].Array()

					for _, materialSavedPropertiesTexEnv := range materialSavedPropertiesTexEnvs {
						key := materialSavedPropertiesTexEnv.Array()[0].Str
						if key == "_MainTex" {
							mainTexValue := materialSavedPropertiesTexEnv.Array()[1].Map()

							if !mainTexValue["m_Texture"].Exists() {
								return nil, nil, fmt.Errorf("cannot find m_Texture")
							}

							mainTexPathId := mainTexValue["m_Texture"].Map()["m_PathID"].String()

							if mainTexPathId == "0" {
								return nil, nil, fmt.Errorf("cannot find m_Texture")
							}

							for _, resourceInPreloadDataFile := range resourcesInPreloadDataFile {
								if strings.Contains(resourceInPreloadDataFile, mainTexPathId+"_Texture2D") {
									texturePath = resourceInPreloadDataFile
									break
								}
							}
						} else if key == "_EmissionMap" {
							emissionMapValue := materialSavedPropertiesTexEnv.Array()[1].Map()

							if !emissionMapValue["m_Texture"].Exists() {
								return nil, nil, fmt.Errorf("cannot find m_Texture")
							}

							emissionMapPathId := emissionMapValue["m_Texture"].Map()["m_PathID"].String()

							if emissionMapPathId == "0" {
								continue
							}

							for _, resourceInPreloadDataFile := range resourcesInPreloadDataFile {
								if strings.Contains(resourceInPreloadDataFile, emissionMapPathId+"_Texture2D") {
									emissionMapPath = resourceInPreloadDataFile
									break
								}
							}
						}
					}

					materialSavedPropertiesColors := materialSavedProperties["m_Colors"].Array()

					for _, materialSavedPropertiesColor := range materialSavedPropertiesColors {
						key := materialSavedPropertiesColor.Array()[0].Str
						if key == "_Color" {
							colorValue := materialSavedPropertiesColor.Array()[1].Map()
							color = &Color{
								R: colorValue["r"].Float(),
								G: colorValue["g"].Float(),
								B: colorValue["b"].Float(),
								A: colorValue["a"].Float(),
							}
							continue
						} else if key == "_EmissionColor" {
							emissionColorValue := materialSavedPropertiesColor.Array()[1].Map()
							emissionColor = &Color{
								R: emissionColorValue["r"].Float(),
								G: emissionColorValue["g"].Float(),
								B: emissionColorValue["b"].Float(),
								A: emissionColorValue["a"].Float(),
							}
							continue
						}
					}
				}
			}
			// Params * has bug
			// TODO: https://github.com/gofiber/fiber/issues/1921
			// resourceInPreloadDataFileUrl, err := ctx.GetRouteURL("map3d.material", fiber.Map{
			// 	"server":   ctx.Params("server"),
			// 	"platform": ctx.Params("platform"),
			// 	"*":        strings.Replace(resourceInPreloadDataFile, "unpacked_assetbundle/assets/torappu/dynamicassets/arts/maps/", "", 1),
			// })
			// if err != nil {
			// 	return nil, nil, err
			// }
			if texturePath != "" {
				materials[materialPathId] = MaterialConfig{
					Texture: ctx.BaseURL() + "/api/v0/AK/" + ctx.Params("server") + "/" + ctx.Params("platform") + "/map3d/material/" + strings.Replace(strings.Replace(texturePath, ".png", "", 1), "unpacked_assetbundle/assets/torappu/dynamicassets/arts/maps/", "", 1),
				}
			}

			if materialConfig, ok := materials[materialPathId]; ok {
				if emissionMapPath != "" {
					emissionMapUrl := ctx.BaseURL() + "/api/v0/AK/" + ctx.Params("server") + "/" + ctx.Params("platform") + "/map3d/material/" + strings.Replace(strings.Replace(emissionMapPath, ".png", "", 1), "unpacked_assetbundle/assets/torappu/dynamicassets/arts/maps/", "", 1)
					materialConfig.EmissionMap = &emissionMapUrl
					materials[materialPathId] = materialConfig
				}

				if color != (&Color{}) {
					materialConfig.Color = color
				}

				if emissionColor != (&Color{}) {
					materialConfig.EmissionColor = emissionColor
				}

				materials[materialPathId] = materialConfig
			}
		}
	}

	return meshConfigs, materials, nil
}

func (c *StaticMap3DController) Map3DConfig(ctx *fiber.Ctx) error {

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	stageTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/stage_table.json")

	stageTableJsonResult, err := c.AkAbFs.NewJsonObject(stageTableJsonPath)
	if err != nil {
		return err
	}

	stageTableJson := stageTableJsonResult.Map()

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
		"stageId": ctx.Params("stageId"),
	})
	if err != nil {
		return err
	}
	// rootSceneLightmapPath, err := ctx.GetRouteURL("map3d.rootScene.lightmap", fiber.Map{
	// 	"server":   ctx.Params("server"),
	// 	"platform": ctx.Params("platform"),
	// 	// TODO: https://github.com/gofiber/fiber/issues/1907
	// 	"stageid": ctx.Params("stageId"),
	// })
	// if err != nil {
	// 	return err
	// }
	rootSceneObjUrl := ctx.BaseURL() + rootSceneObjPath
	// rootSceneLightmapUrl := ctx.BaseURL() + rootSceneLightmapPath

	stageInfo := stages[ctx.Params("stageId")].Map()
	levelId := stageInfo["levelId"].Str

	lowerLevelId := strings.ToLower(levelId)

	meshConfigs, materials, err := c.meshConfig(ctx, staticProdVersionPath, lowerLevelId)
	if err != nil {
		return err
	}

	rootSceneObj := Obj{
		Obj:         rootSceneObjUrl,
		MeshConfigs: meshConfigs,
		Materials:   materials,
	}

	map3DConfig := Map3DConfig{
		RootScene: rootSceneObj,
	}
	return ctx.JSON(map3DConfig)
}

func (c *StaticMap3DController) Map3DRootSceneObj(ctx *fiber.Ctx) error {

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	stageTableJsonPath := fmt.Sprintf("%s/%s", staticProdVersionPath, "unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/excel/stage_table.json")

	stageTableJsonResult, err := c.AkAbFs.NewJsonObject(stageTableJsonPath)
	if err != nil {
		return err
	}

	stageTableJson := stageTableJsonResult.Map()

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

	stageTableJsonResult, err := c.AkAbFs.NewJsonObject(stageTableJsonPath)
	if err != nil {
		return err
	}

	stageTableJson := stageTableJsonResult.Map()

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

// texture map in arts/maps/...
func (c *StaticMap3DController) Map3DTextureMap(ctx *fiber.Ctx) error {

	staticProdVersionPath := c.StaticVersionService.StaticProdVersionPath(ctx.Params("server"), ctx.Params("platform"))

	mapTexturePath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/arts/maps/%s.png", ctx.Params("*"))

	newObject, err := c.AkAbFs.NewObject(mapTexturePath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}
	newObjectIoReader, err := newObject.Open(context.Background())
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(newObjectIoReader)

	encodedWebpBuffer, err := webpService.EncodeWebp(buf.Bytes(), 100)

	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	return ctx.SendStream(bytes.NewReader(encodedWebpBuffer))
}

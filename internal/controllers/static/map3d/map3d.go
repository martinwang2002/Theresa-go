package staticMap3DController

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/tidwall/gjson"
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
	appStaticApiV0AK.Get("/map3d/stage/:stageId/config", c.Map3DConfig).Name("map3d.rootScene.config")
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

type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type TexEnv struct {
	Texture *string `json:"texture"`
	Scale   XY      `json:"scale"`
	Offset  XY      `json:"offset"`
}

type MaterialConfig struct {
	TexEnvs *map[string]*TexEnv  `json:"texEnvs"`
	Floats  *map[string]*float64 `json:"floats"`
	Colors  *map[string]*Color   `json:"colors"`
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

func (c *StaticMap3DController) formatMaterialKey(key string) string {
	trimedString := strings.TrimLeft(key, "_")
	// to camelCase
	toLowerIndex := 0
	for index, char := range trimedString {
		if char >= 'A' && char <= 'Z' {
			continue
		} else {
			toLowerIndex = index
			break
		}
	}
	if toLowerIndex > 1 {
		toLowerIndex = toLowerIndex - 1
	}
	formattedString := strings.ToLower(trimedString[0:toLowerIndex]) + trimedString[toLowerIndex:]
	return formattedString
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

	typetreeFile := ""

	for _, sceneAbLockJsonFile := range sceneAbLockJson["files"].Array() {
		splittedPath := strings.Split(sceneAbLockJsonFile.Str, "/")
		fileName := splittedPath[len(splittedPath)-1]

		if strings.Contains(fileName, ".ab.json") {
			typetreeFile = sceneAbLockJsonFile.Str
		}
	}

	// load typetreeFile
	if typetreeFile == "" {
		return nil, nil, fmt.Errorf("no typetree found")
	}
	typetreeFileJsonResult, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/" + typetreeFile)
	if err != nil {
		return nil, nil, err
	}

	typetreeFileJsonResultMap := typetreeFileJsonResult.Map()

	preloadDataFileKey := ""

	meshRendererFileKeys := []string{}

	for key := range typetreeFileJsonResultMap {
		if strings.Contains(key, "MeshRenderer") {
			meshRendererFileKeys = append(meshRendererFileKeys, key)
		} else if strings.Contains(key, "PreloadData") {
			preloadDataFileKey = key
		}
	}

	if preloadDataFileKey == "" {
		return nil, nil, fmt.Errorf("no preloadDataFile found")
	}
	preloadDataFileJsonResult := typetreeFileJsonResultMap[preloadDataFileKey]

	// load all resource references in preloadDataFile and corresponding typetrees
	typetreesInPreloadDataFile := map[string]*gjson.Result{}
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
			// load typetree
			if strings.Contains(preloadDataFileJsonDependencyLockFileJsonFile.Str, ".ab.json") {
				resourceInPreloadDataFileJsonResult, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/" + preloadDataFileJsonDependencyLockFileJsonFile.Str)
				if err != nil {
					return nil, nil, err
				}
				typetreesInPreloadDataFile[preloadDataFileJsonDependencyLockFileJsonFile.Str] = resourceInPreloadDataFileJsonResult
			}
			// load resource
			resourcesInPreloadDataFile = append(resourcesInPreloadDataFile, preloadDataFileJsonDependencyLockFileJsonFile.Str)
		}
	}

	// Generate meshConfigs
	meshConfigs := make([]MeshConfig, len(meshRendererFileKeys))
	materials := map[string]MaterialConfig{}

	for _, meshRendererFileKey := range meshRendererFileKeys {
		meshRendererFileJson := typetreeFileJsonResultMap[meshRendererFileKey].Map()

		if !meshRendererFileJson["m_StaticBatchInfo"].Exists() || !meshRendererFileJson["m_StaticBatchInfo"].Map()["firstSubMesh"].Exists() {
			return nil, nil, fmt.Errorf("cannot find firstSubMesh")
		}

		firstSubMesh := meshRendererFileJson["m_StaticBatchInfo"].Map()["firstSubMesh"].Int()
		subMeshCount := meshRendererFileJson["m_StaticBatchInfo"].Map()["subMeshCount"].Int()

		for subMeshIndex := int64(0); subMeshIndex < subMeshCount; subMeshIndex++ {
			subMesh := firstSubMesh + subMeshIndex
			if int(subMesh) >= len(meshConfigs) {
				meshConfigs = append(meshConfigs, make([]MeshConfig, int(subMesh)-len(meshConfigs)+1)...)
			}

			if !meshRendererFileJson["m_LightmapTilingOffset"].Exists() {
				return nil, nil, fmt.Errorf("cannot find m_LightmapTilingOffset")
			}

			meshConfigs[subMesh].LightmapConfig = LightmapConfig{
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

			if !m_Materials[subMeshIndex].Exists() || !m_Materials[subMeshIndex].Map()["m_PathID"].Exists() {
				return nil, nil, fmt.Errorf("cannot find m_PathID")
			}

			materialPathId := m_Materials[subMeshIndex].Map()["m_PathID"].String()

			meshConfigs[subMesh].Material = materialPathId

			// texture
			if _, exists := materials[materialPathId]; !exists {
				texEnvs := make(map[string]*TexEnv)
				floats := make(map[string]*float64)
				colors := make(map[string]*Color)

				for _, typetreeInPreloadDataFile := range typetreesInPreloadDataFile {
					typetreeInPreloadDataFileMap := typetreeInPreloadDataFile.Map()
					for typetreeInPreloadDataFileKey := range typetreeInPreloadDataFileMap {
						if strings.HasPrefix(typetreeInPreloadDataFileKey, materialPathId+"_Material") {
							typetreeInPreloadDataFileJsonResult := typetreeInPreloadDataFileMap[typetreeInPreloadDataFileKey]

							if !typetreeInPreloadDataFileJsonResult.Map()["m_SavedProperties"].Exists() {
								return nil, nil, fmt.Errorf("cannot find m_SavedProperties")
							}

							materialSavedProperties := typetreeInPreloadDataFileJsonResult.Map()["m_SavedProperties"].Map()

							if !materialSavedProperties["m_TexEnvs"].Exists() {
								return nil, nil, fmt.Errorf("cannot find m_TexEnvs")
							}

							materialSavedPropertiesTexEnvs := materialSavedProperties["m_TexEnvs"].Array()

							for _, materialSavedPropertiesTexEnv := range materialSavedPropertiesTexEnvs {
								key := materialSavedPropertiesTexEnv.Array()[0].Str
								TexValue := materialSavedPropertiesTexEnv.Array()[1].Map()
								if !TexValue["m_Texture"].Exists() {
									return nil, nil, fmt.Errorf("cannot find m_Texture")
								}

								texPathId := TexValue["m_Texture"].Map()["m_PathID"].String()
								if texPathId != "0" {
									texturePath := ""

									for _, resourceInPreloadDataFile := range resourcesInPreloadDataFile {
										if strings.Contains(resourceInPreloadDataFile, texPathId+"_Texture2D") {
											texturePath = resourceInPreloadDataFile
											break
										}
									}
									texturePathUri, err := ctx.GetRouteURL("map3d.material", fiber.Map{
										"server":   ctx.Params("server"),
										"platform": ctx.Params("platform"),
										"*":        strings.Replace(strings.Replace(texturePath, ".png", "", 1), "unpacked_assetbundle/assets/torappu/dynamicassets/arts/maps/", "", 1),
									})
									if err != nil {
										return nil, nil, err
									}
									textureUri := ctx.BaseURL() + texturePathUri
									scale := XY{
										X: TexValue["m_Scale"].Get("x").Float(),
										Y: TexValue["m_Scale"].Get("y").Float(),
									}

									offset := XY{
										X: TexValue["m_Offset"].Get("x").Float(),
										Y: TexValue["m_Offset"].Get("y").Float(),
									}

									texEnv := TexEnv{
										Texture: &textureUri,
										Scale:   scale,
										Offset:  offset,
									}
									texEnvs[c.formatMaterialKey(key)] = &texEnv
								} else {
									texEnvs[c.formatMaterialKey(key)] = &TexEnv{}
								}
							}

							if !materialSavedProperties["m_Floats"].Exists() {
								return nil, nil, fmt.Errorf("cannot find m_Floats")
							}

							materialSavedPropertiesFloats := materialSavedProperties["m_Floats"].Array()

							for _, materialSavedPropertiesFloat := range materialSavedPropertiesFloats {
								key := materialSavedPropertiesFloat.Array()[0].Str
								floatValue := materialSavedPropertiesFloat.Array()[1].Float()
								floats[c.formatMaterialKey(key)] = &floatValue
							}

							if !materialSavedProperties["m_Colors"].Exists() {
								return nil, nil, fmt.Errorf("cannot find m_Colors")
							}

							materialSavedPropertiesColors := materialSavedProperties["m_Colors"].Array()

							for _, materialSavedPropertiesColor := range materialSavedPropertiesColors {
								key := materialSavedPropertiesColor.Array()[0].Str
								colorValue := materialSavedPropertiesColor.Array()[1].Map()
								colors[c.formatMaterialKey(key)] = &Color{
									R: colorValue["r"].Float(),
									G: colorValue["g"].Float(),
									B: colorValue["b"].Float(),
									A: colorValue["a"].Float(),
								}
							}
						}
					}
				}
				materials[materialPathId] = MaterialConfig{}

				if materialConfig, ok := materials[materialPathId]; ok {
					materialConfig.TexEnvs = &texEnvs
					materialConfig.Floats = &floats
					materialConfig.Colors = &colors
					materials[materialPathId] = materialConfig
				}
			}
		}
	}

	return meshConfigs, materials, nil
}

func (c *StaticMap3DController) Map3DConfig(ctx *fiber.Ctx) error {
	stageId := strings.ReplaceAll(ctx.Params("stageId"), "__", "#")

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
	if !stages[stageId].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	// stageInfo := stages[ctx.Params("stageId")].Map()
	rootSceneObjPath, err := ctx.GetRouteURL("map3d.rootScene.obj", fiber.Map{
		"server":   ctx.Params("server"),
		"platform": ctx.Params("platform"),
		"stageId":  ctx.Params("stageId"),
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

	stageInfo := stages[stageId].Map()
	levelId := stageInfo["levelId"].Str

	battleMiscTableJson, err := c.AkAbFs.NewJsonObject(staticProdVersionPath + "/unpacked_assetbundle/assets/torappu/dynamicassets/gamedata/battle/battle_misc_table.json")
	if err != nil {
		return err
	}

	if battleMiscTableJson.Get("levelScenePairs." + levelId).Exists() {
		hookedLevelId := battleMiscTableJson.Get("levelScenePairs." + levelId + ".sceneId").Str

		for stageId, stageInfo := range stages {
			if stageInfo.Get("levelId").Str == hookedLevelId {
				rootScenePath, err := ctx.GetRouteURL("map3d.rootScene.config", fiber.Map{
					"server":   ctx.Params("server"),
					"platform": ctx.Params("platform"),
					"stageId":  stageId,
				})
				if err != nil {
					return err
				}
				return ctx.Redirect(rootScenePath)
			}
		}
		return ctx.SendStatus(fiber.StatusNotFound)
	}

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
	stageId := strings.ReplaceAll(ctx.Params("stageId"), "__", "#")

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
	if !stages[stageId].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	stageInfo := stages[stageId].Map()

	levelId := stageInfo["levelId"].Str

	lowerLevelId := strings.ToLower(levelId)

	splittedLowerLevelId := strings.Split(lowerLevelId, "/")

	mapPreviewPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/scenes/%s/%s.ab/1_Mesh_Combined Mesh (root_ scene).obj", lowerLevelId, splittedLowerLevelId[len(splittedLowerLevelId)-1])

	newObject, err := c.AkAbFs.NewObject(mapPreviewPath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}
	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	newObjectIoReader, err := newObject.Open(cancelContext)
	if err != nil {
		return err
	}
	return ctx.SendStream(newObjectIoReader)
}

func (c *StaticMap3DController) Map3DRootSceneLightmap(ctx *fiber.Ctx) error {
	stageId := strings.ReplaceAll(ctx.Params("stageId"), "__", "#")

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
	if !stages[stageId].Exists() {
		return ctx.SendStatus(fiber.StatusNotFound)
	}

	stageInfo := stages[stageId].Map()

	levelId := stageInfo["levelId"].Str

	lowerLevelId := strings.ToLower(levelId)

	splittedLowerLevelId := strings.Split(lowerLevelId, "/")

	mapPreviewPath := staticProdVersionPath + fmt.Sprintf("/unpacked_assetbundle/assets/torappu/dynamicassets/scenes/%s/%s/lightmap-0_comp_light.png", lowerLevelId, splittedLowerLevelId[len(splittedLowerLevelId)-1])

	newObject, err := c.AkAbFs.NewObject(mapPreviewPath)
	if err != nil {
		return ctx.SendStatus(fiber.StatusNotFound)
	}
	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	newObjectIoReader, err := newObject.Open(cancelContext)
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
	cancelContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	newObjectIoReader, err := newObject.Open(cancelContext)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	defer buf.Reset()
	buf.ReadFrom(newObjectIoReader)

	encodedWebpBuffer, err := webpService.EncodeWebp(buf.Bytes(), 100)

	if err != nil {
		return err
	}

	ctx.Set("Content-Type", "image/webp")

	return ctx.SendStream(bytes.NewReader(encodedWebpBuffer))
}

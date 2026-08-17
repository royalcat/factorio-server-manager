package factorio

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

type ModPackMap map[string]*ModPack
type ModPack struct {
	Mods     Mods
	Metadata ModPackMetadata
	Name     string
	Path     string
}
type ModPackMetadata struct {
	Description              string `json:"description"`
	ValidatedFactorioVersion string `json:"validated_factorio_version"`
}

type ModPackResult struct {
	Name                     string         `json:"name"`
	Description              string         `json:"description"`
	ValidatedFactorioVersion string         `json:"validated_factorio_version"`
	Mods                     ModsResultList `json:"mods"`
}

type ModPackDiff struct {
	Added    []ModsResult       `json:"added"`
	Removed  []ModsResult       `json:"removed"`
	Updated  []ModPackDiffEntry `json:"updated"`
	Enabled  []ModsResult       `json:"enabled"`
	Disabled []ModsResult       `json:"disabled"`
}

type ModPackDiffEntry struct {
	Name    string     `json:"name"`
	Active  ModsResult `json:"active"`
	ModPack ModsResult `json:"mod_pack"`
}

type ModPackValidationResult struct {
	Valid  bool     `json:"valid"`
	Issues []string `json:"issues"`
}

func NewModPackMap() (ModPackMap, error) {
	var err error
	modPackMap := make(ModPackMap)

	err = modPackMap.reload()
	if err != nil {
		log.Printf("error on loading the modpacks: %s", err)
		return modPackMap, err
	}

	return modPackMap, nil
}

func newModPack(modPackFolder string) (*ModPack, error) {
	var err error
	modPack := ModPack{
		Name: filepath.Base(modPackFolder),
		Path: modPackFolder,
	}

	modPack.Mods, err = NewMods(modPackFolder)
	if err != nil {
		log.Printf("error on loading mods in mod_pack_dir: %s", err)
		return &modPack, err
	}

	modPack.Metadata, err = loadModPackMetadata(modPackFolder)
	if err != nil {
		log.Printf("error on loading mod pack metadata: %s", err)
		return &modPack, err
	}

	return &modPack, err
}

func (modPackMap *ModPackMap) reload() error {
	var err error
	newModPackMap := make(ModPackMap)
	config := bootstrap.GetConfig()

	err = filepath.Walk(config.FactorioModPackDir, func(path string, info os.FileInfo, err error) error {
		if path == config.FactorioModPackDir || !info.IsDir() {
			return nil
		}

		modPackName := filepath.Base(path)

		newModPackMap[modPackName], err = newModPack(path)
		if err != nil {
			log.Printf("error on creating newModPack: %s", err)
			return err
		}

		return nil
	})
	if err != nil {
		log.Printf("error on walking over the ModDir: %s", err)
		return err
	}

	*modPackMap = newModPackMap

	return nil
}

func (modPackMap *ModPackMap) ListInstalledModPacks() []ModPackResult {
	list := make([]ModPackResult, 0)

	for modPackName, modPack := range *modPackMap {
		var modPackResult ModPackResult
		modPackResult.Name = modPackName
		modPackResult.Description = modPack.Metadata.Description
		modPackResult.ValidatedFactorioVersion = modPack.Metadata.ValidatedFactorioVersion
		modPackResult.Mods = modPack.Mods.ListInstalledMods()

		list = append(list, modPackResult)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}

func (modPackMap *ModPackMap) CreateModPack(modPackName string, description string) error {
	var err error
	if err := validateModPackName(modPackName); err != nil {
		return err
	}
	config := bootstrap.GetConfig()
	modPackFolder := filepath.Join(config.FactorioModPackDir, modPackName)

	if modPackMap.CheckModPackExists(modPackName) == true {
		log.Printf("ModPack %s already existis", modPackName)
		return errors.New("ModPack " + modPackName + " already exists, please choose a different name")
	}

	sourceFileInfo, err := os.Stat(config.FactorioModsDir)
	if err != nil {
		log.Printf("error when reading factorioModsDir. %s", err)
		return err
	}

	//Create the modPack-folder
	err = os.MkdirAll(modPackFolder, sourceFileInfo.Mode())
	if err != nil {
		log.Printf("error on creating the new ModPack directory: %s", err)
		return err
	}

	files, err := ioutil.ReadDir(config.FactorioModsDir)
	if err != nil {
		log.Printf("error on reading the factorio mods dir: %s", err)
		return err
	}

	for _, file := range files {
		if file.IsDir() == false {
			sourceFilepath := filepath.Join(config.FactorioModsDir, file.Name())
			destinationFilepath := filepath.Join(modPackFolder, file.Name())

			sourceFile, err := os.Open(sourceFilepath)
			if err != nil {
				log.Printf("error on opening sourceFilepath: %s", err)
				return err
			}
			defer sourceFile.Close()

			destinationFile, err := os.Create(destinationFilepath)
			if err != nil {
				log.Printf("error on creating destinationFilepath: %s", err)
				return err
			}
			defer destinationFile.Close()

			_, err = io.Copy(destinationFile, sourceFile)
			if err != nil {
				log.Printf("error on copying data from source to destination: %s", err)
				return err
			}

			sourceFile.Close()
			destinationFile.Close()
		}
	}

	err = saveModPackMetadata(modPackFolder, ModPackMetadata{
		Description:              description,
		ValidatedFactorioVersion: GetFactorioServer().BaseModVersion,
	})
	if err != nil {
		log.Printf("error saving mod pack metadata: %s", err)
		return err
	}

	//reload the ModPackList
	err = modPackMap.reload()
	if err != nil {
		log.Printf("error reloading ModPack: %s", err)
		return err
	}

	return nil
}

func (modPackMap *ModPackMap) CreateEmptyModPack(packName string) error {
	var err error
	if err := validateModPackName(packName); err != nil {
		return err
	}
	config := bootstrap.GetConfig()
	modPackFolder := filepath.Join(config.FactorioModPackDir, packName)

	if modPackMap.CheckModPackExists(packName) == true {
		log.Printf("ModPack %s already existis", packName)
		return errors.New("ModPack " + packName + " already exists, please choose a different name")
	}

	// Create the modPack-folder
	err = os.MkdirAll(modPackFolder, 0777)
	if err != nil {
		log.Printf("error creating the new ModPack directory: %s", err)
		return err
	}

	err = modPackMap.reload()
	if err != nil {
		log.Printf("error reloading ModPack: %s", err)
		return err
	}
	return nil
}

func (modPackMap *ModPackMap) CheckModPackExists(modPackName string) bool {
	for modPackId := range *modPackMap {
		if modPackId == modPackName {
			return true
		}
	}

	return false
}

func (modPackMap *ModPackMap) DeleteModPack(modPackName string) error {
	var err error
	config := bootstrap.GetConfig()
	modPackDir := filepath.Join(config.FactorioModPackDir, modPackName)

	err = os.RemoveAll(modPackDir)
	if err != nil {
		log.Printf("error on removing the ModPack: %s", err)
		return err
	}

	err = modPackMap.reload()
	if err != nil {
		log.Printf("error on reloading the ModPackList: %s", err)
		return err
	}

	return nil
}

func (modPackMap *ModPackMap) CloneModPack(sourceName string, targetName string, description string) error {
	if err := validateModPackName(targetName); err != nil {
		return err
	}
	if modPackMap.CheckModPackExists(targetName) {
		return errors.New("ModPack " + targetName + " already exists, please choose a different name")
	}

	config := bootstrap.GetConfig()
	sourceDir := filepath.Join(config.FactorioModPackDir, sourceName)
	targetDir := filepath.Join(config.FactorioModPackDir, targetName)

	if err := copyDir(sourceDir, targetDir); err != nil {
		return err
	}

	metadata := (*modPackMap)[sourceName].Metadata
	if description != "" {
		metadata.Description = description
	}
	if err := saveModPackMetadata(targetDir, metadata); err != nil {
		return err
	}

	return modPackMap.reload()
}

func (modPackMap *ModPackMap) RenameModPack(oldName string, newName string) error {
	if err := validateModPackName(newName); err != nil {
		return err
	}
	if modPackMap.CheckModPackExists(newName) {
		return errors.New("ModPack " + newName + " already exists, please choose a different name")
	}

	config := bootstrap.GetConfig()
	oldDir := filepath.Join(config.FactorioModPackDir, oldName)
	newDir := filepath.Join(config.FactorioModPackDir, newName)

	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}

	return modPackMap.reload()
}

func (modPack *ModPack) UpdateMetadata(metadata ModPackMetadata) error {
	modPack.Metadata = metadata
	return saveModPackMetadata(modPack.Path, metadata)
}

func (modPack *ModPack) DiffAgainstActiveMods() (ModPackDiff, error) {
	config := bootstrap.GetConfig()
	activeMods, err := NewMods(config.FactorioModsDir)
	if err != nil {
		return ModPackDiff{}, err
	}

	return DiffMods(activeMods.ListInstalledMods(), modPack.Mods.ListInstalledMods()), nil
}

func (modPack *ModPack) Validate() ModPackValidationResult {
	issues := modPack.Mods.ValidateEnabledDependencies()
	return ModPackValidationResult{
		Valid:  len(issues) == 0,
		Issues: issues,
	}
}

func DiffMods(active ModsResultList, modPack ModsResultList) ModPackDiff {
	diff := ModPackDiff{
		Added:    make([]ModsResult, 0),
		Removed:  make([]ModsResult, 0),
		Updated:  make([]ModPackDiffEntry, 0),
		Enabled:  make([]ModsResult, 0),
		Disabled: make([]ModsResult, 0),
	}

	activeByName := mapModsByName(active.ModsResult)
	packByName := mapModsByName(modPack.ModsResult)

	for name, packMod := range packByName {
		activeMod, exists := activeByName[name]
		if !exists {
			diff.Added = append(diff.Added, packMod)
			continue
		}

		if activeMod.Version != packMod.Version {
			diff.Updated = append(diff.Updated, ModPackDiffEntry{Name: name, Active: activeMod, ModPack: packMod})
		}
		if !activeMod.Enabled && packMod.Enabled {
			diff.Enabled = append(diff.Enabled, packMod)
		}
		if activeMod.Enabled && !packMod.Enabled {
			diff.Disabled = append(diff.Disabled, packMod)
		}
	}

	for name, activeMod := range activeByName {
		if _, exists := packByName[name]; !exists {
			diff.Removed = append(diff.Removed, activeMod)
		}
	}

	sortModsResult(diff.Added)
	sortModsResult(diff.Removed)
	sortModsResult(diff.Enabled)
	sortModsResult(diff.Disabled)
	sort.Slice(diff.Updated, func(i, j int) bool {
		return diff.Updated[i].Name < diff.Updated[j].Name
	})

	return diff
}

func (modPack *ModPack) LoadModPack() error {
	var err error
	validation := modPack.Validate()
	if !validation.Valid {
		return fmt.Errorf("mod pack validation failed: %v", validation.Issues)
	}

	config := bootstrap.GetConfig()
	//clean factorio mod directory
	if err = clearDirectoryContents(config.FactorioModsDir); err != nil {
		log.Printf("error on clearing the factorio mods dir: %s", err)
		return err
	}

	//copy the modpack folder to the normal mods directory
	err = filepath.Walk(modPack.Mods.ModInfoList.Destination, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if info.Name() == modPackMetadataFile {
			return nil
		}
		newFile, err := os.Create(filepath.Join(config.FactorioModsDir, info.Name()))
		if err != nil {
			log.Printf("error on creting mod file: %s", err)
			return err
		}
		defer newFile.Close()

		oldFile, err := os.Open(path)
		if err != nil {
			log.Printf("error on opening modFile: %s", err)
			return err
		}
		defer oldFile.Close()

		_, err = io.Copy(newFile, oldFile)
		if err != nil {
			log.Printf("error on copying data to the new file: %s", err)
			return err
		}

		return nil
	})
	if err != nil {
		log.Printf("error on copying the mod pack: %s", err)
		return err
	}

	modPack.Metadata.ValidatedFactorioVersion = GetFactorioServer().BaseModVersion
	if err := modPack.UpdateMetadata(modPack.Metadata); err != nil {
		return err
	}

	return nil
}

func (modPackMap *ModPackMap) ImportModPack(packName string, archive io.ReaderAt, size int64) error {
	if err := validateModPackName(packName); err != nil {
		return err
	}
	if modPackMap.CheckModPackExists(packName) {
		return errors.New("ModPack " + packName + " already exists, please choose a different name")
	}

	config := bootstrap.GetConfig()
	targetDir := filepath.Join(config.FactorioModPackDir, packName)
	if err := os.MkdirAll(targetDir, 0777); err != nil {
		return err
	}

	reader, err := zip.NewReader(archive, size)
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return err
	}

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		name := filepath.Base(file.Name)
		if name == "." || name == string(filepath.Separator) {
			continue
		}

		if filepath.Ext(name) != ".zip" && name != "mod-list.json" && name != "mod-settings.dat" && name != modPackMetadataFile {
			continue
		}

		if err := unzipFile(file, filepath.Join(targetDir, name)); err != nil {
			_ = os.RemoveAll(targetDir)
			return err
		}
	}

	if _, err := os.Stat(filepath.Join(targetDir, modPackMetadataFile)); os.IsNotExist(err) {
		if err := saveModPackMetadata(targetDir, ModPackMetadata{}); err != nil {
			_ = os.RemoveAll(targetDir)
			return err
		}
	}

	return modPackMap.reload()
}

const modPackMetadataFile = "mod-pack.json"

func loadModPackMetadata(modPackFolder string) (ModPackMetadata, error) {
	var metadata ModPackMetadata
	path := filepath.Join(modPackFolder, modPackMetadataFile)

	file, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return metadata, nil
	}
	if err != nil {
		return metadata, err
	}

	if err := json.Unmarshal(file, &metadata); err != nil {
		return metadata, err
	}

	return metadata, nil
}

func saveModPackMetadata(modPackFolder string, metadata ModPackMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(modPackFolder, modPackMetadataFile), data, 0664)
}

func copyDir(sourceDir string, targetDir string) error {
	sourceFileInfo, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, sourceFileInfo.Mode()); err != nil {
		return err
	}

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		sourceFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer sourceFile.Close()

		destinationFile, err := os.Create(filepath.Join(targetDir, info.Name()))
		if err != nil {
			return err
		}
		defer destinationFile.Close()

		_, err = io.Copy(destinationFile, sourceFile)
		return err
	})
}

func unzipFile(file *zip.File, destination string) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func mapModsByName(mods []ModsResult) map[string]ModsResult {
	result := make(map[string]ModsResult)
	for _, mod := range mods {
		result[mod.Name] = mod
	}
	return result
}

func sortModsResult(mods []ModsResult) {
	sort.Slice(mods, func(i, j int) bool {
		return mods[i].Name < mods[j].Name
	})
}

func validateModPackName(name string) error {
	if name == "" || filepath.Base(name) != name || strings.Contains(name, "\\") || strings.Contains(name, "\x00") || name == "." || name == ".." {
		return errors.New("mod pack name must be a simple folder name")
	}

	return nil
}

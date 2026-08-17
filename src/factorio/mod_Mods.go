package factorio

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/lockfile"
)

type Mods struct {
	ModSimpleList ModSimpleList `json:"mod_simple_list"`
	ModInfoList   ModInfoList   `json:"mod_info_list"`
}
type ModsResult struct {
	ModInfo
	Enabled bool `json:"enabled"`
}
type ModsResultList struct {
	ModsResult []ModsResult `json:"mods"`
}

var FileLock lockfile.FileLock = lockfile.NewLock()

func NewMods(destination string) (Mods, error) {
	var err error
	var mods Mods

	mods.ModSimpleList, err = newModSimpleList(destination)
	if err != nil {
		log.Printf("error on creating newModSimpleList: %s", err)
		return mods, err
	}

	mods.ModInfoList, err = newModInfoList(destination)
	if err != nil {
		log.Printf("error on creating newModInfoList: %s", err)
		return mods, err
	}

	return mods, nil
}

func (mods *Mods) ListInstalledMods() ModsResultList {
	result := ModsResultList{make([]ModsResult, 0)}

	for _, modInfo := range mods.ModInfoList.Mods {
		var modsResult ModsResult
		modsResult.Name = modInfo.Name
		modsResult.FileName = modInfo.FileName
		modsResult.Author = modInfo.Author
		modsResult.Title = modInfo.Title
		modsResult.Version = modInfo.Version
		modsResult.FactorioVersion = modInfo.FactorioVersion
		modsResult.Compatibility = modInfo.Compatibility

		for _, simpleMod := range mods.ModSimpleList.Mods {
			if simpleMod.Name == modsResult.Name {
				modsResult.Enabled = simpleMod.Enabled
				break
			}
		}

		result.ModsResult = append(result.ModsResult, modsResult)
	}

	return result
}

func (mods *Mods) DeleteMod(modName string) error {
	var err error

	err = mods.ModInfoList.deleteMod(modName)
	if err != nil {
		log.Printf("error when deleting mod in ModInfoList: %s", err)
		return err
	}

	err = mods.ModSimpleList.deleteMod(modName)
	if err != nil {
		log.Printf("error when deleting mod in ModSimpleList: %s", err)
		return err
	}

	return nil
}

func (mods *Mods) DeleteModWithDependencyCheck(modName string) error {
	if err := mods.ValidateModCanDisable(modName); err != nil {
		return err
	}

	return mods.DeleteMod(modName)
}

func (mods *Mods) ToggleModWithDependencyCheck(modName string) (error, bool) {
	enabled, found := mods.isModEnabled(modName)
	if !found {
		return errors.New("mod is not installed"), false
	}

	if enabled {
		if err := mods.ValidateModCanDisable(modName); err != nil {
			return err, enabled
		}
	} else if err := mods.ValidateModCanEnable(modName); err != nil {
		return err, enabled
	}

	return mods.ModSimpleList.ToggleMod(modName)
}

func (mods *Mods) ValidateModCanEnable(modName string) error {
	modInfo, ok := mods.modInfoByName(modName)
	if !ok {
		return fmt.Errorf("mod %s is not installed", modName)
	}

	missing := make([]string, 0)
	for _, dependency := range requiredDependencyNames(modInfo.Dependencies) {
		if _, ok := mods.modInfoByName(dependency); !ok {
			missing = append(missing, dependency+" (not installed)")
			continue
		}
		if enabled, ok := mods.isModEnabled(dependency); !ok || !enabled {
			missing = append(missing, dependency+" (disabled)")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("cannot enable mod %s because required dependencies are unavailable: %s", modName, strings.Join(missing, ", "))
	}

	return nil
}

func (mods *Mods) ValidateModCanDisable(modName string) error {
	if _, ok := mods.modInfoByName(modName); !ok {
		return fmt.Errorf("mod %s is not installed", modName)
	}

	dependents := make([]string, 0)
	for _, modInfo := range mods.ModInfoList.Mods {
		if modInfo.Name == modName {
			continue
		}
		enabled, ok := mods.isModEnabled(modInfo.Name)
		if !ok || !enabled {
			continue
		}

		for _, dependency := range requiredDependencyNames(modInfo.Dependencies) {
			if dependency == modName {
				dependents = append(dependents, modInfo.Name)
				break
			}
		}
	}

	if len(dependents) > 0 {
		return fmt.Errorf("cannot disable or delete mod %s because enabled mods depend on it: %s", modName, strings.Join(dependents, ", "))
	}

	return nil
}

func (mods *Mods) ValidateEnabledDependencies() []string {
	issues := make([]string, 0)

	for _, modInfo := range mods.ModInfoList.Mods {
		enabled, ok := mods.isModEnabled(modInfo.Name)
		if !ok || !enabled {
			continue
		}

		for _, dependency := range requiredDependencyNames(modInfo.Dependencies) {
			if _, ok := mods.modInfoByName(dependency); !ok {
				issues = append(issues, fmt.Sprintf("%s requires %s, but it is not installed", modInfo.Name, dependency))
				continue
			}
			if dependencyEnabled, ok := mods.isModEnabled(dependency); !ok || !dependencyEnabled {
				issues = append(issues, fmt.Sprintf("%s requires %s, but it is disabled", modInfo.Name, dependency))
			}
		}

		if !modInfo.Compatibility {
			issues = append(issues, fmt.Sprintf("%s is not compatible with the installed Factorio version", modInfo.Name))
		}
	}

	return issues
}

func (mods *Mods) modInfoByName(modName string) (ModInfo, bool) {
	for _, modInfo := range mods.ModInfoList.Mods {
		if modInfo.Name == modName {
			return modInfo, true
		}
	}

	return ModInfo{}, false
}

func (mods *Mods) isModEnabled(modName string) (bool, bool) {
	for _, mod := range mods.ModSimpleList.Mods {
		if mod.Name == modName {
			return mod.Enabled, true
		}
	}

	return false, false
}

func requiredDependencyNames(dependencies []string) []string {
	required := make([]string, 0)

	for _, dependency := range dependencies {
		name, ok := requiredDependencyName(dependency)
		if ok {
			required = append(required, name)
		}
	}

	return required
}

var builtInMods = map[string]struct{}{
	"base":           {},
	"elevated-rails": {},
	"quality":        {},
	"space-age":      {},
}

func IsBuiltInMod(modName string) bool {
	_, ok := builtInMods[modName]
	return ok
}

var modPrefixes = []string{"!", "?", "+", "(?)", "~"}
var modNonRequiredPrefixes = []string{"?", "!", "(?)"}
var modVersionEqualityOperators = []string{"<", "<=", "=", ">=", ">"}

func requiredDependencyName(dependency string) (string, bool) {
	// There are multiple edge conditions that need to be handled
	// * Mod name CAN have spaces in name
	// * Dependency string MAY OR MAY NOT have spaces between prefix, name and version

	dependency = strings.TrimSpace(dependency)
	for _, prefix := range modNonRequiredPrefixes {
		if strings.HasPrefix(dependency, prefix) {
			return "", false
		}
	}

	name := dependency
	for _, prefix := range modPrefixes {
		name = strings.TrimPrefix(name, prefix)
	}
	for _, operator := range modVersionEqualityOperators {
		name, _, _ = strings.Cut(name, operator)
	}
	name = strings.TrimSpace(name)

	if IsBuiltInMod(name) {
		return "", false
	}

	return name, true
}

func (mods *Mods) createMod(modName string, fileName string, fileRc io.Reader) error {
	var err error

	var oldFileName string
	for _, mod := range mods.ModInfoList.Mods {
		if mod.Name == modName {
			oldFileName = mod.FileName
			break
		}
	}

	err = mods.ModInfoList.createMod(modName, fileName, fileRc)
	if err != nil {
		log.Printf("error on creating mod-file: %s", err)
		return err
	}

	if oldFileName != "" && oldFileName != fileName {
		oldPath := filepath.Join(mods.ModInfoList.Destination, oldFileName)
		FileLock.LockW(oldPath)
		err = os.Remove(oldPath)
		FileLock.Unlock(oldPath)
		if err != nil && !os.IsNotExist(err) {
			log.Printf("error removing old mod file %s: %s", oldPath, err)
		}
		mods.ModInfoList.listInstalledMods()
	}

	if !mods.ModSimpleList.CheckModExists(modName) {
		err = mods.ModSimpleList.createMod(modName)
		if err != nil {
			log.Printf("error creating mod in modSimpleList: %s", err)
			return err
		}
	}

	return nil
}

func (mods *Mods) DownloadMod(url string, filename string, modId string) error {
	var err error

	var credentials Credentials
	status, err := credentials.Load()
	if err != nil {
		log.Printf("error loading credentials: %s", err)
		return err
	}
	if status == false {
		log.Printf("error: credentials are invalid")
		return errors.New("error: credentials are invalid")
	}

	//download the mod from the mod portal api
	completeUrl := modPortalBaseURL + url + "?username=" + credentials.Username + "&token=" + credentials.Userkey

	response, err := http.Get(completeUrl)
	if err != nil {
		log.Printf("error on downloading mod: %s", err)
		return err
	}

	log.Printf("download complete\n StatusCode: %d\n Status: %s", response.StatusCode, response.Status)

	defer response.Body.Close()

	if response.StatusCode != 200 {
		log.Printf("StatusCode: %d", response.StatusCode)

		return errors.New("Statuscode not 200: " + fmt.Sprint(response.StatusCode))
	}

	err = mods.createMod(modId, filename, response.Body)
	if err != nil {
		log.Printf("error when creating Mod: %s", err)
		return err
	}

	log.Printf("completed copying the response.Body")

	//done everything is made inside the createMod

	return nil
}

func (mods *Mods) UploadMod(file multipart.File, header *multipart.FileHeader) error {
	var err error

	if filepath.Ext(header.Filename) != ".zip" {
		log.Print("The uploaded file wasn't a zip-file")
		return errors.New("the uploaded file wasn't a zip-file")
	}

	fileByteArray, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("error reading file: %s", err)
		return err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(fileByteArray), int64(len(fileByteArray)))
	if err != nil {
		log.Printf("Uploaded file could not put into zip.Reader: %s", err)
		return err
	}

	var modInfo ModInfo
	err = modInfo.getModInfo(zipReader)
	if err != nil {
		log.Printf("Error in getModInfo: %s", err)
		return err
	}

	err = mods.createMod(modInfo.Name, header.Filename, bytes.NewReader(fileByteArray))
	if err != nil {
		log.Printf("error on creating Mod: %s", err)
		return err
	}

	return nil
}

func (mods *Mods) UpdateMod(modName string, url string, filename string) error {
	var err error

	err = mods.DownloadMod(url, filename, modName)
	if err != nil {
		log.Printf("updateMod ... error when downloading the new Mod: %s", err)
		return err
	}

	return nil
}

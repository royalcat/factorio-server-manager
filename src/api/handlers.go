package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
	"github.com/OpenFactorioServerManager/factorio-server-manager/src/factorio"
	"github.com/gorilla/sessions"

	"github.com/gorilla/mux"
)

const readHttpBodyError = "Could not read the Request Body."

type JSONResponseFileInput struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,string"`
	Error     string      `json:"error"`
	ErrorKeys []int       `json:"errorkeys"`
}

type NetworkInterfaceAddress struct {
	IP      string `json:"ip"`
	CIDR    string `json:"cidr"`
	Version string `json:"version"`
}

type NetworkInterface struct {
	Name        string                    `json:"name"`
	DisplayName string                    `json:"display_name"`
	Addresses   []NetworkInterfaceAddress `json:"addresses"`
}

func WriteResponse(w http.ResponseWriter, data interface{}) {
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error writing response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func ReadRequestBody(w http.ResponseWriter, r *http.Request) (body []byte, resp interface{}, err error) {
	if r.Body == nil {
		resp = fmt.Sprintf("%s: no request body", readHttpBodyError)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		err = errors.New("no request body")
		return
	}

	body, err = ioutil.ReadAll(r.Body)
	if err != nil {
		resp = fmt.Sprintf("%s: %s", readHttpBodyError, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

func ReadSessionStore(w http.ResponseWriter, r *http.Request, name string) (session *sessions.Session, resp interface{}, err error) {
	session, err = sessionStore.Get(r, name)
	if err != nil {
		resp = fmt.Sprintf("Error reading session cookie [%s]: %s", name, err)
		log.Println(resp)
		if session != nil {
			session.Options.MaxAge = -1
			err2 := session.Save(r, w)
			if err2 != nil {
				log.Printf("Error deleting session cookie: %s", err2)
			}
		}
		w.WriteHeader(http.StatusUnauthorized)
	}
	return
}

func SaveSession(w http.ResponseWriter, r *http.Request, session *sessions.Session) (resp interface{}, err error) {
	err = session.Save(r, w)
	if err != nil {
		resp = fmt.Sprintf("Error saving session cookie: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
	}
	return
}

// Lists all save files in the factorio/saves directory
func ListSaves(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	latestParam := r.URL.Query().Get("latest")

	var withLatest bool

	if latestParam != "" {
		var err error
		withLatest, err = strconv.ParseBool(latestParam)
		if err != nil {
			resp = fmt.Sprintf("Error parsing latestParam: %s", err)
			log.Println(resp)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	savesList, err := factorio.ListSaves()
	if err != nil {
		resp = fmt.Sprintf("Error listing save files: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// get actual latest and add name
	// but only if requested
	if withLatest && len(savesList) != 0 {
		latestSave, err := factorio.GetLatestSave()
		if err != nil {
			resp = fmt.Sprintf("Error getting latest save: %s", err)
			log.Println(resp)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		latestSave.Name = fmt.Sprintf("Load Latest (%s)", latestSave.Name)
		savesList = append(savesList, latestSave)
	}

	resp = savesList
}

func DLSave(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	config := bootstrap.GetConfig()
	vars := mux.Vars(r)
	save, err := factorio.ValidateSaveName(vars["save"])
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid save name: %s", err), http.StatusBadRequest)
		return
	}
	saveName := filepath.Join(config.FactorioSavesDir, save)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", save))
	log.Printf("%s downloading: %s", r.Host, saveName)

	http.ServeFile(w, r, saveName)
}

func UploadSave(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	log.Println("Uploading save file")

	r.ParseMultipartForm(32 << 20)
	config := bootstrap.GetConfig()

	for _, saveFile := range r.MultipartForm.File["savefile"] {
		fileName, err := factorio.ValidateSaveName(saveFile.Filename)
		if err != nil {
			resp = fmt.Sprintf("Invalid save file name: %s", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ext := filepath.Ext(saveFile.Filename)
		if ext != ".zip" {
			// Only zip-files allowed
			resp = fmt.Sprintf("Fileformat {%s} is not allowed", ext)
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		file, err := saveFile.Open()
		if err != nil {
			resp = fmt.Sprintf("Error opening uploaded saveFile: %s", err)
			log.Println(resp)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer file.Close()

		out, err := os.Create(filepath.Join(config.FactorioSavesDir, fileName))
		if err != nil {
			resp = fmt.Sprintf("Error creating new savefile to copy uploaded on to: %s", err)
			log.Println(resp)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			resp = fmt.Sprintf("Error coping uploaded file to created file on disk: %s", err)
			log.Println(resp)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	resp = "Uploading files successful"
}

// Deletes provided save
func RemoveSave(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	vars := mux.Vars(r)
	name := vars["save"]

	save, err := factorio.FindSave(name)
	if err != nil {
		resp = fmt.Sprintf("Error finding save {%s}: %s", name, err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = save.Remove()
	if err != nil {
		resp = fmt.Sprintf("Error removing save {%s}: %s", name, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// save was removed
	resp = fmt.Sprintf("Removed save: %s", save.Name)
}

// Launches Factorio server binary with --create flag to create save
// Url must include save name for creation of savefile
func CreateSaveHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	vars := mux.Vars(r)
	saveName := vars["save"]

	saveName, err = factorio.ValidateSaveName(saveName)
	if err != nil {
		resp = fmt.Sprintf("Error creating save: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	config := bootstrap.GetConfig()
	saveFile := filepath.Join(config.FactorioSavesDir, saveName)
	cmdOut, err := factorio.CreateSave(saveFile)
	if err != nil {
		resp = fmt.Sprintf("Error creating save {%s}: %s", saveName, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = fmt.Sprintf("Save %s created successfully. Command output: \n%s", saveName, cmdOut)
}

func FreshRestartSave(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	vars := mux.Vars(r)
	saveName := vars["save"]

	saveName, err := factorio.ValidateSaveName(saveName)
	if err != nil {
		resp = fmt.Sprintf("Error validating save name: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	server := factorio.GetFactorioServer()

	if !server.GetRunning() {
		resp = "Server must be running to extract map settings via RCON"
		w.WriteHeader(http.StatusConflict)
		return
	}

	if server.Rcon == nil {
		resp = "RCON connection not available"
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Backup the save BEFORE any RCON commands, because /silent-command disables
	// achievements for the running save. Backing up first ensures the backup
	// captures a clean, achievement-eligible state.
	backup, err := factorio.BackupSave(saveName)
	if err != nil {
		resp = fmt.Sprintf("Error backing up save: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Only allow fresh restart on the currently running save — RCON settings
	// extraction is only meaningful for the active save, and the server will
	// restart on the new generated save.
	if server.Savefile != saveName {
		resp = fmt.Sprintf("Save %q is not the currently running save (%q). Fresh restart is only available for the active save.", saveName, server.Savefile)
		log.Println(resp)
		w.WriteHeader(http.StatusConflict)
		return
	}

	mapGenSettingsFile, mapSettingsFile, err := factorio.ExtractMapGenSettings(server)
	if err != nil {
		resp = fmt.Sprintf("Error extracting map settings: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	lifecycle, _ := factorio.LoadLifecycleConfig()
	stopErr := server.StopWithTimeout(lifecycle.GracefulStopTimeout)
	if stopErr != nil {
		resp = fmt.Sprintf("Error stopping server: %s", stopErr)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	config := bootstrap.GetConfig()
	baseName := saveName
	if strings.HasSuffix(baseName, ".zip") {
		baseName = baseName[:len(baseName)-4]
	}
	timestamp := time.Now().Format("20060102-150405")
	newSaveName := fmt.Sprintf("%s-fresh-%s.zip", baseName, timestamp)
	newSavePath := filepath.Join(config.FactorioSavesDir, newSaveName)

	_, createErr := factorio.CreateSaveWithSettings(newSavePath, mapGenSettingsFile, mapSettingsFile)
	if createErr != nil {
		resp = fmt.Sprintf("Error creating new save: %s. Server is stopped — you can restart manually.", createErr)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	scriptOutputDir := filepath.Join(config.FactorioDir, "script-output")
	os.Remove(filepath.Join(scriptOutputDir, "fsm-parsed-settings.json"))
	os.Remove(filepath.Join(scriptOutputDir, "fsm-map-gen-settings.json"))
	os.Remove(filepath.Join(scriptOutputDir, "fsm-map-settings.json"))

	lifecycle.StartupProfile.Savefile = newSaveName
	factorio.SaveLifecycleConfig(lifecycle)
	factorio.AppendLifecycleEvent("fresh-restart", fmt.Sprintf("Fresh restart: created new save %s from map settings of %s", newSaveName, saveName))

	server.Savefile = newSaveName
	go func() {
		if err := server.Run(); err != nil {
			log.Printf("Error starting server after fresh restart: %s", err)
		}
	}()

	resp = map[string]interface{}{
		"status":        "success",
		"new_save_name": newSaveName,
		"backup_name":   backup.Name,
		"original_save": saveName,
	}
}

func ListSaveBackups(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	respBackups, err := factorio.ListSaveBackups()
	if err != nil {
		resp = fmt.Sprintf("Error listing save backups: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = respBackups
}

func BackupSave(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	vars := mux.Vars(r)
	backup, err := factorio.BackupSave(vars["save"])
	if err != nil {
		resp = fmt.Sprintf("Error backing up save {%s}: %s", vars["save"], err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = backup
}

func GetSaveBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	schedule, err := factorio.LoadSaveBackupSchedule()
	if err != nil {
		resp = fmt.Sprintf("Error loading save backup schedule: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = schedule
}

func UpdateSaveBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	var schedule factorio.SaveBackupSchedule
	resp, err := ReadFromRequestBody(w, r, &schedule)
	if err != nil {
		return
	}

	schedule, err = factorio.SaveBackupScheduleConfig(schedule)
	if err != nil {
		resp = fmt.Sprintf("Error saving save backup schedule: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp = schedule
}

func RunSaveBackupSchedule(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	schedule, backups, err := factorio.RunScheduledSaveBackup()
	if err != nil {
		resp = fmt.Sprintf("Error running save backup schedule: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = struct {
		Schedule factorio.SaveBackupSchedule `json:"schedule"`
		Backups  []factorio.SaveBackup       `json:"backups"`
	}{
		Schedule: schedule,
		Backups:  backups,
	}
}

func RestoreSave(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	var restoreRequest struct {
		BackupName string `json:"backup_name"`
		TargetName string `json:"target_name"`
	}
	resp, err := ReadFromRequestBody(w, r, &restoreRequest)
	if err != nil {
		return
	}

	save, err := factorio.RestoreSave(restoreRequest.BackupName, restoreRequest.TargetName)
	if err != nil {
		resp = fmt.Sprintf("Error restoring save backup {%s}: %s", restoreRequest.BackupName, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = save
	factorio.AppendLifecycleEvent("restore", fmt.Sprintf("Restored save backup %s to %s", restoreRequest.BackupName, save.Name))
}

func RenameSave(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	var renameRequest struct {
		Name    string `json:"name"`
		NewName string `json:"new_name"`
	}
	resp, err := ReadFromRequestBody(w, r, &renameRequest)
	if err != nil {
		return
	}

	save, err := factorio.RenameSave(renameRequest.Name, renameRequest.NewName)
	if err != nil {
		resp = fmt.Sprintf("Error renaming save {%s}: %s", renameRequest.Name, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = save
}

func DuplicateSave(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	var duplicateRequest struct {
		Name    string `json:"name"`
		NewName string `json:"new_name"`
	}
	resp, err := ReadFromRequestBody(w, r, &duplicateRequest)
	if err != nil {
		return
	}

	save, err := factorio.DuplicateSave(duplicateRequest.Name, duplicateRequest.NewName)
	if err != nil {
		resp = fmt.Sprintf("Error duplicating save {%s}: %s", duplicateRequest.Name, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = save
}

// LogTail returns last lines of the factorio-current.log file
func LogTail(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	config := bootstrap.GetConfig()
	resp, err = factorio.TailLog()
	if err != nil {
		resp = fmt.Sprintf("Could not tail %s: %s", config.FactorioLog, err)
		return
	}
}

// LoadConfig returns JSON response of config.ini file
func LoadConfig(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	config := bootstrap.GetConfig()
	configContents, err := factorio.LoadConfig(config.FactorioConfigFile)
	if err != nil {
		resp = fmt.Sprintf("Could not retrieve config.ini: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = configContents

	log.Printf("Sent config.ini response")
}

func StartServer(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}
	var server = factorio.GetFactorioServer()
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	if server.GetRunning() {
		resp = "Factorio server is already running"
		w.WriteHeader(http.StatusConflict)
		return
	}

	log.Printf("Starting Factorio server.")

	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}

	log.Printf("Starting Factorio server with settings: %v", string(body))

	err = json.Unmarshal(body, &server)
	if err != nil {
		resp = fmt.Sprintf("Error unmarshalling server settings JSON: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	server.Installed = factorio.IsFactorioInstalled()

	// Check if savefile was submitted with request to start server.
	if server.Savefile == "" {
		resp = "Error starting Factorio server: No save file provided"
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if issues := factorio.ValidateServerStart(server.Savefile); len(issues) > 0 {
		resp = fmt.Sprintf("Error starting Factorio server: compatibility validation failed: %s", strings.Join(issues, "; "))
		log.Println(resp)
		w.WriteHeader(http.StatusConflict)
		return
	}

	go func() {
		err = server.Run()
		if err != nil {
			log.Printf("Error starting Factorio server: %+v", err)
			return
		}
	}()

	timeout := 0
	for timeout <= 3 {
		time.Sleep(1 * time.Second)
		if server.GetRunning() {
			log.Printf("Running Factorio server detected")
			break
		} else {
			log.Printf("Did not detect running Factorio server attempt: %+v", timeout)
		}

		timeout++
	}

	if server.GetRunning() == false {
		resp = fmt.Sprintf("Error starting Factorio server: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = fmt.Sprintf("Factorio server with save: %s started on port: %d", server.Savefile, server.Port)
	log.Println(resp)
}

func StopServer(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	var server = factorio.GetFactorioServer()
	if server.GetRunning() {
		lifecycle, _ := factorio.LoadLifecycleConfig()
		err := server.StopWithTimeout(lifecycle.GracefulStopTimeout)
		if err != nil {
			resp = fmt.Sprintf("Error stopping factorio server: %s", err)
			log.Println(resp)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp = fmt.Sprintf("Factorio server stopped")
		log.Println(resp)
	} else {
		resp = "Factorio server is not running"
		w.WriteHeader(http.StatusConflict)
		return
	}
}

func GetServerLifecycle(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	lifecycle, err := factorio.LoadLifecycleConfig()
	if err != nil {
		resp = fmt.Sprintf("Error loading server lifecycle config: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp = lifecycle
}

func UpdateServerLifecycle(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	var lifecycle factorio.LifecycleConfig
	resp, err := ReadFromRequestBody(w, r, &lifecycle)
	if err != nil {
		return
	}

	lifecycle, err = factorio.SaveLifecycleConfig(lifecycle)
	if err != nil {
		resp = fmt.Sprintf("Error saving server lifecycle config: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	factorio.AppendLifecycleEvent("config", "Updated server lifecycle settings")
	resp = lifecycle
}

func KillServer(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	var server = factorio.GetFactorioServer()
	if server.GetRunning() {
		err := server.Kill()
		if err != nil {
			resp = fmt.Sprintf("Error killing factorio server: %s", err)
			log.Println(resp)
			return
		}

		log.Printf("Killed Factorio server.")
		resp = fmt.Sprintf("Factorio server killed")
	} else {
		resp = "Factorio server is not running"
		w.WriteHeader(http.StatusBadRequest)
	}
}

func CheckServer(w http.ResponseWriter, r *http.Request) {
	defer func() {
		WriteResponse(w, factorio.GetFactorioServer())
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
}

func FactorioVersion(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	var server = factorio.GetFactorioServer()
	resp["version"] = server.Version.SemverString()
	resp["base_mod_version"] = factorio.SemverString(server.BaseModVersion)
}

func ServerInterfaces(w http.ResponseWriter, r *http.Request) {
	var resp interface{}
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	interfaces := []NetworkInterface{{
		Name:        "all",
		DisplayName: "All interfaces",
		Addresses: []NetworkInterfaceAddress{{
			IP:      "0.0.0.0",
			CIDR:    "0.0.0.0/0",
			Version: "ipv4",
		}},
	}}

	netInterfaces, err := net.Interfaces()
	if err != nil {
		resp = fmt.Sprintf("Error listing network interfaces: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, netInterface := range netInterfaces {
		if netInterface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := netInterface.Addrs()
		if err != nil {
			log.Printf("Error listing network interface %s addresses: %s", netInterface.Name, err)
			continue
		}

		addresses := []NetworkInterfaceAddress{}
		for _, addr := range addrs {
			ip, ipNet, err := net.ParseCIDR(addr.String())
			if err != nil || ip.To4() == nil {
				continue
			}
			addresses = append(addresses, NetworkInterfaceAddress{
				IP:      ip.String(),
				CIDR:    ipNet.String(),
				Version: "ipv4",
			})
		}

		if len(addresses) == 0 {
			continue
		}
		interfaces = append(interfaces, NetworkInterface{
			Name:        netInterface.Name,
			DisplayName: netInterface.Name,
			Addresses:   addresses,
		})
	}

	resp = interfaces
}

func FactorioInstallStatus(w http.ResponseWriter, r *http.Request) {
	defer func() {
		WriteResponse(w, factorio.GetInstallStatus())
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
}

func InstallFactorio(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	var req struct {
		Version string `json:"version"`
	}
	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		resp = fmt.Sprintf("Unable to parse the request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := factorio.InstallFactorio(req.Version); err != nil {
		resp = fmt.Sprintf("Error installing Factorio: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	status := factorio.GetInstallStatus()
	resp = status
	factorio.AppendLifecycleEvent("update", fmt.Sprintf("Installed Factorio %s", status.Version))
}

// Unmarshall the User object from the given bytearray
// This function has side effects (it will write to resp and to w, in case of an error)
func UnmarshallUserJson(body []byte, w http.ResponseWriter) (user User, resp interface{}, err error) {
	err = json.Unmarshal(body, &user)
	if err != nil {
		resp = fmt.Sprintf("Unable to parse the request body: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
	}
	return
}

// Handler for the Login
func LoginUser(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	// add resp to the response
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}

	user, resp, err := UnmarshallUserJson(body, w)
	if err != nil {
		return
	}

	log.Printf("Logging in user: %s", user.Username)

	err = auth.checkPassword(user.Username, user.Password)
	if err != nil {
		resp = fmt.Sprintf("Password for user %s wrong", user.Username)
		log.Println(resp)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	session, resp, err := ReadSessionStore(w, r, "authentication")
	if err != nil {
		return
	}

	session.Values["username"] = user.Username

	resp, err = SaveSession(w, r, session)
	if err != nil {
		return
	}

	log.Printf("User: %s, logged in successfully", user.Username)

	user.Password = ""
	resp = user
}

func LogoutUser(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	session, resp, err := ReadSessionStore(w, r, "authentication")
	if err != nil {
		return
	}

	delete(session.Values, "username")

	resp, err = SaveSession(w, r, session)
	if err != nil {
		return
	}

	resp = "User logged out successfully."
}

func GetCurrentLogin(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	// add resp to the response
	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	session, resp, err := ReadSessionStore(w, r, "authentication")
	if err != nil {
		return
	}

	username := session.Values["username"].(string)

	user, err := auth.getUser(username)
	if err != nil {
		resp = fmt.Sprintf("Error getting user: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	user.Password = ""

	resp = user
}

func ListUsers(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	users, err := auth.listUsers()
	if err != nil {
		resp = fmt.Sprintf("Error listing users: %s", err)
		log.Println(resp)
		return
	}

	resp = users
}

func AddUser(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}

	user, resp, err := UnmarshallUserJson(body, w)
	if err != nil {
		return
	}

	err = auth.addUser(user)
	if err != nil {
		resp = fmt.Sprintf("Error in adding user {%s}: %s", user.Username, err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = fmt.Sprintf("User: %s successfully added.", user.Username)
}

func RemoveUser(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}

	user, resp, err := UnmarshallUserJson(body, w)
	if err != nil {
		return
	}

	err = auth.deleteUser(user.Username)
	if err != nil {
		resp = fmt.Sprintf("Error in removing user {%s}, error: %s", user.Username, err)
		log.Println(resp)
		return
	}

	resp = fmt.Sprintf("User: %s successfully removed.", user.Username)
}

func ChangePassword(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}

	var user struct {
		OldPassword        string `json:"old_password"`
		NewPassword        string `json:"new_password"`
		NewPasswordConfirm string `json:"new_password_confirmation"`
	}
	err = json.Unmarshal(body, &user)
	if err != nil {
		resp = fmt.Sprintf("Unable to parse the request body: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// only allow to change its own password
	// get username from session cookie
	session, resp, err := ReadSessionStore(w, r, "authentication")
	if err != nil {
		return
	}

	username := session.Values["username"].(string)

	// check if password for user is correct
	err = auth.checkPassword(username, user.OldPassword)
	if err != nil {
		resp = fmt.Sprintf("Password for user %s wrong", username)
		log.Println(resp)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// only run, when confirmation correct
	if user.NewPassword != user.NewPasswordConfirm {
		resp = fmt.Sprintf("Password confirmation incorrect")
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = auth.changePassword(username, user.NewPassword)
	if err != nil {
		resp = fmt.Sprintf("Error changing password: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp = true
}

// GetServerSettings returns JSON response of server-settings.json file
func GetServerSettings(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	var server = factorio.GetFactorioServer()
	resp = server.Settings

	log.Printf("Sent server settings response")
}

func UpdateServerSettings(w http.ResponseWriter, r *http.Request) {
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	body, resp, err := ReadRequestBody(w, r)
	if err != nil {
		return
	}
	log.Printf("Received settings JSON: %s", body)
	var server = factorio.GetFactorioServer()

	// Race Condition while unmarshal possible
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		err = json.Unmarshal(body, &server.Settings)
		wg.Done()
	}()

	// Wait for unmarshal to avoid race condition
	wg.Wait()

	if err != nil {
		resp = fmt.Sprintf("Error unmarhaling server settings JSON: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	settings, err := json.MarshalIndent(&server.Settings, "", "  ")
	if err != nil {
		resp = fmt.Sprintf("Failed to marshal server settings: %s", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	config := bootstrap.GetConfig()
	err = ioutil.WriteFile(config.SettingsFile, settings, 0644)
	if err != nil {
		resp = fmt.Sprintf("Failed to save server settings: %v\n", err)
		log.Println(resp)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Printf("Saved Factorio server settings in server-settings.json")

	if (server.Version.Greater(factorio.Version{0, 17, 0})) {
		// save admins to adminJson
		admins, err := json.MarshalIndent(server.Settings["admins"], "", "  ")
		if err != nil {
			resp = fmt.Sprintf("Failed to marshal admins-Setting: %s", err)
			log.Println(resp)
			return
		}

		err = ioutil.WriteFile(config.FactorioAdminFile, admins, 0664)
		if err != nil {
			resp = fmt.Sprintf("Failed to save admins: %s", err)
			log.Println(resp)
			return
		}
	}

	resp = fmt.Sprintf("Settings successfully saved")
	factorio.AppendLifecycleEvent("config", "Updated server settings")
}

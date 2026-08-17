package factorio

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

const saveBackupDirName = "backups"

var ErrRCONNotConnected = errors.New("RCON not connected")

type Save struct {
	Name     string        `json:"name"`
	LastMod  time.Time     `json:"last_mod"`
	Size     int64         `json:"size"`
	Metadata *SaveMetadata `json:"metadata,omitempty"`
}

type SaveBackup struct {
	Name     string    `json:"name"`
	SaveName string    `json:"save_name"`
	LastMod  time.Time `json:"last_mod"`
	Size     int64     `json:"size"`
}

type SaveMetadata struct {
	MapName         string        `json:"map_name,omitempty"`
	FactorioVersion Version       `json:"factorio_version,omitempty"`
	PlayTimeTicks   uint64        `json:"play_time_ticks,omitempty"`
	Mods            []SaveModInfo `json:"mods,omitempty"`
	Error           string        `json:"error,omitempty"`
}

type SaveModInfo struct {
	Name    string  `json:"name"`
	Version Version `json:"version"`
}

func (s *Save) String() string {
	return s.Name
}

func ValidateSaveName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("save name cannot be blank")
	}
	if filepath.IsAbs(name) || filepath.Base(name) != name || strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "\x00") {
		return "", fmt.Errorf("invalid save name: %s", name)
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("invalid save name: %s", name)
	}
	return name, nil
}

func validateBackupName(name string) (string, error) {
	name, err := ValidateSaveName(name)
	if err != nil {
		return "", err
	}
	if _, saveName, err := parseBackupName(name); err != nil || saveName == "" {
		return "", fmt.Errorf("invalid backup name: %s", name)
	}
	return name, nil
}

func savePath(name string) (string, error) {
	name, err := ValidateSaveName(name)
	if err != nil {
		return "", err
	}
	config := bootstrap.GetConfig()
	return filepath.Join(config.FactorioSavesDir, name), nil
}

func saveBackupDir() string {
	config := bootstrap.GetConfig()
	return filepath.Join(config.FactorioSavesDir, saveBackupDirName)
}

func saveBackupSchedulePath() string {
	return filepath.Join(saveBackupDir(), "schedule.json")
}

func saveBackupPath(name string) (string, error) {
	name, err := validateBackupName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(saveBackupDir(), name), nil
}

func copyFile(src, dst string) error {
	return copyFileAtomic(src, dst, false)
}

func copyFileOverwrite(src, dst string) error {
	return copyFileAtomic(src, dst, true)
}

func copyFileAtomic(src, dst string, overwrite bool) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("file already exists: %s", filepath.Base(dst))
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(dst), fmt.Sprintf(".%s-*.tmp", filepath.Base(dst)))
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if err := tmpFile.Chmod(info.Mode()); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if !overwrite {
		if err := linkOrRename(tmpName, dst); err != nil {
			return err
		}
	} else if err := os.Rename(tmpName, dst); err != nil {
		return err
	}
	return syncDir(filepath.Dir(dst))
}

func linkOrRename(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("file already exists: %s", filepath.Base(dst))
		}
		return err
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func uniqueBackupName(saveName string) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s_%s_%s", time.Now().UTC().Format("20060102T150405.000000000Z"), hex.EncodeToString(suffix[:]), saveName), nil
}

func parseBackupName(name string) (createdAt string, saveName string, err error) {
	parts := strings.SplitN(name, "_", 3)
	if len(parts) == 2 {
		if parts[0] == "" || parts[1] == "" {
			return "", "", errors.New("backup name is missing timestamp or save name")
		}
		return parts[0], parts[1], nil
	}
	if len(parts) != 3 {
		return "", "", errors.New("backup name must include timestamp and save name")
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", errors.New("backup name is missing timestamp or save name")
	}
	return parts[0], parts[2], nil
}

// Lists save files in factorio/saves
func ListSaves() (saves []Save, err error) {
	config := bootstrap.GetConfig()
	saves = []Save{}
	entries, err := os.ReadDir(config.FactorioSavesDir)
	if err != nil {
		return saves, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return saves, err
		}
		metadata := readSaveMetadata(filepath.Join(config.FactorioSavesDir, info.Name()))
		saves = append(saves, Save{
			Name:     info.Name(),
			LastMod:  info.ModTime(),
			Size:     info.Size(),
			Metadata: metadata,
		})
	}

	return saves, nil
}

func ListSaveBackups() (backups []SaveBackup, err error) {
	backups = []SaveBackup{}
	entries, err := os.ReadDir(saveBackupDir())
	if errors.Is(err, os.ErrNotExist) {
		return backups, nil
	}
	if err != nil {
		return backups, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		_, saveName, err := parseBackupName(entry.Name())
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return backups, err
		}
		backups = append(backups, SaveBackup{
			Name:     info.Name(),
			SaveName: saveName,
			LastMod:  info.ModTime(),
			Size:     info.Size(),
		})
	}

	return backups, nil
}

func FindSave(name string) (*Save, error) {
	name, err := ValidateSaveName(name)
	if err != nil {
		return nil, err
	}

	saves, err := ListSaves()
	if err != nil {
		return nil, fmt.Errorf("error listing saves: %v", err)
	}

	for _, save := range saves {
		if save.Name == name {
			return &save, nil
		}
	}

	return nil, errors.New("save not found")
}

func (s *Save) Remove() error {
	path, err := savePath(s.Name)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func BackupSave(name string) (*SaveBackup, error) {
	name, err := ValidateSaveName(name)
	if err != nil {
		return nil, err
	}
	src, err := savePath(name)
	if err != nil {
		return nil, err
	}

	backupName, err := uniqueBackupName(name)
	if err != nil {
		return nil, err
	}
	dst := filepath.Join(saveBackupDir(), backupName)
	if err := copyFile(src, dst); err != nil {
		return nil, err
	}

	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	return &SaveBackup{
		Name:     backupName,
		SaveName: name,
		LastMod:  info.ModTime(),
		Size:     info.Size(),
	}, nil
}

func RestoreSave(backupName, targetName string) (*Save, error) {
	backupName, err := validateBackupName(backupName)
	if err != nil {
		return nil, err
	}
	if targetName == "" {
		_, targetName, err = parseBackupName(backupName)
		if err != nil {
			return nil, err
		}
	} else {
		targetName, err = ValidateSaveName(targetName)
		if err != nil {
			return nil, err
		}
	}

	src, err := saveBackupPath(backupName)
	if err != nil {
		return nil, err
	}
	dst, err := savePath(targetName)
	if err != nil {
		return nil, err
	}
	if err := copyFileOverwrite(src, dst); err != nil {
		return nil, err
	}

	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	return &Save{
		Name:     targetName,
		LastMod:  info.ModTime(),
		Size:     info.Size(),
		Metadata: readSaveMetadata(dst),
	}, nil
}

func RenameSave(name, newName string) (*Save, error) {
	name, err := ValidateSaveName(name)
	if err != nil {
		return nil, err
	}
	newName, err = ValidateSaveName(newName)
	if err != nil {
		return nil, err
	}
	if name == newName {
		return nil, errors.New("new save name must be different")
	}

	src, err := savePath(name)
	if err != nil {
		return nil, err
	}
	dst, err := savePath(newName)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dst); err == nil {
		return nil, fmt.Errorf("save already exists: %s", newName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.Rename(src, dst); err != nil {
		return nil, err
	}

	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	return &Save{
		Name:     newName,
		LastMod:  info.ModTime(),
		Size:     info.Size(),
		Metadata: readSaveMetadata(dst),
	}, nil
}

func DuplicateSave(name, newName string) (*Save, error) {
	name, err := ValidateSaveName(name)
	if err != nil {
		return nil, err
	}
	newName, err = ValidateSaveName(newName)
	if err != nil {
		return nil, err
	}
	if name == newName {
		return nil, errors.New("new save name must be different")
	}

	src, err := savePath(name)
	if err != nil {
		return nil, err
	}
	dst, err := savePath(newName)
	if err != nil {
		return nil, err
	}
	if err := copyFile(src, dst); err != nil {
		return nil, err
	}

	info, err := os.Stat(dst)
	if err != nil {
		return nil, err
	}
	return &Save{
		Name:     newName,
		LastMod:  info.ModTime(),
		Size:     info.Size(),
		Metadata: readSaveMetadata(dst),
	}, nil
}

// Create savefiles for Factorio
func CreateSave(filePath string) (string, error) {
	return CreateSaveWithSettings(filePath, "", "")
}

func CreateSaveWithSettings(filePath string, mapGenSettingsFile string, mapSettingsFile string) (string, error) {
	err := os.MkdirAll(filepath.Dir(filePath), 0755)
	if err != nil {
		log.Printf("Error in creating Factorio save: %s", err)
		return "", err
	}

	args := buildCreateSaveArgs(filePath, mapGenSettingsFile, mapSettingsFile)
	cmdOutput, err := runFactorio(args...).Output()
	if err != nil {
		log.Printf("Error in creating Factorio save: %s", err)
		log.Println(string(cmdOutput))
		return "", err
	}

	result := string(cmdOutput)

	return result, nil
}

func buildCreateSaveArgs(filePath string, mapGenSettingsFile string, mapSettingsFile string) []string {
	args := []string{"--create", filePath}
	if mapGenSettingsFile != "" {
		args = append(args, "--map-gen-settings", mapGenSettingsFile)
	}
	if mapSettingsFile != "" {
		args = append(args, "--map-settings", mapSettingsFile)
	}
	return args
}

// sendRconCommand sends a Lua command via RCON twice. Factorio 2.0 requires
// the player to confirm console commands that disable achievements: the first
// invocation shows the achievement-warning prompt and the second actually
// executes the command. RCON connections count as player "<server>" so this
// gate applies even to headless servers.
func sendRconCommand(server *Server, command string) (reqId int, err error) {
	// Factorio 2.0 requires confirming console commands that disable achievements.
	// The first invocation shows the achievement-warning prompt and the second
	// actually executes the command. RCON connections count as player "<server>"
	// so this gate applies even to headless servers.
	reqId, err = server.Rcon.Write(command)
	if err != nil {
		return reqId, fmt.Errorf("error sending RCON command (confirm): %v", err)
	}
	log.Printf("RCON command sent (confirm), request id: %v", reqId)

	time.Sleep(500 * time.Millisecond)

	reqId, err = server.Rcon.Write(command)
	if err != nil {
		return reqId, fmt.Errorf("error sending RCON command (execute): %v", err)
	}
	log.Printf("RCON command sent (execute), request id: %v", reqId)

	return reqId, nil
}

func ExtractMapGenSettings(server *Server) (mapGenSettingsPath string, mapSettingsPath string, err error) {
	if server.Rcon == nil {
		return "", "", ErrRCONNotConnected
	}

	writeParsedCommand := "/silent-command helpers.write_file('fsm-parsed-settings.json', helpers.table_to_json(helpers.parse_map_exchange_string(game.get_map_exchange_string())))"
	if _, err = sendRconCommand(server, writeParsedCommand); err != nil {
		return "", "", err
	}

	time.Sleep(2 * time.Second)

	config := bootstrap.GetConfig()
	scriptOutputDir := filepath.Join(config.FactorioDir, "script-output")
	parsedPath := filepath.Join(scriptOutputDir, "fsm-parsed-settings.json")

	parsedData, err := os.ReadFile(parsedPath)
	if err != nil {
		return "", "", fmt.Errorf("parsed settings file not found: %w", err)
	}

	var parsed struct {
		MapGenSettings json.RawMessage `json:"map_gen_settings"`
		MapSettings    json.RawMessage `json:"map_settings"`
	}
	if err = json.Unmarshal(parsedData, &parsed); err != nil {
		return "", "", fmt.Errorf("error parsing settings JSON: %w", err)
	}

	mapGenSettingsPath = filepath.Join(scriptOutputDir, "fsm-map-gen-settings.json")
	mapSettingsPath = filepath.Join(scriptOutputDir, "fsm-map-settings.json")

	if err = os.WriteFile(mapGenSettingsPath, parsed.MapGenSettings, 0644); err != nil {
		return "", "", fmt.Errorf("error writing map gen settings: %w", err)
	}
	if err = os.WriteFile(mapSettingsPath, parsed.MapSettings, 0644); err != nil {
		return "", "", fmt.Errorf("error writing map settings: %w", err)
	}

	mapGenSettingsPath, err = filepath.Abs(mapGenSettingsPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve map gen settings path: %w", err)
	}
	mapSettingsPath, err = filepath.Abs(mapSettingsPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve map settings path: %w", err)
	}

	return mapGenSettingsPath, mapSettingsPath, nil
}

func GetLatestSave() (save Save, err error) {
	saves, err := ListSaves()
	if err != nil {
		return save, err
	}
	for _, item := range saves {
		if save.LastMod.Before(item.LastMod) {
			save = Save{
				Name:     item.Name,
				LastMod:  item.LastMod,
				Size:     item.Size,
				Metadata: item.Metadata,
			}
		}
	}

	return
}

func readSaveMetadata(path string) *SaveMetadata {
	f, err := OpenArchiveFile(path, "level.dat", "level-init.dat")
	if err != nil {
		return &SaveMetadata{Error: err.Error()}
	}
	defer f.Close()

	var header SaveHeader
	if err := header.ReadFrom(f); err != nil {
		return &SaveMetadata{Error: err.Error()}
	}

	mods := make([]SaveModInfo, 0, len(header.Mods))
	for _, mod := range header.Mods {
		mods = append(mods, SaveModInfo{
			Name:    mod.Name,
			Version: mod.Version,
		})
	}

	return &SaveMetadata{
		MapName:         header.Name,
		FactorioVersion: header.FactorioVersion,
		PlayTimeTicks:   readSavePlayTimeTicks(path),
		Mods:            mods,
	}
}

func readSavePlayTimeTicks(path string) uint64 {
	f, err := OpenArchiveFile(path, "level.datmetadata")
	if err != nil {
		return 0
	}
	defer f.Close()

	var data [8]byte
	if _, err := io.ReadFull(f, data[:]); err != nil {
		return 0
	}

	return binary.LittleEndian.Uint64(data[:])
}

type SaveBackupSchedule struct {
	Enabled         bool      `json:"enabled"`
	IntervalMinutes int       `json:"interval_minutes"`
	Retention       int       `json:"retention"`
	Mode            string    `json:"mode"`
	LastRun         time.Time `json:"last_run,omitempty"`
	NextRun         time.Time `json:"next_run,omitempty"`
}

func defaultSaveBackupSchedule() SaveBackupSchedule {
	return SaveBackupSchedule{
		Enabled:         false,
		IntervalMinutes: 60,
		Retention:       5,
		Mode:            "latest",
	}
}

func LoadSaveBackupSchedule() (SaveBackupSchedule, error) {
	schedule := defaultSaveBackupSchedule()
	data, err := os.ReadFile(saveBackupSchedulePath())
	if errors.Is(err, os.ErrNotExist) {
		return schedule, nil
	}
	if err != nil {
		return schedule, err
	}
	if err := json.Unmarshal(data, &schedule); err != nil {
		return schedule, err
	}
	return normalizeSaveBackupSchedule(schedule)
}

func SaveBackupScheduleConfig(schedule SaveBackupSchedule) (SaveBackupSchedule, error) {
	schedule, err := normalizeSaveBackupSchedule(schedule)
	if err != nil {
		return schedule, err
	}
	if schedule.Enabled && schedule.NextRun.IsZero() {
		schedule.NextRun = time.Now().UTC().Add(time.Duration(schedule.IntervalMinutes) * time.Minute)
	}
	data, err := json.MarshalIndent(schedule, "", "    ")
	if err != nil {
		return schedule, err
	}
	if err := os.MkdirAll(saveBackupDir(), 0755); err != nil {
		return schedule, err
	}
	return schedule, os.WriteFile(saveBackupSchedulePath(), data, 0664)
}

func normalizeSaveBackupSchedule(schedule SaveBackupSchedule) (SaveBackupSchedule, error) {
	if schedule.IntervalMinutes <= 0 {
		return schedule, errors.New("interval_minutes must be greater than zero")
	}
	if schedule.Retention <= 0 {
		return schedule, errors.New("retention must be greater than zero")
	}
	if schedule.Mode == "" {
		schedule.Mode = "latest"
	}
	if schedule.Mode != "latest" && schedule.Mode != "all" {
		return schedule, errors.New("mode must be latest or all")
	}
	return schedule, nil
}

func RunScheduledSaveBackup() (SaveBackupSchedule, []SaveBackup, error) {
	schedule, err := LoadSaveBackupSchedule()
	if err != nil {
		return schedule, nil, err
	}

	backups, err := backupScheduledSaves(schedule)
	if err != nil {
		return schedule, backups, err
	}
	if err := PruneSaveBackups(schedule.Retention); err != nil {
		return schedule, backups, err
	}

	schedule.LastRun = time.Now().UTC()
	schedule.NextRun = schedule.LastRun.Add(time.Duration(schedule.IntervalMinutes) * time.Minute)
	schedule, err = SaveBackupScheduleConfig(schedule)
	return schedule, backups, err
}

func backupScheduledSaves(schedule SaveBackupSchedule) ([]SaveBackup, error) {
	saves, err := ListSaves()
	if err != nil {
		return nil, err
	}
	if len(saves) == 0 {
		return []SaveBackup{}, nil
	}

	if schedule.Mode == "latest" {
		latest, err := GetLatestSave()
		if err != nil {
			return nil, err
		}
		backup, err := BackupSave(latest.Name)
		if err != nil {
			return nil, err
		}
		return []SaveBackup{*backup}, nil
	}

	backups := make([]SaveBackup, 0, len(saves))
	for _, save := range saves {
		backup, err := BackupSave(save.Name)
		if err != nil {
			return backups, err
		}
		backups = append(backups, *backup)
	}
	return backups, nil
}

func PruneSaveBackups(retention int) error {
	if retention <= 0 {
		return errors.New("retention must be greater than zero")
	}
	backups, err := ListSaveBackups()
	if err != nil {
		return err
	}

	bySave := make(map[string][]SaveBackup)
	for _, backup := range backups {
		bySave[backup.SaveName] = append(bySave[backup.SaveName], backup)
	}

	for _, saveBackups := range bySave {
		sortSaveBackupsNewestFirst(saveBackups)
		if len(saveBackups) <= retention {
			continue
		}
		for _, backup := range saveBackups[retention:] {
			path, err := saveBackupPath(backup.Name)
			if err != nil {
				return err
			}
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}

	return nil
}

func sortSaveBackupsNewestFirst(backups []SaveBackup) {
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].LastMod.After(backups[j].LastMod)
	})
}

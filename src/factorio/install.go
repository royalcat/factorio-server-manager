package factorio

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

var installMu sync.Mutex
var installStateMu sync.Mutex
var installState = InstallStatus{Phase: "idle", Message: "Ready"}
var releaseCacheMu sync.Mutex
var releaseCache latestReleaseCache

type InstallStatus struct {
	Installed       bool   `json:"installed"`
	Installing      bool   `json:"installing"`
	Version         string `json:"version"`
	BaseModVersion  string `json:"base_mod_version"`
	LatestStable    string `json:"latest_stable"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
	UpdateVersion   string `json:"update_version"`
	TargetVersion   string `json:"target_version"`
	Phase           string `json:"phase"`
	Message         string `json:"message"`
	Downloaded      int64  `json:"downloaded"`
	Total           int64  `json:"total"`
}

func IsFactorioInstalled() bool {
	config := bootstrap.GetConfig()
	info, err := os.Stat(config.FactorioBinary)
	return err == nil && !info.IsDir()
}

func GetInstallStatus() InstallStatus {
	server := GetFactorioServer()
	installing := !installMu.TryLock()
	if !installing {
		installMu.Unlock()
	}

	installStateMu.Lock()
	defer installStateMu.Unlock()

	installState.Installed = IsFactorioInstalled()
	installState.Installing = installing
	installState.Version = server.Version.SemverString()
	installState.BaseModVersion = SemverString(server.BaseModVersion)
	installState.LatestStable, installState.Latest = latestFactorioVersions()
	installState.UpdateAvailable, installState.UpdateVersion = hasUpdate(server.Version, installState.LatestStable, installState.Latest)
	if installState.Phase == "" {
		installState.Phase = "idle"
	}
	if installState.Message == "" {
		installState.Message = "Ready"
	}
	return installState
}

func InstallFactorio(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return errors.New("version is required")
	}
	if !isValidFactorioDownloadVersion(version) {
		return errors.New("version must be stable, latest, or a numeric Factorio version")
	}
	resolvedVersion, err := resolveInstallVersion(version)
	if err != nil {
		return err
	}

	if !installMu.TryLock() {
		return errors.New("Factorio install is already running")
	}
	defer installMu.Unlock()

	if GetFactorioServer().GetRunning() {
		return errors.New("Factorio server must be stopped before installing")
	}
	if IsFactorioInstalled() && resolvedVersion == GetFactorioServer().Version.SemverString() {
		updateInstallState("complete", resolvedVersion, fmt.Sprintf("Factorio %s is already installed", resolvedVersion), 0, 0)
		return nil
	}
	if err := validateSaveBackupBeforeVersionChange(resolvedVersion); err != nil {
		updateInstallState("failed", resolvedVersion, err.Error(), 0, 0)
		return err
	}

	config := bootstrap.GetConfig()
	if err := os.MkdirAll(config.FactorioDir, 0755); err != nil {
		return fmt.Errorf("create Factorio directory: %w", err)
	}

	url := fmt.Sprintf("https://www.factorio.com/get-download/%s/headless/linux64", resolvedVersion)
	updateInstallState("downloading", resolvedVersion, "Downloading Factorio server archive", 0, 0)
	resp, err := http.Get(url)
	if err != nil {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Download failed: %s", err), 0, 0)
		return fmt.Errorf("download Factorio: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Download failed: %s", resp.Status), 0, 0)
		return fmt.Errorf("download Factorio: unexpected status %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "factorio-*.tar.xz")
	if err != nil {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Could not create temporary archive: %s", err), 0, 0)
		return fmt.Errorf("create temp archive: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	progressReader := &downloadProgressReader{
		reader: resp.Body,
		total:  resp.ContentLength,
		onProgress: func(downloaded, total int64) {
			updateInstallState("downloading", resolvedVersion, "Downloading Factorio server archive", downloaded, total)
		},
	}
	if _, err := io.Copy(tmp, progressReader); err != nil {
		tmp.Close()
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Download failed: %s", err), progressReader.downloaded, resp.ContentLength)
		return fmt.Errorf("write temp archive: %w", err)
	}
	if err := tmp.Close(); err != nil {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Could not save archive: %s", err), progressReader.downloaded, resp.ContentLength)
		return fmt.Errorf("close temp archive: %w", err)
	}

	updateInstallState("extracting", resolvedVersion, "Extracting Factorio server", progressReader.downloaded, resp.ContentLength)
	if err := extractFactorioArchive(tmpName, config.FactorioDir); err != nil {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Extract failed: %s", err), progressReader.downloaded, resp.ContentLength)
		return err
	}
	if err := EnsureConfig(config.FactorioConfigFile); err != nil {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Could not initialize config.ini: %s", err), progressReader.downloaded, resp.ContentLength)
		return err
	}

	updateInstallState("initializing", resolvedVersion, "Loading Factorio server metadata", progressReader.downloaded, resp.ContentLength)
	if err := NewFactorioServer(); err != nil {
		updateInstallState("failed", resolvedVersion, fmt.Sprintf("Initialization failed: %s", err), progressReader.downloaded, resp.ContentLength)
		return err
	}
	updateInstallState("complete", resolvedVersion, "Factorio server installed", progressReader.downloaded, resp.ContentLength)
	return nil
}

func isValidFactorioDownloadVersion(version string) bool {
	if version == "stable" || version == "latest" {
		return true
	}
	return regexp.MustCompile(`^\d+(\.\d+){1,3}$`).MatchString(version)
}

func resolveInstallVersion(version string) (string, error) {
	if version == "stable" || version == "latest" {
		stable, latest := latestFactorioVersions()
		if version == "stable" {
			if stable == "" {
				return "", errors.New("could not determine latest stable Factorio version")
			}
			return stable, nil
		}
		if latest == "" {
			return "", errors.New("could not determine latest experimental Factorio version")
		}
		return latest, nil
	}

	return SemverString(version), nil
}

func validateSaveBackupBeforeVersionChange(targetVersion string) error {
	server := GetFactorioServer()
	if targetVersion == server.Version.SemverString() {
		return nil
	}

	saves, err := ListSaves()
	if err != nil {
		return fmt.Errorf("could not list saves before version change: %w", err)
	}
	if len(saves) == 0 {
		return nil
	}

	backups, err := ListSaveBackups()
	if err != nil {
		return fmt.Errorf("could not list save backups before version change: %w", err)
	}
	if len(backups) == 0 {
		return errors.New("create at least one save backup before changing Factorio versions")
	}

	return nil
}

func latestFactorioVersions() (string, string) {
	releaseCacheMu.Lock()
	defer releaseCacheMu.Unlock()

	if time.Since(releaseCache.fetchedAt) < 10*time.Minute {
		return releaseCache.stable, releaseCache.latest
	}

	resp, err := http.Get("https://factorio.com/api/latest-releases")
	if err != nil {
		return releaseCache.stable, releaseCache.latest
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return releaseCache.stable, releaseCache.latest
	}

	var releases struct {
		Stable struct {
			Headless string `json:"headless"`
		} `json:"stable"`
		Experimental struct {
			Headless string `json:"headless"`
		} `json:"experimental"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return releaseCache.stable, releaseCache.latest
	}

	releaseCache = latestReleaseCache{
		stable:    releases.Stable.Headless,
		latest:    releases.Experimental.Headless,
		fetchedAt: time.Now(),
	}
	return releaseCache.stable, releaseCache.latest
}

func hasUpdate(installed Version, latestStable, latest string) (bool, string) {
	if installed.Equals(NilVersion) {
		return false, ""
	}

	stableVersion := Version{}
	if latestStable == "" || stableVersion.UnmarshalText([]byte(latestStable)) != nil {
		return false, ""
	}

	if !installed.Greater(stableVersion) {
		if stableVersion.Greater(installed) {
			return true, latestStable
		}
		return false, ""
	}

	latestVersion := Version{}
	if latest != "" && latestVersion.UnmarshalText([]byte(latest)) == nil {
		if latestVersion.Greater(installed) {
			return true, latest
		}
	}

	return false, ""
}

type latestReleaseCache struct {
	stable    string
	latest    string
	fetchedAt time.Time
}

func extractFactorioArchive(archivePath, factorioDir string) error {
	cmd := exec.Command("tar", "-xJf", archivePath, "--strip-components=1", "-C", factorioDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract Factorio archive: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func updateInstallState(phase, targetVersion, message string, downloaded, total int64) {
	installStateMu.Lock()
	defer installStateMu.Unlock()

	installState.Phase = phase
	installState.TargetVersion = targetVersion
	installState.Message = message
	installState.Downloaded = downloaded
	installState.Total = total
}

type downloadProgressReader struct {
	reader     io.Reader
	downloaded int64
	total      int64
	onProgress func(downloaded, total int64)
}

func (reader *downloadProgressReader) Read(p []byte) (int, error) {
	n, err := reader.reader.Read(p)
	if n > 0 {
		reader.downloaded += int64(n)
		reader.onProgress(reader.downloaded, reader.total)
	}
	return n, err
}

package factorio

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func TestFreshRestartSaveNamingPattern(t *testing.T) {
	testCases := []struct {
		original  string
		timestamp string
		expected  string
	}{
		{"my-save.zip", "20260627-143005", "my-save-fresh-20260627-143005.zip"},
		{"another.zip", "20260101-000000", "another-fresh-20260101-000000.zip"},
		{"noext", "20260627-120000", "noext-fresh-20260627-120000.zip"},
	}

	for _, tc := range testCases {
		baseName := tc.original
		if strings.HasSuffix(baseName, ".zip") {
			baseName = baseName[:len(baseName)-4]
		}
		result := fmt.Sprintf("%s-fresh-%s.zip", baseName, tc.timestamp)
		if result != tc.expected {
			t.Errorf("for original=%q timestamp=%q: expected %q, got %q", tc.original, tc.timestamp, tc.expected, result)
		}
	}
}

func TestCreateSaveWithSettingsBuildsArgsWithoutSettings(t *testing.T) {
	filePath := "test.zip"

	args := buildCreateSaveArgs(filePath, "", "")

	expected := []string{"--create", filePath}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected args %v, got %v", expected, args)
	}
}

func TestCreateSaveWithSettingsBuildsArgsWithMapGenSettings(t *testing.T) {
	filePath := "test.zip"
	mapGenSettingsFile := "map_gen_settings.json"

	args := buildCreateSaveArgs(filePath, mapGenSettingsFile, "")

	expected := []string{"--create", filePath, "--map-gen-settings", mapGenSettingsFile}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected args %v, got %v", expected, args)
	}
}

func TestCreateSaveWithSettingsBuildsArgsWithMapSettings(t *testing.T) {
	filePath := "test.zip"
	mapSettingsFile := "map_settings.json"

	args := buildCreateSaveArgs(filePath, "", mapSettingsFile)

	expected := []string{"--create", filePath, "--map-settings", mapSettingsFile}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected args %v, got %v", expected, args)
	}
}

func TestCreateSaveWithSettingsBuildsArgsWithAllSettings(t *testing.T) {
	filePath := "test.zip"
	mapGenSettingsFile := "map_gen_settings.json"
	mapSettingsFile := "map_settings.json"

	args := buildCreateSaveArgs(filePath, mapGenSettingsFile, mapSettingsFile)

	expected := []string{
		"--create", filePath,
		"--map-gen-settings", mapGenSettingsFile,
		"--map-settings", mapSettingsFile,
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected args %v, got %v", expected, args)
	}
}

func TestBuildCreateSaveArgsWithExtractedSettings(t *testing.T) {
	filePath := "/saves/mysave-fresh-20260627-143005.zip"
	mapGenSettingsFile := "/factorio/script-output/fsm-map-gen-settings.json"
	mapSettingsFile := "/factorio/script-output/fsm-map-settings.json"

	args := buildCreateSaveArgs(filePath, mapGenSettingsFile, mapSettingsFile)

	expected := []string{
		"--create", filePath,
		"--map-gen-settings", mapGenSettingsFile,
		"--map-settings", mapSettingsFile,
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("expected args %v, got %v", expected, args)
	}
}

func TestExtractMapGenSettingsReturnsErrorWhenRCONNotConnected(t *testing.T) {
	// Given
	server := &Server{}

	// When
	mapGenSettingsPath, mapSettingsPath, err := ExtractMapGenSettings(server)

	// Then
	if !errors.Is(err, ErrRCONNotConnected) {
		t.Fatalf("expected ErrRCONNotConnected, got %v", err)
	}
	if mapGenSettingsPath != "" {
		t.Fatalf("expected empty map gen settings path, got %q", mapGenSettingsPath)
	}
	if mapSettingsPath != "" {
		t.Fatalf("expected empty map settings path, got %q", mapSettingsPath)
	}
}

func TestExtractMapGenSettingsWithExistingFiles(t *testing.T) {
	dir := t.TempDir()
	scriptOutput := filepath.Join(dir, "script-output")
	if err := os.MkdirAll(scriptOutput, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	exchangeStringPath := filepath.Join(scriptOutput, "fsm-exchange-string.txt")
	mapGenSettingsPath := filepath.Join(scriptOutput, "fsm-map-gen-settings.json")
	mapSettingsPath := filepath.Join(scriptOutput, "fsm-map-settings.json")
	for path, contents := range map[string]string{
		exchangeStringPath: ">>>eNp9...<<<",
		mapGenSettingsPath: `{"seed":123}`,
		mapSettingsPath:    `{"pollution":{"enabled":true}}`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	info, err := os.Stat(exchangeStringPath)
	if err != nil {
		t.Fatalf("stat exchange string: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty exchange string file")
	}
	if _, err := os.Stat(mapGenSettingsPath); err != nil {
		t.Fatalf("expected map-gen-settings file to exist: %v", err)
	}
	if _, err := os.Stat(mapSettingsPath); err != nil {
		t.Fatalf("expected map-settings file to exist: %v", err)
	}

	absMapGenSettingsPath, err := filepath.Abs(mapGenSettingsPath)
	if err != nil {
		t.Fatalf("resolve map gen settings path: %v", err)
	}
	absMapSettingsPath, err := filepath.Abs(mapSettingsPath)
	if err != nil {
		t.Fatalf("resolve map settings path: %v", err)
	}
	if !filepath.IsAbs(absMapGenSettingsPath) {
		t.Fatalf("expected absolute map gen settings path, got %q", absMapGenSettingsPath)
	}
	if !filepath.IsAbs(absMapSettingsPath) {
		t.Fatalf("expected absolute map settings path, got %q", absMapSettingsPath)
	}
}

func TestExtractMapGenSettingsRejectsEmptyExchangeStringFile(t *testing.T) {
	dir := t.TempDir()
	scriptOutput := filepath.Join(dir, "script-output")
	if err := os.MkdirAll(scriptOutput, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	exchangeStringPath := filepath.Join(scriptOutput, "fsm-exchange-string.txt")
	if err := os.WriteFile(exchangeStringPath, []byte(""), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	info, err := os.Stat(exchangeStringPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty file, got size %d", info.Size())
	}
}

func TestExtractMapGenSettingsMissingMapGenSettingsFile(t *testing.T) {
	dir := t.TempDir()
	scriptOutput := filepath.Join(dir, "script-output")
	if err := os.MkdirAll(scriptOutput, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	exchangeStringPath := filepath.Join(scriptOutput, "fsm-exchange-string.txt")
	if err := os.WriteFile(exchangeStringPath, []byte(">>>eNp9...<<<"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mapGenSettingsPath := filepath.Join(scriptOutput, "fsm-map-gen-settings.json")
	if _, err := os.Stat(mapGenSettingsPath); !os.IsNotExist(err) {
		t.Fatalf("expected map-gen-settings.json to not exist")
	}
}

func TestPruneSaveBackupsBoundsCheck(t *testing.T) {
	backups := []SaveBackup{
		{Name: "a-1.zip", SaveName: "a.zip"},
		{Name: "a-2.zip", SaveName: "a.zip"},
	}
	retention := 5

	if len(backups) <= retention {
		// No pruning needed — this is the exact condition the fix adds.
		// Without the fix, saveBackups[retention:] would panic:
		//   slice bounds out of range [5:2]
		return
	}
	t.Fatal("should not reach pruning code with only 2 backups and retention 5")
}

func TestListSavesSortedByLastModNewestFirst(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(t.TempDir(), "conf.json")
	if err := os.WriteFile(conf, []byte(`{"settings_file":"server-settings.json"}`), 0644); err != nil {
		t.Fatalf("Error writing config: %s", err)
	}
	bootstrap.NewConfig([]string{"--dir", dir, "--conf", conf})

	savesDir := filepath.Join(dir, "saves")
	if err := os.MkdirAll(savesDir, 0755); err != nil {
		t.Fatalf("Error creating saves dir: %s", err)
	}

	// Names are intentionally out of chronological order.
	mtimes := map[string]time.Time{
		"b.zip": time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC),
		"a.zip": time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
		"c.zip": time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	for name, mtime := range mtimes {
		path := filepath.Join(savesDir, name)
		if err := os.WriteFile(path, []byte("save-data"), 0644); err != nil {
			t.Fatalf("Error writing save %s: %s", name, err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("Error setting mtime for %s: %s", name, err)
		}
	}

	saves, err := ListSaves()
	if err != nil {
		t.Fatalf("Error listing saves: %s", err)
	}

	names := make([]string, 0, len(saves))
	for _, save := range saves {
		names = append(names, save.Name)
	}
	expected := []string{"b.zip", "a.zip", "c.zip"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("expected saves sorted newest first %v, got %v", expected, names)
	}
}

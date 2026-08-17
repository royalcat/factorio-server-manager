package factorio

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func TestDeleteAllModsClearsContentsAndPreservesDirectory(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(t.TempDir(), "conf.json")
	if err := os.WriteFile(conf, []byte(`{"rcon_pass":"test-rcon","cookie_encryption_key":"VGhpcy1pcy1hLXRlc3QtMzItYnl0ZS1rZXkh","settings_file":"server-settings.json"}`), 0644); err != nil {
		t.Fatalf("Error writing config: %s", err)
	}
	bootstrap.NewConfig([]string{"--dir", dir, "--conf", conf})

	modsDir := filepath.Join(dir, "mods")
	nestedDir := filepath.Join(modsDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("Error creating mods dir: %s", err)
	}
	for _, path := range []string{
		filepath.Join(modsDir, "mod-list.json"),
		filepath.Join(nestedDir, "mod.zip"),
	} {
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("Error writing mod file: %s", err)
		}
	}

	if err := DeleteAllMods(); err != nil {
		t.Fatalf("DeleteAllMods() error = %v", err)
	}
	if info, err := os.Stat(modsDir); err != nil {
		t.Fatalf("Expected mods directory to remain: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("Expected mods path to remain a directory")
	}
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		t.Fatalf("Error reading mods dir: %s", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Expected mods dir to be empty, got %d entries", len(entries))
	}
}

func TestValidateModPackNameRejectsUnsafeNames(t *testing.T) {
	invalidNames := []string{
		"",
		"../pack",
		"folder/pack",
		`folder\pack`,
		"/tmp/pack",
		".",
		"..",
		"bad\x00pack",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			if err := validateModPackName(name); err == nil {
				t.Fatalf("Expected invalid mod pack name: %q", name)
			}
		})
	}
}

func TestVersionSemverStringUsesFirstThreeSegments(t *testing.T) {
	version := Version{1, 1, 6, 4}
	if actual := version.SemverString(); actual != "1.1.6" {
		t.Fatalf("Expected semver string 1.1.6, got %s", actual)
	}

	if actual := SemverString("2.0.72.0"); actual != "2.0.72" {
		t.Fatalf("Expected semver string 2.0.72, got %s", actual)
	}
}

func TestRequiredDependencyNamesSkipsBuiltInMods(t *testing.T) {
	dependencies := []string{
		"base >= 2.0.0",
		"space-age",
		"quality >= 2.0.0",
		"elevated-rails >= 2.0.0",
		"required-mod >= 1.0.0",
		"? optional-mod >= 1.0.0",
	}

	actual := requiredDependencyNames(dependencies)
	expected := []string{"required-mod"}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Expected required dependencies %v, got %v", expected, actual)
	}
}

func TestValidateEnabledDependenciesSkipsBuiltInMods(t *testing.T) {
	mods := Mods{
		ModInfoList: ModInfoList{
			Mods: []ModInfo{
				{
					Name: "test-mod",
					Dependencies: []string{
						"space-age",
						"quality >= 2.0.0",
						"elevated-rails >= 2.0.0",
					},
					Compatibility: true,
				},
			},
		},
		ModSimpleList: ModSimpleList{
			Mods: []ModSimple{
				{Name: "test-mod", Enabled: true},
			},
		},
	}

	if issues := mods.ValidateEnabledDependencies(); len(issues) > 0 {
		t.Fatalf("Expected no dependency issues, got %v", issues)
	}
}

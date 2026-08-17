package factorio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func TestHasUpdateUsesCurrentChannel(t *testing.T) {
	tests := []struct {
		name         string
		installed    Version
		latestStable string
		latest       string
		wantUpdate   bool
		wantVersion  string
	}{
		{
			name:         "current stable ignores newer experimental",
			installed:    Version{2, 0, 72, 0},
			latestStable: "2.0.72",
			latest:       "2.0.76",
			wantUpdate:   false,
		},
		{
			name:         "older stable updates to stable",
			installed:    Version{2, 0, 70, 0},
			latestStable: "2.0.72",
			latest:       "2.0.76",
			wantUpdate:   true,
			wantVersion:  "2.0.72",
		},
		{
			name:         "experimental updates to experimental",
			installed:    Version{2, 0, 75, 0},
			latestStable: "2.0.72",
			latest:       "2.0.76",
			wantUpdate:   true,
			wantVersion:  "2.0.76",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdate, gotVersion := hasUpdate(tt.installed, tt.latestStable, tt.latest)
			if gotUpdate != tt.wantUpdate {
				t.Fatalf("hasUpdate() update = %v, want %v", gotUpdate, tt.wantUpdate)
			}
			if gotVersion != tt.wantVersion {
				t.Fatalf("hasUpdate() version = %q, want %q", gotVersion, tt.wantVersion)
			}
		})
	}
}

func TestResolveInstallVersionNormalizesNumericVersion(t *testing.T) {
	got, err := resolveInstallVersion("2.0.72.0")
	if err != nil {
		t.Fatalf("resolveInstallVersion() error = %v", err)
	}
	if got != "2.0.72" {
		t.Fatalf("resolveInstallVersion() = %q, want 2.0.72", got)
	}
}

func TestValidateSaveBackupBeforeVersionChangeRequiresBackup(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "conf.json")
	if err := os.WriteFile(conf, []byte(`{
		"rcon_pass":"test-rcon",
		"cookie_encryption_key":"VGhpcy1pcy1hLXRlc3QtMzItYnl0ZS1rZXkh",
		"settings_file":"server-settings.json"
	}`), 0644); err != nil {
		t.Fatalf("Error writing config: %s", err)
	}
	bootstrap.NewConfig([]string{"--dir", dir, "--conf", conf})
	if err := os.MkdirAll(filepath.Join(dir, "saves"), 0755); err != nil {
		t.Fatalf("Error creating saves dir: %s", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "saves", "map.zip"), []byte("not-a-real-save"), 0644); err != nil {
		t.Fatalf("Error writing save file: %s", err)
	}
	SetFactorioServer(Server{
		Version:   Version{2, 0, 72, 0},
		Installed: true,
	})

	err := validateSaveBackupBeforeVersionChange("2.0.73")
	if err == nil {
		t.Fatalf("Expected backup validation error")
	}
	if !strings.Contains(err.Error(), "save backup") {
		t.Fatalf("Expected save backup error, got %v", err)
	}
}

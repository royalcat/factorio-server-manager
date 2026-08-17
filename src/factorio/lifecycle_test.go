package factorio

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func TestSaveLifecycleConfigPreservesRuntimeState(t *testing.T) {
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

	AppendLifecycleEvent("start", "started")
	RecordCrash("boom", []string{"last log"})

	saved, err := SaveLifecycleConfig(LifecycleConfig{
		StartupProfile: StartupProfile{
			Savefile: "map.zip",
			BindIP:   "127.0.0.1",
			Port:     34198,
		},
		RestartSchedule: RestartSchedule{
			Enabled:       true,
			IntervalHours: 2,
		},
		GracefulStopTimeout: 10,
	})
	if err != nil {
		t.Fatalf("SaveLifecycleConfig() error = %v", err)
	}
	if len(saved.Events) == 0 {
		t.Fatalf("Expected lifecycle events to be preserved")
	}
	if saved.LastCrash.Reason != "boom" {
		t.Fatalf("LastCrash.Reason = %q, want boom", saved.LastCrash.Reason)
	}
	if saved.RestartSchedule.NextRestart == "" {
		t.Fatalf("Expected enabled restart schedule to get a next restart time")
	}
}

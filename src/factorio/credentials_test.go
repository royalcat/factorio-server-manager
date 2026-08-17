package factorio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func setupCredentialsTestConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	credentialsFile := filepath.Join(dir, "factorio.auth")
	conf := filepath.Join(dir, "conf.json")
	configJSON := `{
		"rcon_pass":"test-rcon",
		"cookie_encryption_key":"VGhpcy1pcy1hLXRlc3QtMzItYnl0ZS1rZXkh",
		"settings_file":"server-settings.json",
		"factorio_credentials_file":"` + credentialsFile + `"
	}`
	if err := os.WriteFile(conf, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Error writing config: %s", err)
	}
	bootstrap.NewConfig([]string{"--dir", dir, "--conf", conf})

	return credentialsFile
}

func TestCredentialsSaveEncryptsFile(t *testing.T) {
	credentialsFile := setupCredentialsTestConfig(t)

	credentials := Credentials{
		Username: "test-user",
		Userkey:  "test-api-key",
	}
	if err := credentials.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fileBytes, err := os.ReadFile(credentialsFile)
	if err != nil {
		t.Fatalf("Error reading credentials file: %s", err)
	}
	fileContents := string(fileBytes)
	if strings.Contains(fileContents, credentials.Username) || strings.Contains(fileContents, credentials.Userkey) {
		t.Fatalf("Expected encrypted credentials file, got %s", fileContents)
	}

	var loaded Credentials
	ok, err := loaded.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatalf("Expected credentials to load")
	}
	if loaded.Username != credentials.Username || loaded.Userkey != credentials.Userkey {
		t.Fatalf("Wrong loaded credentials: %+v", loaded)
	}
}

func TestCredentialsLoadMigratesPlaintextFile(t *testing.T) {
	credentialsFile := setupCredentialsTestConfig(t)
	plaintext := `{"username":"plain-user","userkey":"plain-api-key"}`
	if err := os.WriteFile(credentialsFile, []byte(plaintext), 0600); err != nil {
		t.Fatalf("Error writing plaintext credentials file: %s", err)
	}

	var credentials Credentials
	ok, err := credentials.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatalf("Expected credentials to load")
	}
	if credentials.Username != "plain-user" || credentials.Userkey != "plain-api-key" {
		t.Fatalf("Wrong loaded credentials: %+v", credentials)
	}

	fileBytes, err := os.ReadFile(credentialsFile)
	if err != nil {
		t.Fatalf("Error reading migrated credentials file: %s", err)
	}
	if strings.Contains(string(fileBytes), "plain-api-key") {
		t.Fatalf("Expected plaintext credentials file to be migrated")
	}
}

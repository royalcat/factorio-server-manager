package api

import (
	"testing"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func TestInitialAdminCredentialsDefaults(t *testing.T) {
	username, password, generatedPassword := initialAdminCredentials(bootstrap.Config{})

	if username != "admin" {
		t.Fatalf("Expected default username admin, got %s", username)
	}
	if password == "" {
		t.Fatal("Expected generated password")
	}
	if !generatedPassword {
		t.Fatal("Expected generatedPassword to be true")
	}
}

func TestInitialAdminCredentialsFromConfig(t *testing.T) {
	username, password, generatedPassword := initialAdminCredentials(bootstrap.Config{
		AdminUsername: "ops",
		AdminPassword: "fixed-password",
	})

	if username != "ops" {
		t.Fatalf("Expected configured username ops, got %s", username)
	}
	if password != "fixed-password" {
		t.Fatalf("Expected configured password, got %s", password)
	}
	if generatedPassword {
		t.Fatal("Expected generatedPassword to be false")
	}
}

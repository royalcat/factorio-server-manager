package factorio

import (
	"fmt"
	"strings"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

func ValidateServerStart(savefile string) []string {
	issues := make([]string, 0)
	server := GetFactorioServer()

	if !IsFactorioInstalled() {
		issues = append(issues, "Factorio server is not installed")
		return issues
	}

	save, err := saveForStart(savefile)
	if err != nil {
		issues = append(issues, err.Error())
	} else if save.Metadata != nil {
		if save.Metadata.Error != "" {
			issues = append(issues, fmt.Sprintf("could not read save metadata for %s: %s", save.Name, save.Metadata.Error))
		} else if save.Metadata.FactorioVersion.Greater(server.Version) {
			issues = append(issues, fmt.Sprintf("save %s was created with Factorio %s, but installed Factorio is %s", save.Name, save.Metadata.FactorioVersion.SemverString(), server.Version.SemverString()))
		}
	}

	config := bootstrap.GetConfig()
	mods, err := NewMods(config.FactorioModsDir)
	if err != nil {
		issues = append(issues, fmt.Sprintf("could not load mods for compatibility check: %s", err))
		return issues
	}
	issues = append(issues, mods.ValidateEnabledDependencies()...)

	return issues
}

func saveForStart(savefile string) (*Save, error) {
	if strings.HasPrefix(savefile, "Load Latest") {
		save, err := GetLatestSave()
		if err != nil {
			return nil, err
		}
		return &save, nil
	}
	return FindSave(savefile)
}

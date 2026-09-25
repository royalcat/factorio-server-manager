package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
	"github.com/OpenFactorioServerManager/factorio-server-manager/src/factorio"
)

// modSettingsSectionNames lists the top level sections of mod-settings.dat
// that are editable from the web ui. The runtime-per-user section is kept
// untouched by this API.
var modSettingsSectionNames = []string{"startup", "runtime-global"}

func modSettingsSectionEditable(section string) bool {
	for _, name := range modSettingsSectionNames {
		if name == section {
			return true
		}
	}
	return false
}

// modSettingColor is the JSON representation of a color setting.
type modSettingColor struct {
	R *float64 `json:"r"`
	G *float64 `json:"g"`
	B *float64 `json:"b"`
	A *float64 `json:"a"`
}

// modSettingEntry is the JSON representation of an entry of a list or
// dictionary setting value. Such settings cannot be edited, they are returned
// for display only.
type modSettingEntry struct {
	Key  *string         `json:"key"`
	Node *modSettingNode `json:"node"`
}

// modSettingNode is a typed setting value.
type modSettingNode struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

// modSettingsResponse is the JSON representation of mod-settings.dat.
type modSettingsResponse struct {
	Version    factorio.Version                     `json:"version"`
	HasQuality bool                                 `json:"hasQuality"`
	Sections   map[string]map[string]modSettingNode `json:"sections"`
}

// modSettingChange is a single requested setting update.
type modSettingChange struct {
	Section string          `json:"section"`
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Value   json.RawMessage `json:"value"`
}

type modSettingsUpdateRequest struct {
	Changes []modSettingChange `json:"changes"`
}

func modSettingsFilePath() string {
	config := bootstrap.GetConfig()
	return filepath.Join(config.FactorioModsDir, factorio.ModSettingsFileName)
}

// GetModSettingsHandler returns the editable sections of mod-settings.dat.
func GetModSettingsHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	path := modSettingsFilePath()
	settings, err := factorio.LoadModSettingsFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			resp = fmt.Sprintf("%s was not found in the mods directory.", factorio.ModSettingsFileName)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			resp = fmt.Sprintf("Error reading %s: %s", factorio.ModSettingsFileName, err)
		}
		log.Println(resp)
		return
	}

	resp = buildModSettingsResponse(settings)
}

// UpdateModSettingsHandler applies setting changes to mod-settings.dat. All
// other parts of the file, including the runtime-per-user section, are left
// untouched.
func UpdateModSettingsHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	var resp interface{}

	defer func() {
		WriteResponse(w, resp)
	}()

	w.Header().Set("Content-Type", "application/json;charset=UTF-8")

	var data modSettingsUpdateRequest
	resp, err = ReadFromRequestBody(w, r, &data)
	if err != nil {
		return
	}

	if len(data.Changes) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		resp = "No mod setting changes were provided."
		log.Println(resp)
		return
	}

	path := modSettingsFilePath()

	factorio.FileLock.LockW(path)
	defer factorio.FileLock.Unlock(path)

	settings, err := factorio.LoadModSettingsFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			resp = fmt.Sprintf("%s was not found in the mods directory.", factorio.ModSettingsFileName)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			resp = fmt.Sprintf("Error reading %s: %s", factorio.ModSettingsFileName, err)
		}
		log.Println(resp)
		return
	}

	if err = applyModSettingChanges(settings, data.Changes); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		resp = fmt.Sprintf("Invalid mod settings update: %s", err)
		log.Println(resp)
		return
	}

	if err = factorio.SaveModSettingsFile(path, settings); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp = fmt.Sprintf("Error writing %s: %s", factorio.ModSettingsFileName, err)
		log.Println(resp)
		return
	}

	resp = map[string]interface{}{
		"success": true,
		"changes": len(data.Changes),
	}
}

func buildModSettingsResponse(settings *factorio.ModSettings) modSettingsResponse {
	sections := make(map[string]map[string]modSettingNode, len(modSettingsSectionNames))
	for _, section := range modSettingsSectionNames {
		nodes := make(map[string]modSettingNode)
		if sectionNode := settings.Data.DictionaryChild(section); sectionNode != nil {
			for i := range sectionNode.Children {
				child := &sectionNode.Children[i]
				if child.Key == nil {
					continue
				}
				valueNode := child.DictionaryChild("value")
				if valueNode == nil {
					continue
				}
				nodes[*child.Key] = modSettingNodeFromTree(valueNode)
			}
		}
		sections[section] = nodes
	}

	return modSettingsResponse{
		Version:    settings.Version,
		HasQuality: settings.HasQuality,
		Sections:   sections,
	}
}

func modSettingNodeFromTree(node *factorio.PropertyTree) modSettingNode {
	switch node.ValueType {
	case factorio.PropertyTreeTypeBool:
		return modSettingNode{Type: "bool", Value: node.Bool}
	case factorio.PropertyTreeTypeNumber:
		return modSettingNode{Type: "double", Value: node.Number}
	case factorio.PropertyTreeTypeString:
		if node.String == nil {
			return modSettingNode{Type: "string", Value: nil}
		}
		return modSettingNode{Type: "string", Value: *node.String}
	case factorio.PropertyTreeTypeSignedInt:
		return modSettingNode{Type: "int", Value: strconv.FormatInt(node.SignedInt, 10)}
	case factorio.PropertyTreeTypeUnsignedInt:
		return modSettingNode{Type: "unsigned", Value: strconv.FormatUint(node.UnsignedInt, 10)}
	case factorio.PropertyTreeTypeNull:
		return modSettingNode{Type: "null", Value: nil}
	case factorio.PropertyTreeTypeDictionary:
		if color, ok := modSettingColorFromTree(node); ok {
			return modSettingNode{Type: "color", Value: color}
		}
		return modSettingNode{Type: "dictionary", Value: modSettingEntriesFromTree(node)}
	case factorio.PropertyTreeTypeList:
		return modSettingNode{Type: "list", Value: modSettingEntriesFromTree(node)}
	default:
		return modSettingNode{Type: "unknown", Value: nil}
	}
}

func modSettingEntriesFromTree(node *factorio.PropertyTree) []modSettingEntry {
	entries := make([]modSettingEntry, 0, len(node.Children))
	for i := range node.Children {
		child := &node.Children[i]
		childNode := modSettingNodeFromTree(child)
		entries = append(entries, modSettingEntry{Key: child.Key, Node: &childNode})
	}
	return entries
}

// modSettingColorFromTree detects color values, which are dictionaries with
// exactly the numeric channels r, g, b and a.
func modSettingColorFromTree(node *factorio.PropertyTree) (*modSettingColor, bool) {
	if node.ValueType != factorio.PropertyTreeTypeDictionary || len(node.Children) != 4 {
		return nil, false
	}

	color := &modSettingColor{}
	for i := range node.Children {
		child := &node.Children[i]
		if child.Key == nil || child.ValueType != factorio.PropertyTreeTypeNumber {
			return nil, false
		}
		value := child.Number
		switch *child.Key {
		case "r":
			color.R = &value
		case "g":
			color.G = &value
		case "b":
			color.B = &value
		case "a":
			color.A = &value
		default:
			return nil, false
		}
	}

	if color.R == nil || color.G == nil || color.B == nil || color.A == nil {
		return nil, false
	}

	return color, true
}

func applyModSettingChanges(settings *factorio.ModSettings, changes []modSettingChange) error {
	for i := range changes {
		change := &changes[i]
		if err := applyModSettingChange(settings, change); err != nil {
			return fmt.Errorf("change %d (%s/%s): %v", i+1, change.Section, change.Name, err)
		}
	}
	return nil
}

func applyModSettingChange(settings *factorio.ModSettings, change *modSettingChange) error {
	if change.Section == "" || change.Name == "" {
		return errors.New("section and name are required")
	}

	if !modSettingsSectionEditable(change.Section) {
		return fmt.Errorf("section %q is not editable", change.Section)
	}

	valueNode, err := settings.SettingValue(change.Section, change.Name)
	if err != nil {
		return err
	}

	storedType := modSettingNodeFromTree(valueNode).Type
	if storedType != change.Type {
		return fmt.Errorf("value type %q does not match stored type %q", change.Type, storedType)
	}

	switch valueNode.ValueType {
	case factorio.PropertyTreeTypeBool:
		var value bool
		if err := json.Unmarshal(change.Value, &value); err != nil {
			return errors.New("value must be a boolean")
		}
		valueNode.Bool = value
	case factorio.PropertyTreeTypeSignedInt:
		value, err := parseJSONInteger(change.Value)
		if err != nil {
			return err
		}
		valueNode.SignedInt = value
	case factorio.PropertyTreeTypeNumber:
		var value float64
		if err := json.Unmarshal(change.Value, &value); err != nil {
			return errors.New("value must be a number")
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return errors.New("value must be a finite number")
		}
		valueNode.Number = value
	case factorio.PropertyTreeTypeString:
		var value *string
		if err := json.Unmarshal(change.Value, &value); err != nil {
			return errors.New("value must be a string or null")
		}
		valueNode.String = value
	case factorio.PropertyTreeTypeDictionary:
		var color modSettingColor
		if err := json.Unmarshal(change.Value, &color); err != nil {
			return errors.New("value must be a color with r, g, b and a channels")
		}
		if color.R == nil || color.G == nil || color.B == nil || color.A == nil {
			return errors.New("value must contain r, g, b and a channels")
		}
		for _, channel := range []struct {
			name  string
			value *float64
		}{{"r", color.R}, {"g", color.G}, {"b", color.B}, {"a", color.A}} {
			channelNode := valueNode.DictionaryChild(channel.name)
			if channelNode == nil || channelNode.ValueType != factorio.PropertyTreeTypeNumber {
				return fmt.Errorf("stored setting has no numeric %q channel", channel.name)
			}
			channelNode.Number = *channel.value
		}
	default:
		return fmt.Errorf("editing %q settings is not supported", storedType)
	}

	return nil
}

// parseJSONInteger accepts integers either as decimal strings (used by the web
// ui to stay 64 bit safe) or as JSON numbers.
func parseJSONInteger(raw json.RawMessage) (int64, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		value, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			return 0, errors.New("value must be an integer")
		}
		return value, nil
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err != nil {
		return 0, errors.New("value must be an integer")
	}

	// JSON numbers are decoded as float64, stay inside the exact integer range.
	if math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) ||
		number < -9007199254740992 || number > 9007199254740992 {
		return 0, errors.New("value must be an integer")
	}

	return int64(number), nil
}

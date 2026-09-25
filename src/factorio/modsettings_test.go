package factorio

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// modSettingsFixturePath points to a mod-settings.dat example vendored from
// https://codeberg.org/whitequark/factorio-data-codec (0BSD licensed). That
// project is also the reference implementation this codec was verified
// against.
const modSettingsFixturePath = "../factorio_testfiles/example-mod-settings.dat"

func stringPtr(value string) *string { return &value }

func TestModSettingsSyntheticRoundTrip(t *testing.T) {
	settings := &ModSettings{
		Version:    Version{2, 0, 77, 0},
		HasQuality: true,
		Data: PropertyTree{
			ValueType: PropertyTreeTypeDictionary,
			AnyType:   1,
			Children: []PropertyTree{
				{
					Key:       stringPtr("startup"),
					ValueType: PropertyTreeTypeDictionary,
					Children: []PropertyTree{
						{
							Key:       stringPtr("a-bool"),
							ValueType: PropertyTreeTypeDictionary,
							Children: []PropertyTree{
								{Key: stringPtr("value"), ValueType: PropertyTreeTypeBool, Bool: true, AnyType: 1},
							},
						},
						{Key: stringPtr("a-number"), ValueType: PropertyTreeTypeNumber, Number: -1.5},
						{Key: stringPtr("a-string"), ValueType: PropertyTreeTypeString, String: stringPtr("hello world")},
						{Key: stringPtr("a-long-string"), ValueType: PropertyTreeTypeString, String: stringPtr(strings.Repeat("x", 300))},
						{Key: stringPtr("a-none-string"), ValueType: PropertyTreeTypeString, String: nil},
						{Key: stringPtr("an-int"), ValueType: PropertyTreeTypeSignedInt, SignedInt: -42},
						{Key: stringPtr("an-uint"), ValueType: PropertyTreeTypeUnsignedInt, UnsignedInt: 42},
						{Key: stringPtr("a-null"), ValueType: PropertyTreeTypeNull},
						{
							Key:       stringPtr("a-color"),
							ValueType: PropertyTreeTypeDictionary,
							Children: []PropertyTree{
								{Key: stringPtr("r"), ValueType: PropertyTreeTypeNumber, Number: 0.1},
								{Key: stringPtr("g"), ValueType: PropertyTreeTypeNumber, Number: 0.2},
								{Key: stringPtr("b"), ValueType: PropertyTreeTypeNumber, Number: 0.3},
								{Key: stringPtr("a"), ValueType: PropertyTreeTypeNumber, Number: 0.4},
							},
						},
						{
							Key:       stringPtr("a-list"),
							ValueType: PropertyTreeTypeList,
							Children: []PropertyTree{
								{Key: nil, ValueType: PropertyTreeTypeNull},
								{Key: stringPtr("entry"), ValueType: PropertyTreeTypeBool, Bool: false},
							},
						},
					},
				},
				{Key: stringPtr("runtime-per-user"), ValueType: PropertyTreeTypeDictionary},
			},
		},
	}

	var encoded bytes.Buffer
	if err := settings.WriteTo(&encoded); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	decoded, err := ReadModSettings(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("ReadModSettings: %v", err)
	}

	if !reflect.DeepEqual(settings, decoded) {
		t.Fatalf("round trip mismatch:\nwant %#v\ngot  %#v", settings, decoded)
	}

	value, err := decoded.SettingValue("startup", "a-bool")
	if err != nil {
		t.Fatalf("SettingValue: %v", err)
	}
	if !value.Bool {
		t.Fatal("SettingValue returned the wrong value")
	}

	if _, err := decoded.SettingValue("startup", "does-not-exist"); err == nil {
		t.Fatal("SettingValue should fail for unknown settings")
	}
}

func TestModSettingsExampleFixture(t *testing.T) {
	raw, err := os.ReadFile(modSettingsFixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	settings, err := ReadModSettings(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadModSettings: %v", err)
	}

	if !settings.Version.Equals(Version{2, 0, 76, 0}) {
		t.Fatalf("unexpected fixture version %s", settings.Version)
	}
	if settings.HasQuality {
		t.Fatal("fixture unexpectedly has quality")
	}

	for name, want := range map[string]int{
		"startup":          168,
		"runtime-global":   51,
		"runtime-per-user": 30,
	} {
		section := settings.Data.DictionaryChild(name)
		if section == nil {
			t.Fatalf("fixture section %q is missing", name)
		}
		if len(section.Children) != want {
			t.Fatalf("fixture section %q has %d settings, want %d", name, len(section.Children), want)
		}
	}

	var encoded bytes.Buffer
	if err := settings.WriteTo(&encoded); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !bytes.Equal(raw, encoded.Bytes()) {
		t.Fatal("fixture does not round trip byte for byte")
	}
}

// TestModSettingsLocalFileRoundTrip uses the mod-settings.dat a developer may
// have dropped into the repository root. It is skipped when the file is not
// present, for example in CI.
func TestModSettingsLocalFileRoundTrip(t *testing.T) {
	const path = "../../mod-settings.dat"

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("%s not present", path)
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	settings, err := ReadModSettings(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("ReadModSettings: %v", err)
	}

	var encoded bytes.Buffer
	if err := settings.WriteTo(&encoded); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !bytes.Equal(raw, encoded.Bytes()) {
		t.Fatalf("%s does not round trip byte for byte", path)
	}
}

func TestSaveModSettingsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ModSettingsFileName)

	settings := &ModSettings{
		Version: Version{2, 0, 77, 0},
		Data: PropertyTree{
			ValueType: PropertyTreeTypeDictionary,
			Children: []PropertyTree{
				{
					Key:       stringPtr("startup"),
					ValueType: PropertyTreeTypeDictionary,
					Children: []PropertyTree{
						{Key: stringPtr("a-bool"), ValueType: PropertyTreeTypeBool, Bool: true},
					},
				},
			},
		},
	}

	if err := SaveModSettingsFile(path, settings); err != nil {
		t.Fatalf("SaveModSettingsFile: %v", err)
	}

	loaded, err := LoadModSettingsFile(path)
	if err != nil {
		t.Fatalf("LoadModSettingsFile: %v", err)
	}
	if !reflect.DeepEqual(settings, loaded) {
		t.Fatalf("saved file mismatch:\nwant %#v\ngot  %#v", settings, loaded)
	}

	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if err := SaveModSettingsFile(path, settings); err != nil {
		t.Fatalf("second SaveModSettingsFile: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions not preserved, got %v", info.Mode().Perm())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only %s in %s, got %d files", ModSettingsFileName, dir, len(entries))
	}
}

func TestLoadModSettingsFileMissing(t *testing.T) {
	_, err := LoadModSettingsFile(filepath.Join(t.TempDir(), ModSettingsFileName))
	if !os.IsNotExist(err) {
		t.Fatalf("expected a not exist error, got %v", err)
	}
}

func TestModSettingsRejectsOldVersion(t *testing.T) {
	var raw bytes.Buffer
	for _, part := range []uint16{0, 17, 0, 0} {
		if err := binary.Write(&raw, binary.LittleEndian, part); err != nil {
			t.Fatalf("encode version: %v", err)
		}
	}
	raw.WriteByte(0)

	if _, err := ReadModSettings(&raw); err == nil {
		t.Fatal("expected an error for a version older than 0.18.0.0")
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
	"github.com/OpenFactorioServerManager/factorio-server-manager/src/factorio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testStringPtr(value string) *string { return &value }

func testSettingNode(name string, value factorio.PropertyTree) factorio.PropertyTree {
	value.Key = testStringPtr("value")
	return factorio.PropertyTree{
		Key:       testStringPtr(name),
		ValueType: factorio.PropertyTreeTypeDictionary,
		Children:  []factorio.PropertyTree{value},
	}
}

func testModSettingsData() *factorio.ModSettings {
	color := func(r, g, b, a float64) factorio.PropertyTree {
		return factorio.PropertyTree{
			ValueType: factorio.PropertyTreeTypeDictionary,
			Children: []factorio.PropertyTree{
				{Key: testStringPtr("r"), ValueType: factorio.PropertyTreeTypeNumber, Number: r},
				{Key: testStringPtr("g"), ValueType: factorio.PropertyTreeTypeNumber, Number: g},
				{Key: testStringPtr("b"), ValueType: factorio.PropertyTreeTypeNumber, Number: b},
				{Key: testStringPtr("a"), ValueType: factorio.PropertyTreeTypeNumber, Number: a},
			},
		}
	}

	return &factorio.ModSettings{
		Version: factorio.Version{2, 0, 77, 0},
		Data: factorio.PropertyTree{
			ValueType: factorio.PropertyTreeTypeDictionary,
			Children: []factorio.PropertyTree{
				{
					Key:       testStringPtr("startup"),
					ValueType: factorio.PropertyTreeTypeDictionary,
					Children: []factorio.PropertyTree{
						testSettingNode("a-bool", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeBool, Bool: true}),
						testSettingNode("an-int", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeSignedInt, SignedInt: 42}),
						testSettingNode("a-double", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeNumber, Number: 1.5}),
						testSettingNode("a-string", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeString, String: testStringPtr("hello")}),
						testSettingNode("a-none-string", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeString}),
						testSettingNode("a-color", factorio.PropertyTree{
							ValueType: factorio.PropertyTreeTypeDictionary,
							Children:  color(0.1, 0.2, 0.3, 0.4).Children,
						}),
						testSettingNode("a-dict", factorio.PropertyTree{
							ValueType: factorio.PropertyTreeTypeDictionary,
							Children: []factorio.PropertyTree{
								{Key: testStringPtr("nested"), ValueType: factorio.PropertyTreeTypeBool, Bool: true},
							},
						}),
					},
				},
				{
					Key:       testStringPtr("runtime-global"),
					ValueType: factorio.PropertyTreeTypeDictionary,
					Children: []factorio.PropertyTree{
						testSettingNode("a-global-int", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeSignedInt, SignedInt: 5}),
					},
				},
				{
					Key:       testStringPtr("runtime-per-user"),
					ValueType: factorio.PropertyTreeTypeDictionary,
					Children: []factorio.PropertyTree{
						testSettingNode("a-user-int", factorio.PropertyTree{ValueType: factorio.PropertyTreeTypeSignedInt, SignedInt: 5}),
					},
				},
			},
		},
	}
}

func performModSettingsRequest(handler http.HandlerFunc, method string, body interface{}) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(encoded)
	}

	request := httptest.NewRequest(method, "/api/mods/settings", reader)
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	return recorder
}

func TestBuildModSettingsResponse(t *testing.T) {
	response := buildModSettingsResponse(testModSettingsData())

	assert.True(t, response.Version.Equals(factorio.Version{2, 0, 77, 0}))
	assert.False(t, response.HasQuality)

	startup, ok := response.Sections["startup"]
	require.True(t, ok)
	require.Len(t, startup, 7)

	assert.Equal(t, "bool", startup["a-bool"].Type)
	assert.Equal(t, true, startup["a-bool"].Value)

	assert.Equal(t, "int", startup["an-int"].Type)
	assert.Equal(t, "42", startup["an-int"].Value)

	assert.Equal(t, "double", startup["a-double"].Type)
	assert.InDelta(t, 1.5, startup["a-double"].Value, 0)

	assert.Equal(t, "string", startup["a-string"].Type)
	assert.Equal(t, "hello", startup["a-string"].Value)

	assert.Equal(t, "string", startup["a-none-string"].Type)
	assert.Nil(t, startup["a-none-string"].Value)

	assert.Equal(t, "color", startup["a-color"].Type)
	color, ok := startup["a-color"].Value.(*modSettingColor)
	require.True(t, ok)
	require.NotNil(t, color.R)
	assert.InDelta(t, 0.1, *color.R, 1e-9)
	assert.InDelta(t, 0.4, *color.A, 1e-9)

	assert.Equal(t, "dictionary", startup["a-dict"].Type)

	global, ok := response.Sections["runtime-global"]
	require.True(t, ok)
	require.Len(t, global, 1)
	assert.Equal(t, "5", global["a-global-int"].Value)

	_, hasPerUser := response.Sections["runtime-per-user"]
	assert.False(t, hasPerUser, "runtime-per-user must not be exposed")
}

func TestApplyModSettingChanges(t *testing.T) {
	settings := testModSettingsData()

	changes := []modSettingChange{
		{Section: "startup", Name: "a-bool", Type: "bool", Value: json.RawMessage(`false`)},
		{Section: "startup", Name: "an-int", Type: "int", Value: json.RawMessage(`"7"`)},
		{Section: "startup", Name: "a-double", Type: "double", Value: json.RawMessage(`2.5`)},
		{Section: "startup", Name: "a-string", Type: "string", Value: json.RawMessage(`"changed"`)},
		{Section: "startup", Name: "a-none-string", Type: "string", Value: json.RawMessage(`"not null"`)},
		{Section: "startup", Name: "a-color", Type: "color", Value: json.RawMessage(`{"r":1,"g":0.5,"b":0.25,"a":0.125}`)},
		{Section: "runtime-global", Name: "a-global-int", Type: "int", Value: json.RawMessage(`100`)},
	}
	require.NoError(t, applyModSettingChanges(settings, changes))

	value, err := settings.SettingValue("startup", "a-bool")
	require.NoError(t, err)
	assert.False(t, value.Bool)

	value, err = settings.SettingValue("startup", "an-int")
	require.NoError(t, err)
	assert.Equal(t, int64(7), value.SignedInt)

	value, err = settings.SettingValue("startup", "a-double")
	require.NoError(t, err)
	assert.InDelta(t, 2.5, value.Number, 0)

	value, err = settings.SettingValue("startup", "a-string")
	require.NoError(t, err)
	require.NotNil(t, value.String)
	assert.Equal(t, "changed", *value.String)

	value, err = settings.SettingValue("startup", "a-none-string")
	require.NoError(t, err)
	require.NotNil(t, value.String)
	assert.Equal(t, "not null", *value.String)

	value, err = settings.SettingValue("startup", "a-color")
	require.NoError(t, err)
	require.Equal(t, factorio.PropertyTreeTypeNumber, value.DictionaryChild("g").ValueType)
	assert.InDelta(t, 0.5, value.DictionaryChild("g").Number, 0)

	value, err = settings.SettingValue("runtime-global", "a-global-int")
	require.NoError(t, err)
	assert.Equal(t, int64(100), value.SignedInt)

	// the runtime-per-user section is never touched
	value, err = settings.SettingValue("runtime-per-user", "a-user-int")
	require.NoError(t, err)
	assert.Equal(t, int64(5), value.SignedInt)

	invalidChanges := [][]modSettingChange{
		{{Section: "startup", Name: "a-bool", Type: "int", Value: json.RawMessage(`1`)}},
		{{Section: "startup", Name: "does-not-exist", Type: "bool", Value: json.RawMessage(`true`)}},
		{{Section: "not-a-section", Name: "a-bool", Type: "bool", Value: json.RawMessage(`true`)}},
		{{Section: "startup", Name: "a-bool", Type: "bool", Value: json.RawMessage(`"yes"`)}},
		{{Section: "startup", Name: "an-int", Type: "int", Value: json.RawMessage(`"not a number"`)}},
		{{Section: "startup", Name: "a-dict", Type: "dictionary", Value: json.RawMessage(`{}`)}},
		{{Section: "startup", Name: "a-color", Type: "color", Value: json.RawMessage(`{"r":1}`)}},
		{{Section: "runtime-per-user", Name: "a-user-int", Type: "int", Value: json.RawMessage(`"7"`)}},
	}

	for i, invalid := range invalidChanges {
		err := applyModSettingChanges(testModSettingsData(), invalid)
		assert.Error(t, err, "invalid change %d should fail", i)
	}
}

func TestModSettingsHandlerRoundTrip(t *testing.T) {
	SetupMods(t, true)
	defer CleanupMods(t)

	path := filepath.Join(bootstrap.GetConfig().FactorioModsDir, factorio.ModSettingsFileName)
	require.NoError(t, factorio.SaveModSettingsFile(path, testModSettingsData()))

	recorder := performModSettingsRequest(GetModSettingsHandler, http.MethodGet, nil)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Version  string `json:"version"`
		Sections map[string]map[string]struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"sections"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "2.0.77.0", response.Version)
	assert.JSONEq(t, `"42"`, string(response.Sections["startup"]["an-int"].Value))
	assert.JSONEq(t, `true`, string(response.Sections["startup"]["a-bool"].Value))

	update := map[string]interface{}{
		"changes": []map[string]interface{}{
			{"section": "startup", "name": "a-bool", "type": "bool", "value": false},
			{"section": "startup", "name": "an-int", "type": "int", "value": "12"},
		},
	}
	recorder = performModSettingsRequest(UpdateModSettingsHandler, http.MethodPost, update)
	require.Equal(t, http.StatusOK, recorder.Code)

	reloaded, err := factorio.LoadModSettingsFile(path)
	require.NoError(t, err)

	value, err := reloaded.SettingValue("startup", "a-bool")
	require.NoError(t, err)
	assert.False(t, value.Bool)

	value, err = reloaded.SettingValue("startup", "an-int")
	require.NoError(t, err)
	assert.Equal(t, int64(12), value.SignedInt)

	// the untouched runtime-per-user section is still there
	value, err = reloaded.SettingValue("runtime-per-user", "a-user-int")
	require.NoError(t, err)
	assert.Equal(t, int64(5), value.SignedInt)

	before, err := os.ReadFile(path)
	require.NoError(t, err)

	invalidUpdate := map[string]interface{}{
		"changes": []map[string]interface{}{
			{"section": "startup", "name": "does-not-exist", "type": "bool", "value": true},
		},
	}
	recorder = performModSettingsRequest(UpdateModSettingsHandler, http.MethodPost, invalidUpdate)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(before, after), "rejected updates must not modify the file")
}

func TestModSettingsHandlerMissingFile(t *testing.T) {
	SetupMods(t, true)
	defer CleanupMods(t)

	path := filepath.Join(bootstrap.GetConfig().FactorioModsDir, factorio.ModSettingsFileName)
	_ = os.Remove(path)

	recorder := performModSettingsRequest(GetModSettingsHandler, http.MethodGet, nil)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	update := map[string]interface{}{
		"changes": []map[string]interface{}{
			{"section": "startup", "name": "a-bool", "type": "bool", "value": true},
		},
	}
	recorder = performModSettingsRequest(UpdateModSettingsHandler, http.MethodPost, update)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

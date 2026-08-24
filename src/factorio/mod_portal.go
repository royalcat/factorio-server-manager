package factorio

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var modPortalBaseURL = "https://mods.factorio.com"

type ModPortalStruct struct {
	DownloadsCount int    `json:"downloads_count"`
	Name           string `json:"name"`
	Owner          string `json:"owner"`
	Releases       []struct {
		DownloadURL string `json:"download_url"`
		FileName    string `json:"file_name"`
		InfoJSON    struct {
			FactorioVersion Version  `json:"factorio_version"`
			Dependencies    []string `json:"dependencies"`
		} `json:"info_json"`
		ReleasedAt    time.Time `json:"released_at"`
		Sha1          string    `json:"sha1"`
		Version       Version   `json:"version"`
		Compatibility bool      `json:"compatibility"`
	} `json:"releases"`
	Summary string `json:"summary"`
	Title   string `json:"title"`
}

type ModPortalSearchRequest struct {
	Query    string `json:"query,omitempty"`
	Version  string `json:"version,omitempty"`
	PageSize int    `json:"page_size"`
	Page     int    `json:"page"`
}

func ModPortalSearch(query string, version string) (interface{}, error, int) {
	reqBody := ModPortalSearchRequest{
		Query:    query,
		Version:  version,
		PageSize: 20,
		Page:     1,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err, http.StatusInternalServerError
	}

	req, err := http.NewRequest(http.MethodPost, modPortalBaseURL+"/api/search", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}
	defer resp.Body.Close()

	text, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(text)), resp.StatusCode
	}

	var jsonVal interface{}
	err = json.Unmarshal(text, &jsonVal)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}

	return jsonVal, nil, resp.StatusCode
}

// get all mods uploaded to the factorio modPortal
func ModPortalList() (interface{}, error, int) {
	req, err := http.NewRequest(http.MethodGet, modPortalBaseURL+"/api/mods?page_size=max", nil)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}
	defer resp.Body.Close()

	text, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(text)), resp.StatusCode
	}

	var jsonVal interface{}
	err = json.Unmarshal(text, &jsonVal)
	if err != nil {
		return "error", err, http.StatusInternalServerError
	}

	return jsonVal, nil, resp.StatusCode
}

// get the details (mod-info, releases, etc.) from a specific mod from the modPortal
func ModPortalModDetails(modId string) (ModPortalStruct, error, int) {
	var mod ModPortalStruct

	req, err := http.NewRequest(http.MethodGet, modPortalBaseURL+"/api/mods/"+modId, nil)
	if err != nil {
		return mod, err, http.StatusInternalServerError
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return mod, err, http.StatusInternalServerError
	}
	defer resp.Body.Close()

	text, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return mod, err, http.StatusInternalServerError
	}

	err = json.Unmarshal(text, &mod)
	if err != nil {
		return mod, err, http.StatusInternalServerError
	}

	if resp.StatusCode != http.StatusOK {
		return ModPortalStruct{}, errors.New(string(text)), resp.StatusCode
	}

	server := GetFactorioServer()

	installedBaseVersion := Version{}
	_ = installedBaseVersion.UnmarshalText([]byte(server.BaseModVersion))

	for key, release := range mod.Releases {
		factorioVersion := release.InfoJSON.FactorioVersion
		release.Compatibility = isCompatibleWithRange(installedBaseVersion, factorioVersion, release.InfoJSON.Dependencies)
		mod.Releases[key] = release
	}

	return mod, nil, resp.StatusCode
}

// Log the user into factorio, so mods can be downloaded
func FactorioLogin(username string, password string) (error, int) {
	var err error

	resp, err := http.PostForm("https://auth.factorio.com/api-login",
		url.Values{"require_game_ownership": {"true"}, "username": {username}, "password": {password}})

	if err != nil {
		return err, http.StatusInternalServerError
	}

	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err, http.StatusInternalServerError
	}

	bodyString := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return errors.New(bodyString), resp.StatusCode
	}

	var successResponse []string
	err = json.Unmarshal(bodyBytes, &successResponse)
	if err != nil {
		return err, http.StatusInternalServerError
	}

	credentials := Credentials{
		Username: username,
		Userkey:  successResponse[0],
	}

	err = credentials.Save()
	if err != nil {
		return err, http.StatusInternalServerError
	}

	return nil, http.StatusOK
}

func SetModPortalBaseURL(baseURL string) {
	if strings.TrimSpace(baseURL) == "" {
		modPortalBaseURL = "https://mods.factorio.com"
		return
	}
	modPortalBaseURL = strings.TrimRight(baseURL, "/")
}

func FactorioApiKeyLogin(username string, apiKey string) (error, int) {
	username = strings.TrimSpace(username)
	apiKey = strings.TrimSpace(apiKey)

	if username == "" || apiKey == "" {
		return errors.New("username and API key are required"), http.StatusBadRequest
	}

	credentials := Credentials{
		Username: username,
		Userkey:  apiKey,
	}

	err := credentials.Save()
	if err != nil {
		return err, http.StatusInternalServerError
	}

	return nil, http.StatusOK
}

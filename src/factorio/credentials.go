package factorio

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/OpenFactorioServerManager/factorio-server-manager/src/bootstrap"
)

type Credentials struct {
	Username string `json:"username"`
	Userkey  string `json:"userkey"`
}

type encryptedCredentialsFile struct {
	Version    int    `json:"version"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

func (credentials *Credentials) Save() error {
	var err error
	config := bootstrap.GetConfig()
	credentialsJson, err := json.Marshal(credentials)
	if err != nil {
		log.Printf("error mashalling the credentials: %s", err)
		return err
	}

	fileBytes, err := encryptCredentials(credentialsJson)
	if err != nil {
		log.Printf("error encrypting the credentials: %s", err)
		return err
	}

	err = ioutil.WriteFile(config.FactorioCredentialsFile, fileBytes, 0600)
	if err != nil {
		log.Printf("error on saving the credentials. %s", err)
		return err
	}

	return nil
}

func (credentials *Credentials) Load() (bool, error) {
	var err error
	config := bootstrap.GetConfig()
	if _, err := os.Stat(config.FactorioCredentialsFile); os.IsNotExist(err) {
		return false, nil
	}

	fileBytes, err := ioutil.ReadFile(config.FactorioCredentialsFile)
	if err != nil {
		credentials.Del()
		log.Printf("error reading CredentialsFile: %s", err)
		return false, err
	}

	credentialsJson, encrypted, err := decryptCredentials(fileBytes)
	if err != nil {
		credentials.Del()
		log.Printf("error decrypting credentials_file: %s", err)
		return false, err
	}

	err = json.Unmarshal(credentialsJson, credentials)
	if err != nil {
		credentials.Del()
		log.Printf("error on unmarshal credentials_file: %s", err)
		return false, err
	}

	if !encrypted {
		if err := credentials.Save(); err != nil {
			return false, err
		}
	}

	if credentials.Userkey != "" && credentials.Username != "" {
		return true, nil
	} else {
		credentials.Del()
		return false, errors.New("incredients incomplete")
	}
}

func (credentials *Credentials) Del() error {
	var err error
	config := bootstrap.GetConfig()
	err = os.Remove(config.FactorioCredentialsFile)
	if err != nil {
		log.Printf("error delete the credentialfile: %s", err)
		return err
	}

	return nil
}

func credentialsCipher() (cipher.AEAD, error) {
	config := bootstrap.GetConfig()
	key, err := base64.StdEncoding.DecodeString(config.CookieEncryptionKey)
	if err != nil {
		return nil, err
	}
	encryptionKey := sha256.Sum256(key)

	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func encryptCredentials(plaintext []byte) ([]byte, error) {
	aead, err := credentialsCipher()
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	file := encryptedCredentialsFile{
		Version:    1,
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plaintext, nil)),
	}

	return json.Marshal(file)
}

func decryptCredentials(fileBytes []byte) ([]byte, bool, error) {
	var file encryptedCredentialsFile
	if err := json.Unmarshal(fileBytes, &file); err != nil {
		return nil, false, err
	}

	if file.Version == 0 && file.Nonce == "" && file.Ciphertext == "" {
		return fileBytes, false, nil
	}
	if file.Version != 1 {
		return nil, true, fmt.Errorf("unsupported credentials file version %d", file.Version)
	}

	nonce, err := base64.StdEncoding.DecodeString(file.Nonce)
	if err != nil {
		return nil, true, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(file.Ciphertext)
	if err != nil {
		return nil, true, err
	}

	aead, err := credentialsCipher()
	if err != nil {
		return nil, true, err
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, true, err
	}

	return plaintext, true, nil
}

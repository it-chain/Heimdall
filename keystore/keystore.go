/*
 * Copyright 2018 It-chain
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

// This file provides functions for storing and loading ECDSA key pair.

package keystore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/it-chain/heimdall"
	"github.com/it-chain/heimdall/encryption"
	"github.com/it-chain/heimdall/kdf"
)

var ErrInvalidKeyGenOpt = errors.New("invalid ECDSA key generation option - not supported curve")
var ErrWrongKeyID = errors.New("wrong key id - failed to find key using key ID")
var ErrEmptyKeyPath = errors.New("invalid keyPath - keyPath empty")

// struct for encrypted key's file format.
type KeyFile struct {
	SKI          []byte
	KeyGenOpt    string
	IsPrivate    bool
	EncryptedKey string
	Hints        *EncryptionHints
}

// struct for providing hints of encryption and key derivation function.
type EncryptionHints struct {
	EncOpt  *encryption.Opts
	KDFOpt  *kdf.Opts
	KDFSalt []byte
}

// StoreKey stores private key that is encrypted by key derived from input password.
func StoreKey(key heimdall.Key, pwd string, keyDirPath string, encOpt *encryption.Opts, kdfOpt *kdf.Opts) error {
	ski := key.SKI()
	keyId := key.ID()

	keyGenOpt := key.KeyGenOpt()
	if !keyGenOpt.IsValid() {
		return ErrInvalidKeyGenOpt
	}

	keyFilePath, err := makeKeyFilePath(keyId, keyDirPath)
	if err != nil {
		return err
	}

	salt := make([]byte, 8)
	_, err = rand.Read(salt)
	if err != nil {
		return err
	}

	dKey, err := kdf.DeriveKey([]byte(pwd), salt, encOpt.KeyLen, kdfOpt)
	if err != nil {
		return err
	}

	encryptedKeyBytes, err := encryption.EncryptKey(key, dKey, encOpt)
	if err != nil {
		return err
	}

	encHints := makeEncryptionHints(encOpt, kdfOpt, salt)

	jsonKeyFile, err := makeJsonKeyFile(encHints, ski, keyGenOpt, encryptedKeyBytes, key.IsPrivate())
	if err != nil {
		return err
	}

	if _, err := os.Stat(keyFilePath); os.IsNotExist(err) {
		err = ioutil.WriteFile(keyFilePath, jsonKeyFile, 0700)
		if err != nil {
			return err
		}
	}

	return nil
}

// makeKeyFilePath makes key file path (absolute) of the key file.
func makeKeyFilePath(keyFileName string, keyDirPath string) (string, error) {
	if _, err := os.Stat(keyDirPath); os.IsNotExist(err) {
		err = os.MkdirAll(keyDirPath, 0755)
		if err != nil {
			return "", err
		}
	}

	return filepath.Join(keyDirPath, keyFileName), nil
}

func makeEncryptionHints(encOpt *encryption.Opts, kdfOpt *kdf.Opts, kdfSalt []byte) *EncryptionHints {
	return &EncryptionHints{
		EncOpt:  encOpt,
		KDFOpt:  kdfOpt,
		KDFSalt: kdfSalt,
	}
}

// makeJsonKeyFile marshals keyFile struct to json format.
func makeJsonKeyFile(encHints *EncryptionHints, ski []byte, keyGenOpt heimdall.KeyGenOpts, encryptedKeyBytes []byte, isPrivate bool) ([]byte, error) {
	keyFile := KeyFile{
		SKI:          ski,
		KeyGenOpt:    keyGenOpt.ToString(),
		IsPrivate:    isPrivate,
		EncryptedKey: hex.EncodeToString(encryptedKeyBytes),
		Hints:        encHints,
	}

	return json.Marshal(keyFile)
}

// LoadKey loads private key by key ID and password.
func LoadKey(keyId heimdall.KeyID, pwd string, keyDirPath string, recoverer heimdall.KeyRecoverer) (heimdall.Key, error) {
	var keyFile KeyFile

	if _, err := os.Stat(keyDirPath); os.IsNotExist(err) {
		return nil, err
	}

	if err := heimdall.KeyIDPrefixCheck(keyId); err != nil {
		return nil, err
	}

	keyPath, err := findKeyById(keyId, keyDirPath)
	if err != nil {
		return nil, err
	}

	jsonKeyFile, err := loadJsonKeyFile(keyPath)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(jsonKeyFile, &keyFile); err != nil {
		return nil, err
	}

	if err = heimdall.SKIValidCheck(keyId, keyFile.SKI); err != nil {
		return nil, err
	}

	kdfOpt, err := kdf.NewOpts(keyFile.Hints.KDFOpt.KdfName, keyFile.Hints.KDFOpt.KdfParams)
	if err != nil {
		return nil, err
	}

	encOpt, err := encryption.NewOpts(keyFile.Hints.EncOpt.Algorithm, keyFile.Hints.EncOpt.KeyLen, keyFile.Hints.EncOpt.OpMode)
	if err != nil {
		return nil, err
	}

	dKey, err := kdf.DeriveKey([]byte(pwd), keyFile.Hints.KDFSalt, encOpt.KeyLen, kdfOpt)
	if err != nil {
		return nil, err
	}

	encryptedKeyBytes, err := hex.DecodeString(keyFile.EncryptedKey)
	if err != nil {
		return nil, err
	}

	keyBytes, err := encryption.DecryptKey(encryptedKeyBytes, dKey, encOpt)
	if err != nil {
		return nil, err
	}

	key, err := recoverer.RecoverKeyFromByte(keyBytes, keyFile.IsPrivate)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// findKeyById finds key file path by key id from file names in keystore path.
func findKeyById(keyId string, keyDirPath string) (keyPath string, err error) {
	keyPath = ""

	files, err := ioutil.ReadDir(keyDirPath)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if strings.Compare(file.Name(), keyId) == 0 {
			keyPath = filepath.Join(keyDirPath, file.Name())
			break
		}
	}

	if len(keyPath) == 0 {
		return "", ErrWrongKeyID
	}

	return keyPath, nil
}

// loadJsonKeyFile reads json formatted KeyFile struct from file.
func loadJsonKeyFile(keyPath string) (jsonKeyFile []byte, err error) {
	if len(keyPath) == 0 {
		return nil, ErrEmptyKeyPath
	}

	jsonKeyFile, err = ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	return jsonKeyFile, nil
}

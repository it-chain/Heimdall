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

// This file provides configuration of parameters for signing and verifying.

package config

import (
	"errors"

	"github.com/it-chain/heimdall"
	"github.com/it-chain/heimdall/encryption"
	"github.com/it-chain/heimdall/hashing"
	"github.com/it-chain/heimdall/hecdsa"
	"github.com/it-chain/heimdall/kdf"
)

var ErrInvalidSecLv = "invalid security level"

type Config struct {
	SecLv       int
	KeyDirPath  string
	CertDirPath string
	KeyGenOpt   heimdall.KeyGenOpts
	EncOpt      heimdall.EncryptOpts
	KdfOpt      heimdall.KeyDerivationOpts
	SigAlgo     string
	HashOpt     hashing.HashOpts
}

// NewSimpleConfig makes configuration by input security level
func NewSimpleConfig(secLv int) (conf *Config, err error) {
	conf = new(Config)
	return conf, conf.initSimpleConfig(secLv)
}

// NewDefaultConfig makes configuration by security level 192
func NewDefaultConfig() (conf *Config, err error) {
	conf = new(Config)
	return conf, conf.initSimpleConfig(192)
}

func (conf *Config) initSimpleConfig(secLv int) error {
	conf.KeyDirPath = "./.keys"
	conf.CertDirPath = "./.certs"
	conf.SigAlgo = "ECDSA"

	conf.EncOpt = encryption.NewAESEncOpts(secLv, "CTR")
	conf.KdfOpt = kdf.NewScryptOpts(kdf.DefaultScryptN, kdf.DefaultScryptR, kdf.DefaultScryptP)

	switch secLv {
	case 128:
		conf.KeyGenOpt = hecdsa.KeyGenOpts(hecdsa.ECP256)
		conf.HashOpt = hashing.HashOpts(hashing.SHA256)
	case 192:
		conf.KeyGenOpt = hecdsa.KeyGenOpts(hecdsa.ECP384)
		conf.HashOpt = hashing.HashOpts(hashing.SHA384)
	case 256:
		conf.KeyGenOpt = hecdsa.KeyGenOpts(hecdsa.ECP521)
		conf.HashOpt = hashing.HashOpts(hashing.SHA512)
	default:
		return errors.New(ErrInvalidSecLv)
	}
	return nil
}

// todo: 받을 parameter 결정..
// NewDetailConfig makes configuration by parameters corresponding to config struct members
func NewDetailConfig() (conf *Config, err error) {
	conf = new(Config)
	return conf, conf.initDetailConfig()
}

// todo: string 형태로 각각 받아서 파싱,,, 각 요소별 조건 체크..
func (conf *Config) initDetailConfig() error {
	return nil
}

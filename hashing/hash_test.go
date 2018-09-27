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

package hashing_test

import (
	"testing"

	"github.com/it-chain/heimdall/hashing"
	"github.com/stretchr/testify/assert"
)

func TestHashManager_Hash(t *testing.T) {
	data := []byte("This data will be hashed by hashManager")

	// no input data case
	digest, err := hashing.Hash(nil, hashing.SHA512)
	assert.Nil(t, digest)
	assert.Error(t, err)

	// normal case
	digest, err = hashing.Hash(data, hashing.SHA512)
	assert.NoError(t, err)
	assert.NotNil(t, digest)

	// compare between hashed data by the same hashing function
	anotherDigest, err := hashing.Hash(data, hashing.SHA512)
	assert.Equal(t, digest, anotherDigest)

	// compare between hashed data by the different hashing function
	anotherDigest, err = hashing.Hash(data, hashing.SHA256)
	assert.NotEqual(t, digest, anotherDigest)
}

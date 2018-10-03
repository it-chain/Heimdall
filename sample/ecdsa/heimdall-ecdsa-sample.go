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

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/it-chain/heimdall"
	"github.com/it-chain/heimdall/certstore"
	"github.com/it-chain/heimdall/config"
	"github.com/it-chain/heimdall/encryption"
	"github.com/it-chain/heimdall/hashing"
	"github.com/it-chain/heimdall/hecdsa"
	"github.com/it-chain/heimdall/kdf"
	"github.com/it-chain/heimdall/keystore"
	"github.com/it-chain/heimdall/mocks"
)

/*
This sample shows data to be transmitted
is signed and verified by ECDSA Key.
*/

func main() {
	// set configuration
	myConFig, err := config.NewSimpleConfig(192)
	errorCheck(err)

	defer os.RemoveAll(myConFig.CertDirPath)
	defer os.RemoveAll(myConFig.KeyDirPath)

	// Generate key pair with ECDSA algorithm.
	log.Println("generating key...")
	keyGenOpt := myConFig.KeyGenOpt
	pri, err := hecdsa.GenerateKey(keyGenOpt)
	errorCheck(err)
	log.Println("generating key success!")

	// storing key
	log.Println("storing key...")
	kdfOpt := myConFig.KdfOpt
	encOpt := myConFig.EncOpt
	keyDeriver := &kdf.ScryptKeyDeriver{}
	keyEncryptor := &encryption.AESCTREncryptor{}
	keyStorer := keystore.NewKeyStorer(kdfOpt, encOpt, keyDeriver, keyEncryptor)

	err = keyStorer.StoreKey(pri, "password", myConFig.KeyDirPath)
	errorCheck(err)
	log.Println("storing key is success!")

	// loading key
	log.Println("loading key...")
	keyId := pri.ID()

	keyDecryptor := &encryption.AESCTRDecryptor{}
	keyRecoverer := &hecdsa.KeyRecoverer{}
	loaderKeyDeriver := &kdf.ScryptKeyDeriver{}

	keyLoader := keystore.NewKeyLoader(keyDecryptor, keyRecoverer, loaderKeyDeriver)
	loadedPri, err := keyLoader.LoadKey(keyId, "password", myConFig.KeyDirPath)
	errorCheck(err)
	if loadedPri == nil {
		log.Println("loading key is failed!")
	}
	log.Println("loading key is success!")

	// using other key for testing certificate related functions
	ecPri, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	errorCheck(err)
	hPri := hecdsa.NewPriKey(ecPri)
	////////////////////// config CA for sample /////////////////////
	log.Println("requesting and receiving certificate from testCA...")
	rootCert, clientCert, sampleCA, err := configCA(&ecPri.PublicKey)
	errorCheck(err)
	log.Println("request and receive certificate from testCA success")
	//////////////////////////////////////////////////////////////////

	// storing root and client certificates
	log.Println("storing root and client certificates...")
	certStorer := certstore.CertStorer{}
	err = certStorer.StoreCert(rootCert, myConFig.CertDirPath)
	errorCheck(err)
	err = certStorer.StoreCert(clientCert, myConFig.CertDirPath)
	errorCheck(err)
	log.Println("storing root and client certificates success!")

	// loading client certificate
	log.Println("loading client certificate...")
	certLoader := certstore.CertLoader{}
	loadedClientCert, err := certLoader.LoadCert(hPri.ID(), myConFig.CertDirPath)
	errorCheck(err)
	if loadedClientCert.Equal(clientCert) {
		log.Println("loading client certificate success")
	} else {
		errorCheck(errors.New("loading client certificate failed"))
	}

	// verifying certificate chain
	log.Println("verifying certificate chain...")
	certVerifier := hecdsa.CertVerifier{}
	err = certVerifier.VerifyCertChain(clientCert, myConFig.CertDirPath)
	if err != nil {
		log.Println(err)
	} else {
		log.Println("client certificate chain is valid!")
	}

	// set CRL distribution point
	clientCert.CRLDistributionPoints = []string{sampleCA.URL}
	// verifying client cert with rootCert (can be intermediate certs between root and client)
	log.Println("verifying client certificate...")
	err = certVerifier.VerifyCert(clientCert)
	errorCheck(err)
	log.Println("client certificate is valid")

	message := []byte("This is sample message for signing and verifying.")

	// signing message (making signature)
	log.Println("signing message...")
	signer := hecdsa.Signer{}
	signerOpt := hecdsa.NewSignerOpts(hashing.SHA384)
	signature, err := signer.Sign(hPri, message, signerOpt)
	errorCheck(err)
	log.Println("signing message success!")

	/* --------- After data transmitted --------- */

	// verifying signature with public key
	log.Println("verifying signature with public key...")
	verfier := hecdsa.Verifier{}
	valid, err := verfier.Verify(hPri.PublicKey(), signature, message, signerOpt)
	errorCheck(err)
	log.Println("verifying with public key result: ", valid)

	// verifying signature with certificate
	log.Println("verifying signature with certificate...")
	valid, err = verfier.VerifyWithCert(clientCert, signature, message, signerOpt)
	errorCheck(err)
	log.Println("verifying with certificate result: ", valid)
}

func errorCheck(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

func configCA(clientPub *ecdsa.PublicKey) (rootCert, clientCert *x509.Certificate, sampleCA *httptest.Server, err error) {
	rootPri, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	errorCheck(err)
	rootPub := &rootPri.PublicKey
	hRootPub := hecdsa.NewPubKey(rootPub)

	mocks.TestRootCertTemplate.SubjectKeyId = hRootPub.SKI()

	hClientPub := hecdsa.NewPubKey(clientPub)
	mocks.TestCertTemplate.SubjectKeyId = hClientPub.SKI()

	rootDerBytes, err := x509.CreateCertificate(rand.Reader, &mocks.TestRootCertTemplate, &mocks.TestRootCertTemplate, rootPub, rootPri)
	errorCheck(err)
	clientDerBytes, err := x509.CreateCertificate(rand.Reader, &mocks.TestCertTemplate, &mocks.TestRootCertTemplate, clientPub, rootPri)
	errorCheck(err)

	rootCert, err = heimdall.DERToX509Cert(rootDerBytes)
	errorCheck(err)
	clientCert, err = heimdall.DERToX509Cert(clientDerBytes)
	errorCheck(err)

	// revoked certificate
	revokedCertificate := new(pkix.RevokedCertificate)
	revokedCertificate.SerialNumber = big.NewInt(44)
	revokedCertificate.RevocationTime = time.Now()
	revokedCertificate.Extensions = nil

	revokedCertList := []pkix.RevokedCertificate{*revokedCertificate}

	// create CRL
	crlBytes, err := rootCert.CreateCRL(rand.Reader, rootPri, revokedCertList, time.Now(), time.Now().Add(time.Hour*24))
	errorCheck(err)

	// httptest server
	sampleCA = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, string(crlBytes))
	}))

	return rootCert, clientCert, sampleCA, nil
}

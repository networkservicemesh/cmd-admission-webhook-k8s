// Copyright (c) 2023 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cert provides a manager that helps to obtain and store certificates for cmd-admission-webhook-k8s
package cert

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/networkservicemesh/cmd-admission-webhook/internal/config"
	"github.com/pkg/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// Manager provides tools for working with certificates
type Manager struct {
	config        *config.Config
	caBundle      []byte
	cert          tls.Certificate
	secretsClient coreV1Types.SecretInterface
	once          sync.Once
}

// GetConfig returns a cert.Manager config
func (m *Manager) GetConfig() *config.Config {
	return m.config
}

// NewManager provides an initialized instance of cert.Manager
func NewManager(conf *config.Config) *Manager {
	return &Manager{
		config: conf,
	}
}

// GetOrResolveCertificate tries to create certificate from Config.CertFilePath, Config.KeyFilePath or creates self signed in memory certificate.
func (m *Manager) GetOrResolveCertificate() tls.Certificate {
	m.once.Do(m.initialize)
	return m.cert
}

// GetOrResolveCABundle tries to lookup CA bundle from passed Config.CABundleFilePath or returns ca bundle from self signed in memory certificate.
func (m *Manager) GetOrResolveCABundle() []byte {
	m.once.Do(m.initialize)
	return m.caBundle
}

// GetOrResolveCertificateFromSecret attempts to obtain a certificate from a k8s secret using the specified secret name from Config.SecretName and namespace from Config.Namespace.
func (m *Manager) GetOrResolveCertificateFromSecret(ctx context.Context) tls.Certificate {
	m.initializeCertsClient()
	m.initializeSecretCert(ctx)
	return m.cert
}

func (m *Manager) initialize() {
	m.initializeCert()
	m.initializeCABundle()
}

func (m *Manager) initializeCABundle() {
	if len(m.caBundle) != 0 {
		return
	}
	r, err := os.ReadFile(m.config.CABundleFilePath)
	if err != nil {
		panic(err.Error())
	}
	m.caBundle = r
}

func (m *Manager) initializeCert() {
	if m.config.CertFilePath != "" && m.config.KeyFilePath != "" {
		cert, err := tls.LoadX509KeyPair(m.config.CertFilePath, m.config.KeyFilePath)
		if err != nil {
			panic(err.Error())
		}
		m.cert = cert
		return
	}
	m.cert = m.selfSignedInMemoryCertificate()
}

func (m *Manager) selfSignedInMemoryCertificate() tls.Certificate {
	now := time.Now()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("networkservicemesh.%v-ca", m.config.ServiceName),
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(1, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		DNSNames: []string{
			fmt.Sprintf("%v.%v", m.config.ServiceName, m.config.Namespace),
			fmt.Sprintf("%v.%v.svc", m.config.ServiceName, m.config.Namespace),
		},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		panic(err.Error())
	}

	certRaw, err := x509.CreateCertificate(rand.Reader, template, template, privateKey.Public(), privateKey)

	if err != nil {
		panic(err.Error())
	}

	pemCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certRaw,
	})

	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	result, err := tls.X509KeyPair(pemCert, pemKey)

	if err != nil {
		panic(err.Error())
	}

	m.caBundle = pemCert
	return result
}

func (m *Manager) initializeSecretCert(ctx context.Context) {
	if len(m.cert.Certificate) != 0 {
		return
	}

	if m.config.SecretName == "" {
		panic(errors.New("webhook mode 'secret' requires a non-empty Config.SecretName variable"))
	}

	secret, err := m.secretsClient.Get(ctx, m.config.SecretName, metaV1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	var pemCert []byte
	var pemKey []byte

	for key, value := range secret.Data {
		switch key {
		case config.CertFieldName:
			pemCert = value
		case config.KeyFieldName:
			pemKey = value
		}
	}

	result, err := tls.X509KeyPair(pemCert, pemKey)
	if err != nil {
		panic(err.Error())
	}

	m.cert = result
}

func (m *Manager) initializeCertsClient() {
	if m.secretsClient != nil {
		return
	}

	conf, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		panic(err.Error())
	}

	m.secretsClient = clientset.CoreV1().Secrets(m.config.Namespace)
}

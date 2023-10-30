// Copyright (c) 2021-2023 Doc.ai and/or its affiliates.
//
// Copyright (c) 2023 Cisco and/or its affiliates.
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

// Package config provides env based config and helper functions for cmd-admission-webhook-k8s
package config

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
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	coreV1Types "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// Config represents env configuration for cmd-admission-webhook-k8s
type Config struct {
	Name                  string            `default:"admission-webhook-k8s" desc:"Name of current admission webhook instance" split_words:"true"`
	ServiceName           string            `default:"default" desc:"Name of service that related to this admission webhook instance" split_words:"true"`
	Namespace             string            `default:"default" desc:"Namespace where admission webhook is deployed" split_words:"true"`
	Annotation            string            `default:"networkservicemesh.io" desc:"Name of annotation that means that the resource can be handled by admission-webhook" split_words:"true"`
	Labels                map[string]string `default:"" desc:"Map of labels and their values that should be appended for each deployment that has Config.Annotation" split_words:"true"`
	NSURLEnvName          string            `default:"NSM_NETWORK_SERVICES" desc:"Name of env that contains NSURL in initContainers/Containers" split_words:"true"`
	InitContainerImages   []string          `desc:"List of init containers that should be appended for each deployment that has Config.Annotation" split_words:"true"`
	ContainerImages       []string          `desc:"List of containers that should be appended for each deployment that has Config.Annotation" split_words:"true"`
	Envs                  []string          `desc:"Additional Envs that should be appended for each Config.ContainerImages and Config.InitContainerImages" split_words:"true"`
	WebhookMode           string            `default:"spire" desc:"Set to 'secret' to use custom certificates from k8s secret. Set to 'selfregister' to use the automatically generated webhook configuration" split_words:"true"`
	SecretName            string            `desc:"Name of the k8s secret that allows to use custom certificates for webhook" split_words:"true"`
	CertFilePath          string            `desc:"Path to certificate" split_words:"true"`
	KeyFilePath           string            `desc:"Path to RSA/Ed25519 related to Config.CertFilePath" split_words:"true"`
	CABundleFilePath      string            `desc:"Path to cabundle file related to Config.CertFilePath" split_words:"true"`
	OpenTelemetryEndpoint string            `default:"otel-collector.observability.svc.cluster.local:4317" desc:"OpenTelemetry Collector Endpoint"`
	MetricsExportInterval time.Duration     `default:"10s" desc:"interval between mertics exports" split_words:"true"`
	SidecarLimitsMemory   string            `default:"80Mi" desc:"Lower bound of the NSM sidecar memory limit (in k8s resource management units)" split_words:"true"`
	SidecarLimitsCPU      string            `default:"200m" desc:"Lower bound of the NSM sidecar CPU limit (in k8s resource management units)" split_words:"true"`
	SidecarRequestsMemory string            `default:"40Mi" desc:"Lower bound of the NSM sidecar requests memory limits (in k8s resource management units)" split_words:"true"`
	SidecarRequestsCPU    string            `default:"100m" desc:"Lower bound of the NSM sidecar requests CPU limits (in k8s resource management units)" split_words:"true"`
	envs                  []corev1.EnvVar
	secretsClient         coreV1Types.SecretInterface
	caBundle              []byte
	cert                  tls.Certificate
	mode                  Mode
	once                  sync.Once
}

// Mode type
type Mode uint32

// These are the different mode of webhook setup.
const (
	// SelfregisterMode allows you to use an automatically generated webhook configuration and certificate
	SelfregisterMode Mode = iota
	// SpireMode requires using spire configuration to obtain certificate and manually applying webhook configuration
	SpireMode
	// SecretMode requires to use k8s tls secret from the same Config.Namespace with the provided certificates
	SecretMode
)

// These are the expecting fields name in k8s certificate secret
const (
	certFieldName = "tls.crt"
	keyFieldName  = "tls.key"
)

// GetOrResolveEnvs converts on the first call passed Config.Envs into []corev1.EnvVar or returns parsed values.
func (c *Config) GetOrResolveEnvs(ctx context.Context) []corev1.EnvVar {
	c.once.Do(func() { c.initialize(ctx) })
	return c.envs
}

// GetOrResolveMode tries to parse Config.WebhookMode and return parsed values.
func (c *Config) GetOrResolveMode(ctx context.Context) Mode {
	c.once.Do(func() { c.initialize(ctx) })
	return c.mode
}

// GetOrResolveCABundle tries to lookup CA bundle from passed Config.CABundleFilePath or returns ca bundle from self signed in memory certificate.
func (c *Config) GetOrResolveCABundle(ctx context.Context) []byte {
	c.once.Do(func() { c.initialize(ctx) })
	return c.caBundle
}

// PrepareTLSConfig returns a configuration that includes certificates for proper working of http.Server, depending on the selected webhook mode.
func (c *Config) PrepareTLSConfig(ctx context.Context) (*tls.Config, error) {
	c.once.Do(func() { c.initialize(ctx) })

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if c.mode == SpireMode {
		source, err := workloadapi.NewX509Source(ctx)
		if err != nil {
			return nil, errors.Errorf("error getting x509 source: %v", err.Error())
		}
		tlsConfig.GetCertificate = tlsconfig.GetCertificate(source)

		select {
		case <-ctx.Done():
			err = source.Close()
			if err != nil {
				panic(errors.Errorf("unable to close x509 source: %v", err.Error()))
			}
		default:
		}
	} else {
		tlsConfig.Certificates = append([]tls.Certificate(nil), c.getOrResolveCertificate(ctx))
	}

	return tlsConfig, nil
}

func (c *Config) initialize(ctx context.Context) {
	c.initializeEnvs()
	c.initializeMode()
	c.initializeCert(ctx)
	c.initializeCABundle()
}

// getOrResolveCertificate tries to create certificate from Config.CertFilePath, Config.KeyFilePath or creates self signed in memory certificate.
func (c *Config) getOrResolveCertificate(ctx context.Context) tls.Certificate {
	c.once.Do(func() { c.initialize(ctx) })
	return c.cert
}

func (c *Config) initializeEnvs() {
	for _, envRaw := range c.Envs {
		kv := strings.Split(envRaw, "=")
		c.envs = append(c.envs, corev1.EnvVar{
			Name:  kv[0],
			Value: kv[1],
		})
	}
	c.envs = append(c.envs,
		corev1.EnvVar{
			Name:  "SPIFFE_ENDPOINT_SOCKET",
			Value: "unix:///run/spire/sockets/agent.sock",
		},
		corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	)
}

func (c *Config) initializeMode() {
	mode, err := parseMode(c.WebhookMode)
	if err != nil {
		panic(err.Error())
	}
	c.mode = mode
}

func (c *Config) initializeCertsClient() {
	if c.secretsClient != nil {
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

	c.secretsClient = clientset.CoreV1().Secrets(c.Namespace)
}

func (c *Config) initializeCABundle() {
	if c.mode != SelfregisterMode {
		return
	}

	if len(c.caBundle) != 0 {
		return
	}
	r, err := os.ReadFile(c.CABundleFilePath)
	if err != nil {
		panic(err.Error())
	}
	c.caBundle = r
}

func (c *Config) initializeCert(ctx context.Context) {
	switch c.mode {
	case SelfregisterMode:
		if c.CertFilePath != "" && c.KeyFilePath != "" {
			cert, err := tls.LoadX509KeyPair(c.CertFilePath, c.KeyFilePath)
			if err != nil {
				panic(err.Error())
			}
			c.cert = cert
			return
		}
		c.cert = c.selfSignedInMemoryCertificate()
	case SecretMode:
		c.initializeCertsClient()
		c.initializeSecretCert(ctx)
	}
}

func (c *Config) initializeSecretCert(ctx context.Context) {
	if len(c.cert.Certificate) != 0 {
		return
	}

	if c.SecretName == "" {
		panic(errors.New("webhook mode 'secret' requires a non-empty Config.SecretName variable"))
	}

	secret, err := c.secretsClient.Get(ctx, c.SecretName, metaV1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	var pemCert []byte
	var pemKey []byte

	for key, value := range secret.Data {
		switch key {
		case certFieldName:
			pemCert = value
		case keyFieldName:
			pemKey = value
		}
	}

	result, err := tls.X509KeyPair(pemCert, pemKey)
	if err != nil {
		panic(err.Error())
	}

	c.cert = result
}

func (c *Config) selfSignedInMemoryCertificate() tls.Certificate {
	now := time.Now()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("networkservicemesh.%v-ca", c.ServiceName),
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(1, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		DNSNames: []string{
			fmt.Sprintf("%v.%v", c.ServiceName, c.Namespace),
			fmt.Sprintf("%v.%v.svc", c.ServiceName, c.Namespace),
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

	c.caBundle = pemCert
	return result
}

// parseMode takes a string mode and returns the webhook Mode constant.
func parseMode(mode string) (Mode, error) {
	switch strings.ToLower(mode) {
	case "selfregister":
		return SelfregisterMode, nil
	case "spire":
		return SpireMode, nil
	case "secret":
		return SecretMode, nil
	}

	var m Mode
	return m, errors.Errorf("not a valid webhook mode: %s", mode)
}

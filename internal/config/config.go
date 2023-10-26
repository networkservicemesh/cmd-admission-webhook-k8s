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
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
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
	CertFieldName = "tls.crt"
	KeyFieldName  = "tls.key"
)

// GetOrResolveEnvs converts on the first call passed Config.Envs into []corev1.EnvVar or returns parsed values.
func (c *Config) GetOrResolveEnvs() []corev1.EnvVar {
	c.once.Do(c.initializeEnvs)
	return c.envs
}

// ParseMode takes a string mode and returns the webhook Mode constant.
func ParseMode(mode string) (Mode, error) {
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

func (mode Mode) marshalText() ([]byte, error) {
	switch mode {
	case SelfregisterMode:
		return []byte("selfregister"), nil
	case SpireMode:
		return []byte("spire"), nil
	case SecretMode:
		return []byte("secret"), nil
	}

	return nil, errors.Errorf("not a valid webhook mode %d", mode)
}

// String convert the Mode to a string. E.g. SelfregisterMode becomes "selfregister".
func (mode Mode) String() string {
	if m, err := mode.marshalText(); err == nil {
		return string(m)
	}

	return "unknown"
}

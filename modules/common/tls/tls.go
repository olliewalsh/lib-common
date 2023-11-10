/*
Copyright 2023 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +kubebuilder:object:generate:=true

package tls

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	// CABundleLabel added to the CA bundle secret for the namespace
	CABundleLabel = "combined-ca-bundle"
)

// Service contains server-specific TLS secret
type Service struct {
	// +kubebuilder:validation:Optional
	SecretName string `json:"secretName,omitempty"`

	// +kubebuilder:validation:Optional
	IssuerName *string `json:"issuerName,omitempty"`

	// +kubebuilder:validation:Optional
	DisableNonTLSListeners bool `json:"disableNonTLSListeners,omitempty"`
}

// Ca contains CA-specific settings, which could be used both by services (to define their own CA certificates)
// and by clients (to verify the server's certificate)
type Ca struct {
	// +kubebuilder:validation:Optional
	CaSecretName string `json:"caSecretName,omitempty"`
}

// TLS - a generic type, which encapsulates both the service and CA configurations
type TLS struct {
	Service *Service `json:"service"`
	Ca      *Ca      `json:"ca"`
}

// DeploymentResources - holding information to be passed in to any deployment require tls certificates
// +kubebuilder:object:generate:=false
type DeploymentResources struct {
	// Volumes -
	Volumes []Volume
}

// Volume -
// +kubebuilder:object:generate:=false
type Volume struct {
	// this Volume reflects a CA mount
	IsCA bool
	// Volume base of the mounts
	Volume corev1.Volume
	// VolumeMounts
	VolumeMounts []corev1.VolumeMount
	// Hash of the VolumeMounts. Note: e.g. secret.VerifySecret() can be used to validate
	// the secret holds the expected keys and returns a hash of the values of the expected fields.
	Hash string
}

// GetVolumeMounts - returns all VolumeMounts from a DeploymentResources. If caOnly
// is provided, only the Volumemounts for CA certs gets returned
func (d *DeploymentResources) GetVolumeMounts(caOnly bool) []corev1.VolumeMount {
	volumemounts := []corev1.VolumeMount{}

	for _, vol := range d.Volumes {

		// skip non CA VolumesMounts if caOnly requested
		if caOnly && !vol.IsCA {
			continue
		}

		for _, volmnt := range vol.VolumeMounts {
			// check if the VolumeMount is already in the volumes list
			f := func(v corev1.VolumeMount) bool {
				return v.Name == volmnt.Name && v.SubPath == volmnt.SubPath
			}
			if idx := slices.IndexFunc(volumemounts, f); idx < 0 {
				volumemounts = append(volumemounts, volmnt)
			}
		}
	}

	return volumemounts
}

// GetVolumes - returns all Volumes from a DeploymentResources. If caOnly
// is provided, only the Volumes for CA certs gets returned
func (d *DeploymentResources) GetVolumes(caOnly bool) []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, vol := range d.Volumes {
		// skip non CA volumes if caOnly requested
		if caOnly && !vol.IsCA {
			continue
		}

		// check if the Volume is already in the volumes list
		f := func(v corev1.Volume) bool {
			return v.Name == vol.Volume.Name
		}
		if idx := slices.IndexFunc(volumes, f); idx < 0 {
			volumes = append(volumes, vol.Volume)
		}
	}

	return volumes
}

// NewTLS - initialize and return a TLS struct
func NewTLS(ctx context.Context, h *helper.Helper, namespace string, service *Service, ca *Ca) (*TLS, error) {

	// Ensure service SecretName exists or return an error
	if service != nil && service.SecretName != "" {
		secretData, _, err := secret.GetSecret(ctx, h, service.SecretName, namespace)
		if err != nil {
			return nil, fmt.Errorf("error ensuring secret %s exists: %w", service.SecretName, err)
		}

		_, keyOk := secretData.Data["tls.key"]
		_, certOk := secretData.Data["tls.crt"]
		if !keyOk || !certOk {
			return nil, fmt.Errorf("secret %s does not contain both tls.key and tls.crt", service.SecretName)
		}
	}

	return &TLS{
		Service: service,
		Ca:      ca,
	}, nil
}

// CreateVolumeMounts - add volume mount for TLS certificate and CA certificates, this counts on openstack-operator providing CA certs with unique names
func (t *TLS) CreateVolumeMounts() []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount

	if t.Service != nil && t.Service.SecretName != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "tls-crt",
			MountPath: "/etc/pki/tls/certs/tls.crt",
			SubPath:   "tls.crt",
			ReadOnly:  true,
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "tls-key",
			MountPath: "/etc/pki/tls/certs/tls.key",
			SubPath:   "tls.key",
			ReadOnly:  true,
		})
	}

	if t.Ca != nil && t.Ca.CaSecretName != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "ca-certs",
			MountPath: "/etc/pki/ca-trust/extracted/pem",
			ReadOnly:  true,
		})
	}

	return volumeMounts
}

// CreateVolumes - add volume for TLS certificate and CA certificates
func (t *TLS) CreateVolumes() []corev1.Volume {
	var volumes []corev1.Volume

	if t.Service != nil && t.Service.SecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  t.Service.SecretName,
					DefaultMode: ptr.To[int32](0440),
				},
			},
		})
	}

	if t.Ca != nil && t.Ca.CaSecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ca-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  t.Ca.CaSecretName,
					DefaultMode: ptr.To[int32](0444),
				},
			},
		})
	}

	return volumes
}

// CreateDatabaseClientConfig - connection flags for the MySQL client
// Configures TLS connections for clients that use TLS certificates
// returns a string of mysql config statements
func (t *TLS) CreateDatabaseClientConfig() string {
	conn := []string{}
	// This assumes certificates are always injected in
	// a common directory for all services
	if t.Service.SecretName != "" {
		conn = append(conn,
			"ssl-cert=/etc/pki/tls/certs/tls.crt",
			"ssl-key=/etc/pki/tls/private/tls.key")
	}
	// Client uses a CA certificate that gets merged
	// into the pod's CA bundle by kolla_start
	if t.Ca.CaSecretName != "" {
		conn = append(conn,
			"ssl-ca=/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem")
	}
	if len(conn) > 0 {
		conn = append([]string{"ssl=1"}, conn...)
	}
	return strings.Join(conn, "\n")
}

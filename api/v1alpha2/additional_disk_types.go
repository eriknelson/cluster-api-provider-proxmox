/*
Copyright 2023-2026 IONOS Cloud.

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

package v1alpha2

import (
	"fmt"
	"strings"
)

// DiskType represents the type of disk attachment.
type DiskType string

const (
	// DiskTypeStoragePool indicates a disk from a Proxmox storage pool.
	// Example value: "uldum:vm-100-disk-1".
	DiskTypeStoragePool DiskType = "storagePool"

	// DiskTypePassthrough indicates a physical disk passthrough.
	// Example value: "/dev/disk/by-id/ata-ST20000NM007D-3DJ103_ZVTA2PZC".
	DiskTypePassthrough DiskType = "passthrough"
)

// AdditionalDisk defines a pre-existing disk to attach to the VM.
// This supports both storage pool disks and physical disk passthrough.
type AdditionalDisk struct {
	// name is a unique identifier for this disk within the VM configuration.
	// Used for tracking and debugging purposes.
	// +required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`

	// device is the Proxmox disk device name to use for attachment.
	// Must not conflict with the boot volume device.
	// +required
	// +kubebuilder:validation:MinLength=1
	// Example values: scsi1, scsi2, virtio1, ide1, sata1
	// +kubebuilder:validation:Pattern=`^(scsi|virtio|ide|sata)[0-9]+$`
	Device string `json:"device,omitempty"`

	// type specifies whether this is a storage pool disk or passthrough device.
	// +required
	// +kubebuilder:validation:Enum=storagePool;passthrough
	Type DiskType `json:"type,omitempty"`

	// value is the disk reference.
	// For storagePool type: "storage:disk-name" (e.g., "uldum:vm-100-disk-1")
	// For passthrough type: device path (e.g., "/dev/disk/by-id/ata-ST20000NM007D-3DJ103_ZVTA2PZC")
	// +required
	// +kubebuilder:validation:MinLength=1
	Value string `json:"value,omitempty"`

	// cache specifies the cache mode for the disk.
	// Valid values: none, writethrough, writeback, unsafe, directsync
	// +optional
	// +kubebuilder:validation:Enum=none;writethrough;writeback;unsafe;directsync
	Cache *string `json:"cache,omitempty"`

	// iothread enables IO threading for this disk.
	// Only applies to SCSI and VirtIO disk types.
	// +optional
	IOThread *bool `json:"iothread,omitempty"`

	// discard specifies whether to pass discard/trim requests to the underlying storage.
	// Valid values: on, ignore
	// +optional
	// +kubebuilder:validation:Enum=on;ignore
	Discard *string `json:"discard,omitempty"`

	// ssd enables SSD emulation for this disk.
	// Useful for passing TRIM support to the guest OS.
	// +optional
	SSD *bool `json:"ssd,omitempty"`
}

// FormatDiskValue returns the formatted Proxmox disk configuration string
// including all optional parameters.
func (d *AdditionalDisk) FormatDiskValue() string {
	value := d.Value

	var opts []string
	if d.Cache != nil {
		opts = append(opts, fmt.Sprintf("cache=%s", *d.Cache))
	}
	if d.IOThread != nil && *d.IOThread {
		opts = append(opts, "iothread=1")
	}
	if d.Discard != nil {
		opts = append(opts, fmt.Sprintf("discard=%s", *d.Discard))
	}
	if d.SSD != nil && *d.SSD {
		opts = append(opts, "ssd=1")
	}

	if len(opts) > 0 {
		value = fmt.Sprintf("%s,%s", value, strings.Join(opts, ","))
	}

	return value
}

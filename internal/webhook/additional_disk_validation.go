/*
Copyright 2023-2025 IONOS Cloud.

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

package webhook

import (
	"fmt"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"

	infrav1 "github.com/ionos-cloud/cluster-api-provider-proxmox/api/v1alpha2"
)

// deviceNamePattern validates disk device names like scsi1, virtio0, ide2, sata0.
var deviceNamePattern = regexp.MustCompile(`^(scsi|virtio|ide|sata)[0-9]+$`)

// storagePoolPattern validates storage pool disk references like "uldum:vm-100-disk-1".
var storagePoolPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+:.+$`)

// passthroughPattern validates passthrough device paths like "/dev/disk/by-id/...".
var passthroughPattern = regexp.MustCompile(`^/dev/`)

// ValidateAdditionalDisks validates the additional disks configuration.
func ValidateAdditionalDisks(machine *infrav1.ProxmoxMachine) error {
	if machine.Spec.Disks == nil || len(machine.Spec.Disks.AdditionalDisks) == 0 {
		return nil
	}

	gk, name := machine.GroupVersionKind().GroupKind(), machine.GetName()

	// Track used device names to detect duplicates
	usedDevices := make(map[string]bool)
	usedNames := make(map[string]bool)

	// Get boot volume device if set
	var bootDevice string
	if machine.Spec.Disks.BootVolume != nil {
		bootDevice = machine.Spec.Disks.BootVolume.Disk
		usedDevices[bootDevice] = true
	}

	for i, disk := range machine.Spec.Disks.AdditionalDisks {
		fieldPath := field.NewPath("spec", "disks", "additionalDisks").Index(i)

		// Validate device name format
		if !deviceNamePattern.MatchString(disk.Device) {
			return apierrors.NewInvalid(
				gk,
				name,
				field.ErrorList{
					field.Invalid(
						fieldPath.Child("device"),
						disk.Device,
						"device must match pattern (scsi|virtio|ide|sata)[0-9]+"),
				})
		}

		// Check for device name conflict with boot volume
		if disk.Device == bootDevice {
			return apierrors.NewInvalid(
				gk,
				name,
				field.ErrorList{
					field.Invalid(
						fieldPath.Child("device"),
						disk.Device,
						fmt.Sprintf("device conflicts with boot volume device %q", bootDevice)),
				})
		}

		// Check for duplicate device names
		if usedDevices[disk.Device] {
			return apierrors.NewInvalid(
				gk,
				name,
				field.ErrorList{
					field.Duplicate(
						fieldPath.Child("device"),
						disk.Device),
				})
		}
		usedDevices[disk.Device] = true

		// Check for duplicate names
		if usedNames[disk.Name] {
			return apierrors.NewInvalid(
				gk,
				name,
				field.ErrorList{
					field.Duplicate(
						fieldPath.Child("name"),
						disk.Name),
				})
		}
		usedNames[disk.Name] = true

		// Validate value format based on type
		if err := validateDiskValue(disk, fieldPath); err != nil {
			return apierrors.NewInvalid(gk, name, field.ErrorList{err})
		}

		// Validate IOThread is only used with compatible device types
		if disk.IOThread != nil && *disk.IOThread {
			if !strings.HasPrefix(disk.Device, "scsi") && !strings.HasPrefix(disk.Device, "virtio") {
				return apierrors.NewInvalid(
					gk,
					name,
					field.ErrorList{
						field.Invalid(
							fieldPath.Child("iothread"),
							*disk.IOThread,
							"iothread is only supported for scsi and virtio devices"),
					})
			}
		}
	}

	return nil
}

// validateDiskValue validates the disk value format based on the disk type.
func validateDiskValue(disk infrav1.AdditionalDisk, fieldPath *field.Path) *field.Error {
	switch disk.Type {
	case infrav1.DiskTypeStoragePool:
		if !storagePoolPattern.MatchString(disk.Value) {
			return field.Invalid(
				fieldPath.Child("value"),
				disk.Value,
				"storage pool disk value must be in format 'storage:disk-name' (e.g., 'uldum:vm-100-disk-1')")
		}
	case infrav1.DiskTypePassthrough:
		if !passthroughPattern.MatchString(disk.Value) {
			return field.Invalid(
				fieldPath.Child("value"),
				disk.Value,
				"passthrough disk value must be a device path starting with '/dev/' (e.g., '/dev/disk/by-id/ata-...')")
		}
	default:
		return field.Invalid(
			fieldPath.Child("type"),
			disk.Type,
			"type must be 'storagePool' or 'passthrough'")
	}

	return nil
}

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

package vmservice

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/ionos-cloud/cluster-api-provider-proxmox/pkg/proxmox"
	"github.com/ionos-cloud/cluster-api-provider-proxmox/pkg/scope"
)

// ReconcileAdditionalDisks attaches pre-existing disks to the VM.
// This function is idempotent - it will skip disks that are already attached.
func ReconcileAdditionalDisks(ctx context.Context, machineScope *scope.MachineScope) error {
	disks := machineScope.ProxmoxMachine.Spec.Disks
	if disks == nil || len(disks.AdditionalDisks) == 0 {
		// Nothing to do
		return nil
	}

	vm := machineScope.VirtualMachine
	if vm.IsRunning() || ptr.Deref(machineScope.ProxmoxMachine.Status.Initialization.Provisioned, false) {
		// We only want to do this before the machine was started or is ready.
		// v0.8.1+ moved the readiness signal from Status.Ready to
		// Status.Initialization.Provisioned (CAPI v1beta2 convention).
		return nil
	}

	machineScope.V(4).Info("reconciling additional disks", "count", len(disks.AdditionalDisks))

	// Get existing disk configuration from the VM
	existingDisks := vm.VirtualMachineConfig.MergeDisks()

	var vmOptions []proxmox.VirtualMachineOption
	for _, disk := range disks.AdditionalDisks {
		// Check if disk is already attached (idempotent)
		if _, exists := existingDisks[disk.Device]; exists {
			machineScope.V(4).Info("disk already attached, skipping", "device", disk.Device, "name", disk.Name)
			continue
		}

		machineScope.V(4).Info("attaching additional disk", "device", disk.Device, "name", disk.Name, "type", disk.Type)
		vmOptions = append(vmOptions, proxmox.VirtualMachineOption{
			Name:  disk.Device,
			Value: disk.FormatDiskValue(),
		})
	}

	if len(vmOptions) == 0 {
		// All disks already attached
		return nil
	}

	task, err := machineScope.InfraCluster.ProxmoxClient.ConfigureVM(ctx, vm, vmOptions...)
	if err != nil {
		return errors.Wrapf(err, "failed to attach additional disks to VM %s", machineScope.Name())
	}

	// Set task reference for tracking
	machineScope.ProxmoxMachine.Status.TaskRef = ptr.To(string(task.UPID))

	return nil
}

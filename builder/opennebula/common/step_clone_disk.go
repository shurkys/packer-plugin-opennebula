package opennebula

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// StepCloneDisk creates clones of disks that are not CDROM.
type StepCloneDisk struct {
}

// Run executes the step to create clones of disks.
func (s *StepCloneDisk) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	controller := config.Controller
	vmInfoRaw, ok := state.Get("VM_Info").(*vm.VM)

	if !ok {
		ui.Error("Failed to convert VM_Info to *vm.VM")
		return multistep.ActionHalt
	}

	ui.Say("Checking for remaining disks to clone...")

	// Check if there are remaining disks
	disksToClone := vmInfoRaw.Template.GetDisks()
	if len(disksToClone) == 0 {
		ui.Say("No remaining disks to clone.")
		return multistep.ActionContinue
	}

	// Wait for the VM to be powered off
	err := WaitForResourceState(vmInfoRaw.ID, "POWEROFF", "vm", state, config.EjectISODelay)
	if err != nil {
		ui.Error(fmt.Sprintf("Error waiting for the VM to be powered off: %s", err))
		ui.Say("Powering off VM manually...")
		err := controller.VM(vmInfoRaw.ID).PoweroffHard()
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to power off VM manually: %s", err))
			return multistep.ActionHalt
		}
	}

	ui.Say("Creating clones of remaining disks...")
	if state.Get("ClonedDiskIDs") == nil {
		state.Put("ClonedDiskIDs", []int{})
	}
	for _, disk := range vmInfoRaw.Template.GetDisks() {
		//image_ID,_ := disk.GetInt("IMAGE_ID")
		disk_ID, _ := disk.GetInt("DISK_ID")
		disk_TYPE, _ := disk.GetStr("TYPE")
		if disk_TYPE != "CDROM" {
			ui.Say(fmt.Sprintf("Cloning disk ID: %d", disk_ID))
			cloneID, err := controller.VM(vmInfoRaw.ID).Disk(disk_ID).Saveas(config.SnapshotConfig.Snapshot_Name, "", -1)
			//cloneID, err := controller.Image(image_ID).Clone(config.SnapshotConfig.Snapshot_Name,config.SnapshotConfig.Snapshot_DatastoreID)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to create clone of disk ID %d: %s", disk_ID, err))
				return multistep.ActionHalt
			}
			// Wait for the VM to be powered off
			err = WaitForResourceState(vmInfoRaw.ID, "POWEROFF", "vm", state, 15*time.Minute)
			if err != nil {
				ui.Error(fmt.Sprintf("Error waiting for the VM to be powered off: %s", err))
				return multistep.ActionHalt
			}
			err = WaitForResourceState(cloneID, "READY", "image", state, 15*time.Minute)
			if err != nil {
				ui.Error(fmt.Sprintf("Error waiting for the image to become READY: %s", err))
				return multistep.ActionHalt
			}
			state.Put("ClonedDiskIDs", append(state.Get("ClonedDiskIDs").([]int), cloneID))
		}
	}
	ui.Say("Clones created successfully.")
	return multistep.ActionContinue
}

// Cleanup performs cleanup tasks if necessary.
func (s *StepCloneDisk) Cleanup(state multistep.StateBag) {
	// Cleanup, if necessary
}

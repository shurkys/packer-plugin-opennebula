package opennebula

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// StepDetachISO detaches the currently attached ISO file from a virtual machine if any.
type StepDetachISO struct {
}

// Run executes the step to detach the ISO file.
func (s *StepDetachISO) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	controller := config.Controller
	vmInfoRaw, ok := state.Get("VM_Info").(*vm.VM)

	if !ok {
		ui.Error("Failed to convert VM_Info to *vm.VM")
		return multistep.ActionHalt
	}

	// Debug log statements
	ui.Message(fmt.Sprintf("config.EjectISO: %v", config.EjectISO))

	// Check if state uses ISO file and has the need to eject it
	if !config.EjectISO {
		ui.Say("No ISO file to eject.")
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

	ui.Say("Detaching ISO from the VM...")

	// Detach the ISO file from the VM
	for disk_ID, disk := range vmInfoRaw.Template.GetDisks() {
		disk_type, _ := disk.GetStr("TYPE")
		if disk_type == "CDROM" {
			err := controller.VM(vmInfoRaw.ID).Disk(disk_ID).Detach()
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to detach ISO: %s", err))
				return multistep.ActionHalt
			}
			ui.Say(fmt.Sprintf("ISO %d detached successfully.", disk_ID))
		}
	}

	ui.Say("Waiting for the VM to be powered off...")

	// Wait for the VM to be in POWEROFF state
	err = WaitForResourceState(vmInfoRaw.ID, "POWEROFF", "vm", state, 5*time.Minute)
	if err != nil {
		ui.Error(fmt.Sprintf("Error waiting for the VM to be powered off: %s", err))
		return multistep.ActionHalt
	}

	ui.Say("ISO detached successfully.")
	return multistep.ActionContinue
}

// Cleanup performs cleanup tasks if necessary.
func (s *StepDetachISO) Cleanup(state multistep.StateBag) {
	// Cleanup, if necessary
}

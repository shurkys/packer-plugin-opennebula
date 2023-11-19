package opennebula

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepPowerOffVM struct {
	ShutdownMethod string
}

// Shuts down the virtual machine using the specified method.
func (s *StepPowerOffVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	controller := config.Controller
	vmInfoRaw, ok := state.Get("VM_Info").(*vm.VM)

	if !ok {
		ui.Error("Failed to convert VM_Info to *vm.VM")
		return multistep.ActionHalt
	}

	ui.Say("Shutting down the virtual machine...")

	// Determine the shutdown method
	ShutdownMethod := s.ShutdownMethod
	if ShutdownMethod == "" {
		ShutdownMethod = "poweroff" // Default
	}

	// Power off or power off hard based on the chosen method
	switch ShutdownMethod {
	case "poweroff":
		err := controller.VM(vmInfoRaw.ID).Poweroff()
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to power off the VM: %s", err))
			return multistep.ActionHalt
		}
	case "poweroff_hard":
		err := controller.VM(vmInfoRaw.ID).PoweroffHard()
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to power off the VM (hard): %s", err))
			return multistep.ActionHalt
		}
	default:
		ui.Error("Invalid shutdown method specified")
		return multistep.ActionHalt
	}

	ui.Say("Waiting for the VM to be powered off...")

	// Wait for the VM to be in POWEROFF state
	err := WaitForResourceState(vmInfoRaw.ID, "POWEROFF", "vm", state, 5*time.Minute)
	if err != nil {
		ui.Error(fmt.Sprintf("Error waiting for the VM to be powered off: %s", err))
		return multistep.ActionHalt
	}

	ui.Say("VM successfully powered off.")
	return multistep.ActionContinue
}

func (s *StepPowerOffVM) Cleanup(state multistep.StateBag) {
	// Cleanup, if necessary
}

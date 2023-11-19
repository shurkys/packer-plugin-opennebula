package opennebula

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepStartVM struct{}

// Starts the virtual machine.
func (s *StepStartVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	controller := config.Controller
	vmInfoRaw, ok := state.Get("VM_Info").(*vm.VM)

	if !ok {
		ui.Error("Failed to convert VM_Info to *vm.VM")
		return multistep.ActionHalt
	}

	ui.Say("Starting the virtual machine...")

	// Start the VM
	err := controller.VM(vmInfoRaw.ID).Resume()
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to start the VM: %s", err))
		return multistep.ActionHalt
	}

	ui.Say("Waiting for the VM to be running...")

	// Wait for the VM to be in RUNNING state
	err = WaitForResourceState(vmInfoRaw.ID, "RUNNING", "vm", state, 5*time.Minute)
	if err != nil {
		ui.Error(fmt.Sprintf("Error waiting for the VM to be running: %s", err))
		return multistep.ActionHalt
	}

	ui.Say("VM successfully started.")
	return multistep.ActionContinue
}

func (s *StepStartVM) Cleanup(state multistep.StateBag) {
	// Cleanup, if necessary
}

package opennebula

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/shared"
	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	vmk "github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm/keys"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepCreateVM struct {
	//config Config
	VMTemplateConfig  VMTemplateConfig
	OpenNebulaConnect OpenNebulaConnect
}

func (s *StepCreateVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	ui.Say("Creating OpenNebula VM Template...")

	imageIDs, ok := state.Get("ImageIDs").([]int)
	if !ok || len(imageIDs) == 0 {
		ui.Error("Failed to get source image IDs from state")
		return multistep.ActionHalt
	}

	tpl := vm.NewTemplate()

	tpl.Add(vmk.Name, s.VMTemplateConfig.Name)
	tpl.CPU(s.VMTemplateConfig.CPU)
	tpl.Memory(s.VMTemplateConfig.Memory)
	tpl.VCPU(s.VMTemplateConfig.VCPU)
	tpl.CPUModel(s.VMTemplateConfig.CPUModel)

	// Add disks based on the provided image IDs or names
	for _, imageID := range imageIDs {
		disk := tpl.AddDisk()
		disk.Add(shared.ImageID, imageID)
		// Add other disk-related configurations if needed
	}

	tpl.AddIOGraphic(vmk.GraphicType, s.VMTemplateConfig.GraphicsType)
	tpl.AddIOGraphic(vmk.Keymap, s.VMTemplateConfig.GraphicsKeymap)
	tpl.AddIOGraphic(vmk.Listen, s.VMTemplateConfig.GraphicsListen)

	for _, nicConf := range s.VMTemplateConfig.NICs {
		nic := tpl.AddNIC()
		nic.Add(shared.Network, nicConf.Network)
	}

	tpl.AddCtx(vmk.SetHostname, "$NAME")
	tpl.AddCtx(vmk.SSHPubKey, "$USER[SSH_PUBLIC_KEY]")
	tpl.AddCtx(vmk.NetworkCtx, "YES")
	tpl.AddCtx("USER_DATA", base64.StdEncoding.EncodeToString([]byte(s.VMTemplateConfig.UserData)))
	tpl.AddCtx("USER_DATA_ENCODING", "base64")
	tpl.AddCtx("AUTOSTART", "true")
	tpl.AddOS(vmk.Arch, s.VMTemplateConfig.OSArch)
	tpl.AddOS(vmk.Boot, s.VMTemplateConfig.OSBoot)

	controller := s.OpenNebulaConnect.Controller
	//ui.Say(tpl.String())

	vmID, err := controller.VMs().Create(tpl.String(), false)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to create VM: %s", err))
		return multistep.ActionHalt
	}

	state.Put("vmID", vmID)
	ui.Say(fmt.Sprintf("VM created with ID: %d", vmID))

	err = WaitForResourceState(vmID, "RUNNING", "vm", state, 10*time.Minute)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to start the OpenNebula VM: %s", err))
		return multistep.ActionHalt
	}

	vm, err := controller.VM(vmID).Info(false)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to fetch VM information: %s", err))
		return multistep.ActionHalt
	}
	//ui.Say(fmt.Sprintf("VM Info: %s", vm))
	state.Put("VM_Info", vm)

	vncPortStr, err := vm.Template.GetIOGraphic(vmk.Port)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to get VNC port: %s", err))
		return multistep.ActionHalt
	}

	vncPort, err := strconv.Atoi(vncPortStr)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to convert VNC port to int: %s", err))
		return multistep.ActionHalt
	}
	ui.Say(fmt.Sprintf("OpenNebula VM VncPort: %d", vncPort))
	state.Put("vncPort", vncPort)

	ui.Say("OpenNebula VM is now running.")

	return multistep.ActionContinue
}

func (s *StepCreateVM) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	ui.Say("Cleaning up OpenNebula VM...")

	if existingImages, ok := state.Get("existingImages").(bool); ok && existingImages {
		ui.Say("Skipping cleanup as existing images are used.")
		return
	}

	vmID, ok := state.Get("vmID").(int)
	if !ok {
		ui.Error("Failed to get VM ID from state during cleanup.")
		return
	}

	ui.Say(fmt.Sprintf("Deleting OpenNebula VM with ID: %d", vmID))

	controller := s.OpenNebulaConnect.Controller
	err := controller.VM(vmID).TerminateHard()
	if err != nil {
		ui.Error(fmt.Sprintf("Error deleting the OpenNebula VM: %s", err))
		return
	}

	err = WaitForResourceState(vmID, "LCM_INIT", "vm", state, 5*time.Minute)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to delete the OpenNebula VM: %s", err))
	} else {
		ui.Say("OpenNebula VM deleted successfully.")
	}
}

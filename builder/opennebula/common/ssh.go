// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package opennebula

import (
	"errors"
	"fmt"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

func CommHost(host string, ui packersdk.Ui) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		if host != "" {
			ui.Message(fmt.Sprintf("Using host value: %s", host))
			return host, nil
		}

		vmInfoRaw, ok := state.Get("VM_Info").(*vm.VM)
		if !ok {
			return "", errors.New("failed to convert VM_Info to *vm.VM")
		}
		ips := vmInfoRaw.Template.GetNICs()
		if len(ips) == 0 {
			ui.Error("No NIC information found for the VM")
			return "", errors.New("no NIC information found for the VM")
		}

		ip, err := ips[0].Get("IP")
		if err != nil {
			ui.Error(fmt.Sprintf("Failed to get IP: %s", err))
			return "", err
		}

		ui.Message(fmt.Sprintf("IP: %s", ip))

		// Assuming the first NIC contains the IP address
		return ip, nil
	}
}

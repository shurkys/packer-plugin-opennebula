package opennebula

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/vm"
	"github.com/hashicorp/packer-plugin-sdk/bootcommand"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/mitchellh/go-vnc"
)

type StepVNCBootCommand struct {
	VNCConfig   bootcommand.VNCConfig `mapstructure:",squash"`
	VNCPassword string                `mapstructure:"vm_vnc_password,omitempty"`
	VNCIP       string                `mapstructure:"vnc_ip" required:"false"`
	VNCPort     int                   `mapstructure:"vnc_port" required:"false"`
	BootSteps   [][]string            `mapstructure:"boot_steps" required:"false"`
}

func (s *StepVNCBootCommand) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	debug := state.Get("debug").(bool)
	ui := state.Get("ui").(packersdk.Ui)

	if int64(s.VNCConfig.BootWait) > 0 {
		ui.Say(fmt.Sprintf("Waiting %s for boot...", s.VNCConfig.BootWait))
		select {
		case <-time.After(s.VNCConfig.BootWait):
			break
		case <-ctx.Done():
			return multistep.ActionHalt
		}
	}
	vncIP := s.VNCIP
	if vncIP == "" {
		vncIPFromVM, err := getVNCIP(state, ui)
		if err != nil {
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		vncIP = vncIPFromVM
	}

	vncPort := state.Get("vncPort").(int)

	err := checkVNCConnectivity(vncIP, vncPort, ui)
	if err != nil {
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Connecting to VM via VNC (%s:%d)", vncIP, vncPort))

	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", vncIP, vncPort))
	if err != nil {
		err := fmt.Errorf("Error connecting to VNC: %s", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	defer conn.Close()

	var auth []vnc.ClientAuth

	if len(s.VNCPassword) > 0 {
		auth = []vnc.ClientAuth{&vnc.PasswordAuth{Password: s.VNCPassword}}
	} else {
		auth = []vnc.ClientAuth{new(vnc.ClientAuthNone)}
	}

	client, err := vnc.Client(conn, &vnc.ClientConfig{Auth: auth, Exclusive: false})
	if err != nil {
		err := fmt.Errorf("Error handshaking with VNC: %s", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	defer client.Close()

	var pauseFn multistep.DebugPauseFn
	if debug {
		pauseFn = state.Get("pauseFn").(multistep.DebugPauseFn)
	}

	command := s.VNCConfig.FlatBootCommand()
	bootSteps := s.BootSteps

	if len(command) > 0 {
		bootSteps = [][]string{{command}}
	}

	ui.Say(fmt.Sprintf("Typing Command: %s", command))

	ui.Say("Typing the boot commands over VNC...")

	d := bootcommand.NewVNCDriver(client, s.VNCConfig.BootKeyInterval)

	for _, step := range bootSteps {
		if len(step) == 0 {
			continue
		}

		var description string

		if len(step) >= 2 {
			description = string(step[1])
		} else {
			description = ""
		}

		if len(description) > 0 {
			ui.Say(fmt.Sprintf("Typing boot command for: %s", description))
		}

		var httpServerIP string
		var err error
		c := state.Get("config").(*Config)
		ui.Say(fmt.Sprintf("HTTPAddress: %s", c.HTTPConfig.HTTPAddress))
		if c.HTTPAddress != "0.0.0.0" {
			httpServerIP = c.HTTPAddress
		} else {
			httpServerIP, err = hostIP(c.HTTPInterface)
			if err != nil {
				err := fmt.Errorf("Failed to determine host IP: %s", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
		ui.Say(fmt.Sprintf("httpServerIP: %s", httpServerIP))

		state.Put("httpServerIP", httpServerIP)

		configCtx := &interpolate.Context{
			Data: map[string]interface{}{
				"HTTPIP":   httpServerIP,
				"HTTPPort": state.Get("http_port").(int),
			},
		}

		command, err := interpolate.Render(command, configCtx)
		if err != nil {
			err := fmt.Errorf("Error preparing boot command: %s", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		seq, err := bootcommand.GenerateExpressionSequence(command)
		if err != nil {
			err := fmt.Errorf("Error generating boot command: %s", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		if err := seq.Do(ctx, d); err != nil {
			err := fmt.Errorf("Error running boot command: %s", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		if pauseFn != nil {
			var message string

			if len(description) > 0 {
				message = fmt.Sprintf("boot description: \"%s\", command: %s", description, command)
			} else {
				message = fmt.Sprintf("boot_command: %s", command)
			}

			pauseFn(multistep.DebugLocationAfterRun, message, state)
		}
	}
	return multistep.ActionContinue
}

func (s *StepVNCBootCommand) Cleanup(state multistep.StateBag) {}

func hostIP(ifname string) (string, error) {
	var addrs []net.Addr
	var err error

	if ifname != "" {
		iface, err := net.InterfaceByName(ifname)
		if err != nil {
			return "", err
		}
		addrs, err = iface.Addrs()
		if err != nil {
			return "", err
		}
	} else {
		addrs, err = net.InterfaceAddrs()
		if err != nil {
			return "", err
		}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("No host IP found")
}

func getVNCIP(state multistep.StateBag, ui packersdk.Ui) (string, error) {
	vmInfoRaw, ok := state.Get("VM_Info").(*vm.VM)
	if !ok {
		return "", errors.New("Failed to convert VM_Info to *vm.VM")
	}

	if len(vmInfoRaw.HistoryRecords) == 0 {
		return "", errors.New("VM_Info does not contain any history records")
	}

	vncIP := vmInfoRaw.HistoryRecords[0].Hostname
	return vncIP, nil
}

func checkVNCConnectivity(vncIP string, vncPort int, ui packersdk.Ui) error {
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", vncIP, vncPort))
	if err != nil {
		return fmt.Errorf("Error connecting to VNC: %s", err)
	}
	defer conn.Close()
	return nil
}

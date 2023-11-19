package opennebula

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type Builder struct {
	BuilderID string
	config    Config
	PreSteps  []multistep.Step
	runner    multistep.Runner
}

// NewSharedBuilder creates a new shared builder for OpenNebula.
func NewSharedBuilder(builderID string, config Config, PreSteps []multistep.Step) *Builder {
	return &Builder{
		config:    config,
		PreSteps:  PreSteps,
		BuilderID: builderID,
	}
}

// Run executes a Packer build and returns a packersdk.Artifact representing
// a OpenNebula image.
func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {
	ui.Say("[Info] Starting OpenNebula Packer Build...")

	//log.Printf("[Debug] Config: %s", b.config)
	client, controller, err := NewOpenNebulaConnect(b.config.OpenNebulaURL, b.config.Username, b.config.Password, b.config.Insecure)
	if err != nil {
		return nil, err
	}
	b.config.Controller = controller
	state := new(multistep.BasicStateBag)
	state.Put("OpenNebulaClient", client)
	state.Put("OpenNebulaController", controller)
	state.Put("ui", ui)
	state.Put("debug", b.config.Debug)
	state.Put("config", &b.config)
	state.Put("hook", hook)

	steps := []multistep.Step{}

	// Define execution steps.
	PreCommonSteps := []multistep.Step{
		&StepProcessImages{
			Images: b.config.ImageConfigs,
		},
		&StepCreateVM{
			VMTemplateConfig:  b.config.VMTemplateConfig,
			OpenNebulaConnect: b.config.OpenNebulaConnect,
		},
		commonsteps.HTTPServerFromHTTPConfig(&b.config.HTTPConfig),
	}

	PostCommonSteps := []multistep.Step{
		// &communicator.StepConnect for OpenNebula
		&communicator.StepConnect{
			Config:    &b.config.Comm,
			Host:      CommHost(b.config.Comm.Host(), ui),
			SSHConfig: b.config.Comm.SSHConfigFunc(),
		},
		&commonsteps.StepProvision{},

		&commonsteps.StepCleanupTempKeys{
			Comm: &b.config.Comm,
		},
		&StepPowerOffVM{
			ShutdownMethod: "poweroff",
		},
		&StepCloneDisk{},
		// Other steps to create an ISO image
	}
	steps = append(steps, PreCommonSteps...)
	steps = append(steps, b.PreSteps...)
	steps = append(steps, PostCommonSteps...)

	artifact := Artifact{
		ImageID:    "",
		StateData:  map[string]interface{}{},
		Client:     client,
		Controller: controller,
	}

	// Configure the runner and run the steps.
	b.runner = commonsteps.NewRunnerWithPauseFn(steps, b.config.PackerConfig, ui, state)
	b.runner.Run(ctx, state)

	// If there was an error, return that
	if rawErr, ok := state.GetOk("error"); ok {
		ui.Error(fmt.Sprintf("[Error] Build failed: %s", rawErr))
		return nil, rawErr.(error)
	}

	// If we were interrupted or cancelled, then just exit.
	if _, ok := state.GetOk(multistep.StateCancelled); ok {
		ui.Error("[Error] Build was cancelled.")
		return nil, errors.New("Build was cancelled.")
	}

	if _, ok := state.GetOk(multistep.StateHalted); ok {
		ui.Error("[Error] Build was halted.")
		return nil, errors.New("Build was halted.")
	}

	ui.Say("[Info] OpenNebula Packer Build completed successfully.")
	return &artifact, nil
}

package image

import (
	"context"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	onecommon "packer-plugin-opennebula/builder/opennebula/common"
)

// The unique id for the builder
const BuilderID = "opennebula.image"

type Builder struct {
	config onecommon.Config
}

// Builder implements packersdk.Builder
var _ packersdk.Builder = &Builder{}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	warnings, errs := b.config.Prepare(raws...)
	if errs != nil {
		return nil, warnings, errs
	}
	if b.config.HTTPPortMin == 0 {
		b.config.HTTPPortMin = 8000
	}

	if b.config.HTTPPortMax == 0 {
		b.config.HTTPPortMax = 9000
	}

	return nil, warnings, nil
}

func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {
	state := new(multistep.BasicStateBag)
	state.Put("image-config", &b.config)

	ImagePreSteps := []multistep.Step{
		// 	// Add your pre-steps here, if needed.
	}

	// ImagePostSteps := []multistep.Step{
	// 	// Add your post-steps here, if needed.
	// }

	sb := onecommon.NewSharedBuilder(BuilderID, b.config, ImagePreSteps)
	return sb.Run(ctx, ui, hook)
}

type imageVMCreator struct{}

func (*imageVMCreator) Create(client interface{}, vmConfig interface{}, state multistep.StateBag) error {
	// Add your logic for creating the VM in OpenNebula here
	return nil
}

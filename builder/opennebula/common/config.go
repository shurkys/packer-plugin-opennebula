//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,ImageConfig,NICConfig,SnapshotConfig
package opennebula

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/communicator"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/hashicorp/packer-plugin-sdk/uuid"
)

type Config struct {
	common.PackerConfig    `mapstructure:",squash"`
	commonsteps.HTTPConfig `mapstructure:",squash"`
	Global                 `mapstructure:",squash"`
	OpenNebulaConnect      `mapstructure:",squash"`
	VMTemplateConfig       VMTemplateConfig `mapstructure:",squash"`
	StepVNCBootCommand     `mapstructure:",squash"`
	Comm                   communicator.Config `mapstructure:",squash"`
	Ctx                    interpolate.Context `mapstructure-to-hcl2:",skip"`
}

type Global struct {
	Datastore      string         `mapstructure:"datastore"` //  required:"true"`
	Debug          bool           `mapstructure:"debug"`
	EjectISO       bool           `mapstructure:"eject_iso"`
	EjectISODelay  time.Duration  `mapstructure:"eject_iso_delay"`
	SnapshotConfig SnapshotConfig `mapstructure:"snapshot"`
	ImageConfigs   []ImageConfig  `mapstructure:"image"`
}

type VMTemplateConfig struct {
	Name           string      `mapstructure:"vm_name"`
	CPU            float64     `mapstructure:"vm_cpu"`
	CPUModel       string      `mapstructure:"vm_cpu_model"`
	Description    string      `mapstructure:"vm_description"`
	EnableVNC      bool        `mapstructure:"enable_vnc"`
	GraphicsKeymap string      `mapstructure:"vm_graphics_keymap"`
	GraphicsListen string      `mapstructure:"vm_graphics_listen"`
	GraphicsType   string      `mapstructure:"vm_graphics_type"`
	Hypervisor     string      `mapstructure:"vm_hypervisor"`
	Logo           string      `mapstructure:"vm_logo"`
	Memory         int         `mapstructure:"vm_memory"`
	NICs           []NICConfig `mapstructure:"vm_nics"` // Используется массив для сетевых интерфейсов
	OSArch         string      `mapstructure:"vm_os_arch"`
	OSBoot         string      `mapstructure:"vm_os_boot"`
	VCPU           int         `mapstructure:"vm_vcpu"`
	UserData       string      `mapstructure:"vm_user_data"`
}

// ImageConfig holds the configuration settings for the image
type ImageConfig struct {
	Image_ID             int      `mapstructure:"id"`
	Image_Name           string   `mapstructure:"name"`
	Image_Type           string   `mapstructure:"type"`
	Image_DatastoreID    int      `mapstructure:"datastore_id"`
	Image_Persistent     bool     `mapstructure:"persistent"`
	Image_Lock           string   `mapstructure:"lock"`
	Image_Permissions    int      `mapstructure:"permissions"`
	Image_Group          string   `mapstructure:"group"`
	Image_Path           string   `mapstructure:"path"`
	Image_DevPrefix      string   `mapstructure:"dev_prefix"`
	Image_Target         string   `mapstructure:"target"`
	Image_Driver         string   `mapstructure:"driver"`
	Image_Format         string   `mapstructure:"format"`
	Image_Size           int      `mapstructure:"size"`
	Image_CloneFromImage string   `mapstructure:"clone_from_image"`
	Image_Tags           []string `mapstructure:"tags"`
}

type NICConfig struct {
	Network string `mapstructure:"network"`
}

type SnapshotConfig struct {
	Snapshot_Name        string `mapstructure:"name"`
	Snapshot_DatastoreID int    `mapstructure:"datastore_id"`
}

func (c *Config) Prepare(raws ...interface{}) ([]string, error) {
	err := config.Decode(c, &config.DecodeOpts{
		//PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &c.Ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"boot_command",
				"boot_steps",
				"qemuargs",
			},
		},
	}, raws...)
	if err != nil {
		return nil, err
	}

	var errs *packersdk.MultiError
	var warnings []string

	errs = packersdk.MultiErrorAppend(errs, c.Comm.Prepare(&c.Ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.VNCConfig.Prepare(&c.Ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.HTTPConfig.Prepare(&c.Ctx)...)

	if c.OpenNebulaURL == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("OpenNebulaURL must be specified"))
	}
	if c.Username == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("Username must be specified"))
	}
	if c.Password == "" {
		errs = packersdk.MultiErrorAppend(errs, errors.New("Password must be specified"))
	}

	// Установка значений по умолчанию
	if c.VMTemplateConfig.Name == "" {
		c.VMTemplateConfig.Name = fmt.Sprintf("packer-%s", c.PackerBuildName)
	}

	// Проверка обязательных полей
	// idSet := c.SourceImageConfig.SourceImageID != 0
	// urlSet := c.SourceImageConfig.SourceImageURL != ""
	// nameSet := c.SourceImageConfig.SourceImageName != ""

	// if idSet == urlSet && urlSet == nameSet && idSet {
	//     errs = packersdk.MultiErrorAppend(errs, errors.New("exactly one of source_image_id, source_image_url, or source_image_name must be specified"))
	// }

	// if c.SourceImageConfig.SourceImageURL != "" && c.DatastoreID == 0 {
	//     errs = packersdk.MultiErrorAppend(errs, errors.New("when specifying Path, DatastoreID must also be specified"))
	// }

	// if c.SourceImageConfig.SourceImageURL == "" && c.DatastoreID != 0 {
	//     errs = packersdk.MultiErrorAppend(errs, errors.New("when specifying DatastoreID, Path must also be specified"))
	// }

	// if c.SourceImageConfig.SourceImageName == "" {
	//     errs = packersdk.MultiErrorAppend(errs, errors.New("ImageName must be specified"))
	// }

	// If we are not given an explicit keypair, ssh_password or ssh_private_key_file,
	// then create a temporary one, but only if the temporary_keypair_name has not
	// been provided.
	if c.Comm.SSHKeyPairName == "" && c.Comm.SSHTemporaryKeyPairName == "" &&
		c.Comm.SSHPrivateKeyFile == "" && c.Comm.SSHPassword == "" {
		c.Comm.SSHTemporaryKeyPairName = fmt.Sprintf("packer_%s", uuid.TimeOrderedUUID())
	}

	if errs != nil && len(errs.Errors) > 0 {
		return warnings, errs
	}

	return warnings, nil
}

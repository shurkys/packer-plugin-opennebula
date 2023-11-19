package opennebula

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca/schemas/image"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// StepProcessImages processes multiple image configurations.
type StepProcessImages struct {
	Images []ImageConfig
}

func (s *StepProcessImages) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	// Проверяем, был ли ImageIDs установлен в состоянии
	if state.Get("ImageIDs") == nil {
		state.Put("ImageIDs", []int{})
	}
	// Добавим новое поле для отслеживания ID созданных образов
	if state.Get("CreatedImageIDs") == nil {
		state.Put("CreatedImageIDs", []int{})
	}

	for _, imageConfig := range s.Images {
		ui.Say(fmt.Sprintf("Processing image: %s", imageConfig.Image_Name))
		// Check if image ID or name is provided
		if imageConfig.Image_ID != 0 {
			ui.Say(fmt.Sprintf("Using existing OpenNebula image ID: %d", imageConfig.Image_ID))
			state.Put("ImageIDs", append(state.Get("ImageIDs").([]int), imageConfig.Image_ID))
		} else {
			existingID, _ := s.getImageIDByName(imageConfig.Image_Name, ui, state)

			if existingID != 0 {
				ui.Say(fmt.Sprintf("Using existing OpenNebula image Name: %s", imageConfig.Image_Name))
				state.Put("ImageIDs", append(state.Get("ImageIDs").([]int), existingID))
			} else {
				// Process  images
				s.prepareImage(imageConfig, ui, state)
			}
		}
	}

	return multistep.ActionContinue
}

func (s *StepProcessImages) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	createdImageIDs := state.Get("CreatedImageIDs").([]int)

	if createdImageIDs != nil && len(createdImageIDs) > 0 {
		ui.Say("Cleaning up created images...")
		c := state.Get("config").(*Config)

		for _, imageID := range createdImageIDs {
			ui.Say(fmt.Sprintf("Deleting OpenNebula image ID: %d", imageID))
			err := WaitForResourceState(imageID, "READY", "image", state, 5*time.Minute)
			if err != nil {
				ui.Error(fmt.Sprintf("Error waiting for the image to become READY: %s", err))
			}
			err = c.Controller.Image(imageID).Delete()
			if err != nil {
				ui.Error(fmt.Sprintf("Error deleting image ID %d: %s", imageID, err))
			}
		}
		ui.Say("Cleanup completed.")
	} else {
		ui.Say("No images were created during the build, nothing to clean up.")
	}
}

func (s *StepProcessImages) getImageIDByName(name string, ui packersdk.Ui, state multistep.StateBag) (int, error) {
	c := state.Get("config").(*Config)
	imageID, err := c.Controller.Images().ByName(name)
	if err != nil {
		ui.Error(fmt.Sprintf("Error getting image ID by name: %s, new image will be created", err))
		return 0, err
	}

	if imageID != 0 {
		ui.Say(fmt.Sprintf("Using existing OpenNebula image ID: %d", imageID))
	} else {
		ui.Error(fmt.Sprintf("Image with name '%s' not found in OpenNebula", name))
		return 0, errors.New("Image not found")
	}

	return imageID, nil
}

func (s *StepProcessImages) prepareImage(config ImageConfig, ui packersdk.Ui, state multistep.StateBag) multistep.StepAction {
	ui.Say("Preparing disk image...")
	c := state.Get("config").(*Config)
	var ID int

	// Check if CloneFromImage is specified
	if config.Image_CloneFromImage != "" {
		var err error

		// Check if CloneFromImage is ID or Name
		if id, err := strconv.Atoi(config.Image_CloneFromImage); err == nil {
			// Clone using ID directly
			ID, err = CloneImage(id, config.Image_Name, config.Image_DatastoreID, state)
		} else {
			// Clone using Name, get the ID first
			id, err = s.getImageIDByName(config.Image_CloneFromImage, ui, state)
			if err == nil {
				ID, err = CloneImage(id, config.Image_Name, config.Image_DatastoreID, state)
			}
		}

		if err != nil {
			ui.Error(fmt.Sprintf("Error cloning image: %s", err))
			return multistep.ActionHalt
		}

		// Use the cloned image ID for further steps
		config.Image_CloneFromImage = strconv.Itoa(ID)
		ui.Say(fmt.Sprintf("Image cloned successfully. New Image ID: %d", ID))
	} else {
		// If CloneFromImage is not specified, create a new image
		tpl := &image.Template{}
		tpl.Add("name", config.Image_Name)
		tpl.Add("datastore_id", config.Image_DatastoreID)
		tpl.Add("type", config.Image_Type)
		tpl.Add("path", config.Image_Path)
		tpl.Add("permissions", config.Image_Permissions)
		tpl.Add("persistent", config.Image_Persistent)
		tpl.Add("lock", config.Image_Lock)
		tpl.Add("dev_prefix", config.Image_DevPrefix)
		tpl.Add("target", config.Image_Target)
		tpl.Add("driver", config.Image_Driver)
		tpl.Add("format", config.Image_Format)
		tpl.Add("size", config.Image_Size)
		tpl.Add("group", config.Image_Group)
		tpl.Add("tags", config.Image_Tags)

		ID, err := c.Controller.Images().Create(tpl.String(), uint(config.Image_DatastoreID))
		if err != nil {
			ui.Error(fmt.Sprintf("Error creating the OpenNebula image: %s", err))
			return multistep.ActionHalt
		}
		err = WaitForResourceState(ID, "READY", "image", state, 5*time.Minute)
		if err != nil {
			ui.Error(fmt.Sprintf("Error waiting for the image to become READY: %s", err))
			return multistep.ActionHalt
		}

		ui.Say("OpenNebula disk image created successfully.")

	}
	state.Put("ImageIDs", append(state.Get("ImageIDs").([]int), ID))
	// Add the ID of the created image to CreatedImageIDs
	state.Put("CreatedImageIDs", append(state.Get("CreatedImageIDs").([]int), ID))
	return multistep.ActionContinue
}

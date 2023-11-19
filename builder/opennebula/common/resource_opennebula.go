package opennebula

import (
	"fmt"
	"strings"
	"time"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

const (
	defaultMinTimeout = 20
	defaultTimeout    = time.Duration(defaultMinTimeout) * time.Minute
)

// GetVMState возвращает текущее состояние виртуальной машины.
func GetVMState(ID int, state multistep.StateBag) (string, string, error) {
	vmInfos, err := state.Get("OpenNebulaController").(*goca.Controller).VM(ID).Info(false)
	if err != nil {
		// Если виртуальная машина была удалена и не существует, то возвращаем пустые значения
		if strings.Contains(err.Error(), "Error getting VM") && strings.Contains(err.Error(), "not found") {
			return "", "", nil
		}
		return "", "", err
	}

	vmState, lcmState, err := vmInfos.State()
	if err != nil {
		return "", "", err
	}

	return vmState.String(), lcmState.String(), nil
}

// WaitForResourceState ожидает достижения желаемого состояния указанного ресурса.
func WaitForResourceState(ID int, desiredState string, resourceType string, state multistep.StateBag, timeout time.Duration) error {
	ui := state.Get("ui").(packersdk.Ui)

	// Use the provided timeout or the default timeout if not provided
	if timeout == 0 {
		timeout = defaultTimeout
	}

	startTime := time.Now()

	for {
		currentTime := time.Now()
		elapsedTime := currentTime.Sub(startTime)

		if elapsedTime >= timeout {
			return fmt.Errorf("Timed out waiting for the %s to reach the desired state", resourceType)
		}

		ui.Message(fmt.Sprintf("Refreshing %s state...", resourceType))

		switch resourceType {
		case "image":
			imgInfos, err := state.Get("OpenNebulaController").(*goca.Controller).Image(ID).Info(false)
			if err != nil {
				// Если изображение было удалено и не существует, то завершаем ожидание
				if strings.Contains(err.Error(), "Error getting image") && strings.Contains(err.Error(), "not found") {
					return nil
				}
				return err
			}

			imgState, err := imgInfos.State()
			if err != nil {
				return err
			}

			ui.Say(fmt.Sprintf("%s (ID:%d, name:%s) is currently in state %s", resourceType, imgInfos.ID, imgInfos.Name, imgState))

			if imgState.String() == desiredState {
				return nil
			}

		case "vm":
			vmState, lcmState, err := GetVMState(ID, state)
			if err != nil {
				return err
			}

			ui.Say(fmt.Sprintf("%s (ID:%d) is currently in state %s/%s", resourceType, ID, vmState, lcmState))

			if lcmState == desiredState || vmState == desiredState {
				return nil
			}

		default:
			return fmt.Errorf("Unsupported resource type: %s", resourceType)
		}

		time.Sleep(10 * time.Second) // Подождать перед следующей попыткой
	}
}

// CloneImage клонирует указанный образ в OpenNebula и возвращает ID нового.
func CloneImage(sourceImageID int, targetImageName string, targetDatastoreID int, state multistep.StateBag) (int, error) {
	ui := state.Get("ui").(packersdk.Ui)

	// Получение контроллера OpenNebula из состояния
	controller, ok := state.Get("OpenNebulaController").(*goca.Controller)
	if !ok {
		return 0, fmt.Errorf("Failed to convert OpenNebulaController to *goca.Controller")
	}

	// Метод Clone() контроллера образа OpenNebula
	cloneID, err := controller.Image(sourceImageID).Clone(targetImageName, targetDatastoreID)
	if err != nil {
		ui.Error(fmt.Sprintf("Error cloning image ID %d: %s", sourceImageID, err))
		return 0, err
	}

	// Ожидание завершения клонирования
	err = WaitForResourceState(cloneID, "READY", "image", state, defaultTimeout)
	if err != nil {
		ui.Error(fmt.Sprintf("Error waiting for cloned image to become READY: %s", err))
		return 0, err
	}

	ui.Say(fmt.Sprintf("Image cloned successfully. New Image ID: %d", cloneID))
	return cloneID, nil
}

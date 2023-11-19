package opennebula

import (
	"fmt"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type Artifact struct {
	ImageID    string
	StateData  map[string]interface{}
	builderID  string
	Client     *goca.Client
	Controller *goca.Controller
}

// Artifact implements packersdk.Artifact
var _ packersdk.Artifact = &Artifact{}

// BuilderId returns the builder ID.
func (a *Artifact) BuilderId() string {
	return a.builderID
}

// Files returns the files represented by the artifact.
func (a *Artifact) Files() []string {
	return nil
}

func (a *Artifact) Id() string {
	return a.ImageID
}

func (a *Artifact) String() string {
	return fmt.Sprintf("An image was created: %v", a.ImageID)
}

// State returns specific details from the artifact.
func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

func (a *Artifact) Destroy() error {
	return nil
}

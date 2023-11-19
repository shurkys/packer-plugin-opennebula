package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/shurkys/packer-plugin-opennebula/builder/opennebula/image"
	"github.com/shurkys/packer-plugin-opennebula/builder/opennebula/iso"
	"github.com/shurkys/packer-plugin-opennebula/version"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("iso", new(iso.Builder))
	pps.RegisterBuilder("image", new(image.Builder))
	pps.SetVersion(version.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

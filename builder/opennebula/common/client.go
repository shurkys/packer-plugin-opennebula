package opennebula

import (
	"crypto/tls"
	"log"
	"net/http"

	"github.com/OpenNebula/one/src/oca/go/src/goca"
)

type OpenNebulaConnect struct {
	OpenNebulaURL string           `mapstructure:"opennebula_url"`
	Username      string           `mapstructure:"username"`
	Password      string           `mapstructure:"password"`
	Insecure      bool             `mapstructure:"insecure"`
	Client        *goca.Client     `mapstructure-to-hcl2:",skip"`
	Controller    *goca.Controller `mapstructure-to-hcl2:",skip"`
}

func NewOpenNebulaConnect(OpenNebulaURL, Username, Password string, Insecure bool) (*goca.Client, *goca.Controller, error) {
	log.Print("NewOpenNebulaConnect is starting....")

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: Insecure},
	}
	clientConfig := goca.NewConfig(Username, Password, OpenNebulaURL)
	client := goca.NewClient(clientConfig, &http.Client{Transport: tr})
	controller := goca.NewController(client)

	versionOpenNebula, err := controller.SystemVersion()
	if err != nil {
		return nil, nil, err
	}

	log.Print("Connected to OpenNebula versio: ", versionOpenNebula)
	return client, controller, nil
}

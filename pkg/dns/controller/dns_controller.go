package controller

import (
	"github.com/openshift/origin/pkg/dns"
	"github.com/golang/glog"
	"os"
	"encoding/json"
)

//To be a DNS controller you must provide the facilities for parsing the DNS config into objects
type Interface interface{
	ParseConfig(configFilePath string) *dns.DNSConfig
}

//Default impl of the DNS controller.
//Requires a DNSServer implementation to run as well as either a config file string OR
//a preparsed config object to be set.  If no config object is set the we'll try to parse it
//from the fiel in ConfigFile
type DNSController struct{
	DNSServer dns.DNSServer 	//server implementation
	Config *dns.DNSConfig	 	//optional parsed config (will be set based on file in ConfigFile if not set)
	ConfigFile string			//full path to config file to parse if not setting Config
}

//Run the controller by parsing the config if necessary, asking the DNSServer to write it into
//the implementation's format, and starting the server
func (c *DNSController) Run(){
	if c.Config == nil {
		config, err := c.ParseConfig(c.ConfigFile)
		c.Config = config

		if err != nil {
			glog.Errorf("Error parsing config for DNS: %v", err)
			return
		}
	}

	if err := c.DNSServer.WriteConfig(c.Config); err != nil {
		glog.Errorf("Error writing config for DNS: %v", err)
		return
	}

	c.DNSServer.Run()
}

//Parse a config file into objects
func (c *DNSController) ParseConfig(configFilePath string) (*dns.DNSConfig, error) {
	configFile, err := os.Open(configFilePath)

	if err != nil {
		return nil, err
	}

		decoder := json.NewDecoder(configFile)
		dnsConfig := &dns.DNSConfig{}

	if err = decoder.Decode(dnsConfig); err != nil {
		return nil, err
	}

	return dnsConfig, nil
}

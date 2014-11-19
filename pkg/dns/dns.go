package dns

//DNS Implementations must implement this interface to be used by the DNSController
type DNSServer interface{
	WriteConfig(config *DNSConfig) error 		//write the config in the implementation's format
	Run()										//start the server
}

//DNS configuration is broken into shards. Each shard may have many routers.
type DNSConfig struct {
	Shards []ShardConfig `json:shards,omitempty yaml:shards,omitempty`
}

type ShardConfig struct {
	Name string `json:name,omitempty yaml:name,omitempty`						//human readable name of the shard
	Pattern string `json:pattern,omitempty yaml:pattern,omitempty`				//DNS zone compatible pattern
	RouterList []Router `json:routerlist,omitempty yaml:routerlist,omitempty`	//list of routers to add to the zone
}

type Router struct {
	Name string `json:name,omitempty yaml:name,omitempty`						//human readable name of the router
	IP string `json:ip,omitempty yaml:ip,omitempty`								//accessible ip address
}

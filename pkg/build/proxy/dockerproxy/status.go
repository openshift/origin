package dockerproxy

type Status struct {
	Addresses []string
}

func NewStatus(proxy *Proxy) *Status {
	if proxy == nil {
		return nil
	}
	status := &Status{
		Addresses: proxy.normalisedAddrs,
	}
	return status
}

package nftables

type Data struct {
	AllowedTCPPorts []string
	AllowedUDPPorts []string
}

const Template = `
table ip my_filter {
    chain input {
        type filter hook input priority 0; policy drop;

        iifname "lo" accept;

        # Hard-coded rule to allow SSH traffic for safety
        tcp dport 22 accept;
		
		{{if (len .AllowedTCPPorts) gt 0}}
        tcp dport { {{range .AllowedTCPPorts}}{{.}}, {{end}} } accept;
		{{end}}

		{{if (len .AllowedUDPPorts) gt 0}}
        udp dport { {{range .AllowedUDPPorts}}{{.}}, {{end}} } accept;
		{{end}}
    }
}
`

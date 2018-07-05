# go-nfnetlink - A native Go library for interacting with netfilter subsystems

A library for communicating with Linux netfilter subsystems over netlink sockets.

## What is 'netfilter netlink'?

Linux/net/netfilter/nfnetlink.c:

    Netfilter messages via netlink sockets.  Allows for user space protocol helpers and general
    trouble making from userspace.

Netfilter is composed of several subsystems in the Linux kernel, some of which provide access from userland
over a netlink socket interface.  The protocol API for accessing these subsystems share a common set of protocol conventions
called nfnetlink (netfilter netlink).

## What is the nfqueue package?

A library for the netfilter queue subsystem built on top of the nfnetlink layer.

Here is a basic example of how to use it:

Set up IPTables 

    # iptables -A OUTPUT -p icmp -j NFQUEUE --queue-num 1 --queue-bypass
    
Read ICMP packets from queue number 1

    q := nfqueue.NewNFQueue(1)
    
    ps, err := q.Open()
    if err != nil {
            fmt.Printf("Error opening NFQueue: %v\n", err)
            os.Exit(1)
    }
    defer q.Close()

    for p := range ps {
            fmt.Printf("Packet: %v\n", p.Packet)
            p.Accept()
    }


## How can I implement support for other netfilter subsystems?

You'll probably have to read the C library code or the Linux kernel source to learn about the protocol as there
is usually no documentation at all.  Look at nfqueue for an example of how to implement the protocol using
the nfnetlink layer.

We plan to add some basic support for conntrack in the near future.  Pull requests welcome for new features and
subsystems.



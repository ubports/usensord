package dbus

import (
	"errors"
	"io/ioutil"
	"net"
	"net/url"
	"strings"
)

type transport interface {
	Dial() (net.Conn, error)
}

func newTransport(address string) (transport, error) {
	if len(address) == 0 {
		return nil, errors.New("Unknown address type")
	}
	// Split the address into transport type and options.
	transportType := address[:strings.Index(address, ":")]
	options := make(map[string]string)
	for _, option := range strings.Split(address[len(transportType)+1:], ",") {
		pair := strings.SplitN(option, "=", 2)
		key, err := url.QueryUnescape(pair[0])
		if err != nil {
			return nil, err
		}
		value, err := url.QueryUnescape(pair[1])
		if err != nil {
			return nil, err
		}
		options[key] = value
	}

	switch transportType {
	case "unix":
		if abstract, ok := options["abstract"]; ok {
			return &unixTransport{"@" + abstract}, nil
		} else if path, ok := options["path"]; ok {
			return &unixTransport{path}, nil
		} else {
			return nil, errors.New("unix transport requires 'path' or 'abstract' options")
		}
	case "tcp", "nonce-tcp":
		address := options["host"] + ":" + options["port"]
		var family string
		switch options["family"] {
		case "", "ipv4":
			family = "tcp4"
		case "ipv6":
			family = "tcp6"
		default:
			return nil, errors.New("Unknown family for tcp transport: " + options["family"])
		}
		if transportType == "tcp" {
			return &tcpTransport{address, family}, nil
		} else {
			nonceFile := options["noncefile"]
			return &nonceTcpTransport{address, family, nonceFile}, nil
		}
	// These can be implemented later as needed
	case "launchd":
		// Perform newTransport() on contents of
		// options["env"] environment variable
	case "systemd":
		// Only used when systemd is starting the message bus,
		// so probably not needed in a client library.
	case "unixexec":
		// exec a process with a socket hooked to stdin/stdout
	}

	return nil, errors.New("Unhandled transport type " + transportType)
}

type unixTransport struct {
	Address string
}

func (trans *unixTransport) Dial() (net.Conn, error) {
	return net.Dial("unix", trans.Address)
}

type tcpTransport struct {
	Address, Family string
}

func (trans *tcpTransport) Dial() (net.Conn, error) {
	return net.Dial(trans.Family, trans.Address)
}

type nonceTcpTransport struct {
	Address, Family, NonceFile string
}

func (trans *nonceTcpTransport) Dial() (net.Conn, error) {
	data, err := ioutil.ReadFile(trans.NonceFile)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial(trans.Family, trans.Address)
	if err != nil {
		return nil, err
	}
	// Write the nonce data to the socket
	if _, err := conn.Write(data); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

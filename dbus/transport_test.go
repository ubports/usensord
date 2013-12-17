package dbus

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"net"
	"path"
)

func (s *S) TestNewTransportUnix(c *C) {
	trans, err := newTransport("unix:path=/tmp/dbus%3dsock")
	c.Check(err, Equals, nil)
	unixTrans, ok := trans.(*unixTransport)
	c.Check(ok, Equals, true)
	c.Check(unixTrans.Address, Equals, "/tmp/dbus=sock")

	// And for abstract namespace sockets:
	trans, err = newTransport("unix:abstract=/tmp/dbus%3dsock")
	c.Check(err, Equals, nil)
	unixTrans, ok = trans.(*unixTransport)
	c.Check(ok, Equals, true)
	c.Check(unixTrans.Address, Equals, "@/tmp/dbus=sock")
}

func (s *S) TestUnixTransportDial(c *C) {
	socketFile := path.Join(c.MkDir(), "bus.sock")
	listener, err := net.Listen("unix", socketFile)
	c.Assert(err, IsNil)
	trans, err := newTransport(fmt.Sprintf("unix:path=%s", socketFile))
	c.Assert(err, IsNil)

	errChan := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
		errChan <- err
	}()

	conn, err := trans.Dial()
	c.Assert(err, IsNil)
	conn.Close()
	// Was the other end of the connection established correctly?
	err = <-errChan
	c.Check(err, IsNil)
	listener.Close()
}

func (s *S) TestNewTransportTcp(c *C) {
	trans, err := newTransport("tcp:host=localhost,port=4444")
	c.Check(err, Equals, nil)
	tcpTrans, ok := trans.(*tcpTransport)
	c.Check(ok, Equals, true)
	c.Check(tcpTrans.Address, Equals, "localhost:4444")
	c.Check(tcpTrans.Family, Equals, "tcp4")

	// And with explicit family:
	trans, err = newTransport("tcp:host=localhost,port=4444,family=ipv4")
	c.Check(err, Equals, nil)
	tcpTrans, ok = trans.(*tcpTransport)
	c.Check(ok, Equals, true)
	c.Check(tcpTrans.Address, Equals, "localhost:4444")
	c.Check(tcpTrans.Family, Equals, "tcp4")

	trans, err = newTransport("tcp:host=localhost,port=4444,family=ipv6")
	c.Check(err, Equals, nil)
	tcpTrans, ok = trans.(*tcpTransport)
	c.Check(ok, Equals, true)
	c.Check(tcpTrans.Address, Equals, "localhost:4444")
	c.Check(tcpTrans.Family, Equals, "tcp6")
}

func (s *S) TestTcpTransportDial(c *C) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	addr := listener.Addr().(*net.TCPAddr)
	address := fmt.Sprintf("tcp:host=%s,port=%d", addr.IP.String(), addr.Port)
	trans, err := newTransport(address)
	c.Assert(err, IsNil)

	errChan := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
		}
		errChan <- err
	}()

	conn, err := trans.Dial()
	c.Assert(err, IsNil)
	conn.Close()
	// Was the other end of the connection established correctly?
	err = <-errChan
	c.Check(err, IsNil)
	listener.Close()
}

func (s *S) TestNewTransportNonceTcp(c *C) {
	trans, err := newTransport("nonce-tcp:host=localhost,port=4444,noncefile=/tmp/foo")
	c.Check(err, Equals, nil)
	nonceTcpTrans, ok := trans.(*nonceTcpTransport)
	c.Check(ok, Equals, true)
	c.Check(nonceTcpTrans.Address, Equals, "localhost:4444")
	c.Check(nonceTcpTrans.Family, Equals, "tcp4")
	c.Check(nonceTcpTrans.NonceFile, Equals, "/tmp/foo")
}

func (s *S) TestNonceTcpTransportDial(c *C) {
	nonceFile := path.Join(c.MkDir(), "nonce-file")
	nonceData := []byte("nonce-data")
	c.Assert(ioutil.WriteFile(nonceFile, nonceData, 0600), IsNil)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	addr := listener.Addr().(*net.TCPAddr)
	address := fmt.Sprintf("nonce-tcp:host=%s,port=%d,noncefile=%s", addr.IP.String(), addr.Port, nonceFile)
	trans, err := newTransport(address)
	c.Assert(err, IsNil)

	errChan := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
		// The client starts by writing the nonce data to the socket.
		data := make([]byte, 4096)
		n, err := conn.Read(data)
		if err != nil {
			conn.Close()
			errChan <- err
			return
		}
		if !bytes.Equal(data[:n], nonceData) {
			err = errors.New("Did not receive nonce data")
		}
		conn.Close()
		errChan <- err
	}()

	conn, err := trans.Dial()
	c.Assert(err, IsNil)
	conn.Close()
	// Was the other end of the connection established correctly?
	err = <-errChan
	c.Check(err, IsNil)
	listener.Close()
}

package dbus

import (
	"fmt"
	. "launchpad.net/gocheck"
)

type callTest struct {
	dest          string
	path          ObjectPath
	iface, method string
	args          []interface{}
	validate      func(*Message) error
}

var callTests = []callTest{
	{"org.freedesktop.Notifications", "/org/freedesktop/Notifications",
		"org.freedesktop.Notifications", "Notify",
		[]interface{}{
			"go-dbus", uint32(0),
			"info", "testing go-dbus", "test_body",
			[]string{}, map[string]Variant{},
			int32(2000)},
		func(*Message) error {
			return nil
		}},
}

func (test callTest) Call(c *Connection) error {
	proxy := c.Object(test.dest, test.path)
	reply, err := proxy.Call(test.iface, test.method, test.args...)
	if err != nil {
		return fmt.Errorf("failed Method.Call: %v", err)
	}
	if err = test.validate(reply); err != nil {
		err = fmt.Errorf("failed validation: %v", err)
	}
	return err
}

func (s *S) TestDBus(c *C) {
	bus, err := Connect(SessionBus)
	c.Assert(err, IsNil)
	defer bus.Close()

	for i, test := range callTests {
		err = test.Call(bus)
		if err != nil {
			c.Errorf("callTest %d: %v", i, err)
		}
	}
}

func (s *S) TestConnectionConnectSessionBus(c *C) {
	bus, err := Connect(SessionBus)
	c.Assert(err, IsNil)
	c.Check(bus.Close(), IsNil)
}

func (s *S) TestConnectionConnectSystemBus(c *C) {
	bus, err := Connect(SystemBus)
	c.Assert(err, IsNil)
	c.Check(bus.Close(), IsNil)
}

func (s *S) TestConnectionRegisterMessageFilter(c *C) {
	bus, err := Connect(SessionBus)
	c.Assert(err, IsNil)
	defer bus.Close()

	filter := bus.RegisterMessageFilter(func(msg *Message) *Message {
		// Make a change that shows the filter ran.
		if msg.Type == TypeMethodReturn {
			if err := msg.AppendArgs("Added by filter"); err != nil {
				c.Error(err)
			}
		}
		return msg
	})
	c.Check(filter, NotNil)
	defer bus.UnregisterMessageFilter(filter)

	msg := NewMethodCallMessage(BUS_DAEMON_NAME, BUS_DAEMON_PATH, BUS_DAEMON_IFACE, "GetId")
	reply, err := bus.SendWithReply(msg)
	c.Assert(err, IsNil)

	var busId, extra string
	c.Assert(reply.Args(&busId, &extra), IsNil)
	c.Assert(extra, Equals, "Added by filter")
}

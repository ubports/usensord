/*
 * Copyright 2013 Canonical Ltd.
 *
 * Authors:
 * Michael Frey: michael.frey@canonical.com
 *
 * This file is part of usensord.
 *
 * usensord is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; version 3.
 *
 * usensord is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package haptic

import (
	"fmt"
	"launchpad.net/usensord/dbus"
	"log"
	"os"
)

var (
	err    error
	conn   *dbus.Connection
	logger *log.Logger
)

const (
	HAPTIC_DBUS_IFACE = "com.canonical.usensord.haptic"
	HAPTIC_DEVICE     = "/sys/class/timed_output/vibrator/enable"
)

func watchDBusMethodCalls(msgChan <-chan *dbus.Message) {

	var duration uint32
	var reply *dbus.Message
	var pattern  []uint32

	for msg := range msgChan {
		switch {
		case msg.Interface == HAPTIC_DBUS_IFACE && msg.Member == "On":
			msg.Args(&duration)
			logger.Printf("Received On() method call %d", duration)
			err = On(duration)
			if err == nil {
				reply = dbus.NewMethodReturnMessage(msg)
			} else {
				reply = dbus.NewMethodReturnMessage(nil)
			}
			conn.Send(reply)
		case msg.Interface == HAPTIC_DBUS_IFACE && msg.Member == "Pattern":
			msg.Args(&pattern)
			logger.Printf("Received Pattern() method call %d", duration)
			err = Pattern(pattern)
			if err == nil {
				reply = dbus.NewMethodReturnMessage(msg)
			} else {
				reply = dbus.NewMethodReturnMessage(nil)
			}
			conn.Send(reply)
		case msg.Interface == HAPTIC_DBUS_IFACE && msg.Member == "Off":
			logger.Println("Received Off() method call")
			if err == nil {
				reply = dbus.NewMethodReturnMessage(msg)
			} else {
				reply = dbus.NewMethodReturnMessage(nil)
			}
			conn.Send(reply)
		default:
			logger.Println("Received unkown method call")
			reply := dbus.NewErrorMessage(msg, "org.freedesktop.DBus.Error.UnknownMethod", "Unknown method")
			if err := conn.Send(reply); err != nil {
				logger.Println("Could not send reply:", err)
			}
		}
	}

}

func On(duration uint32) error {

	logger.Println("In On function")
	fi, err := os.Create(HAPTIC_DEVICE)
	if err != nil {
		logger.Println("Error opening haptic device")
		return err
	}

	if _, err := fi.WriteString(fmt.Sprintf("%d", duration)); err != nil {
		fi.Close()
		return err
	}

	fi.Close()
	return nil
}

func Pattern(duration []uint32) error {

	var err error = nil
	var x int = 0

	for _, t := range duration {
		if (x == 0) {
			err = On (uint32(t))
			x = 1
			if (err != nil) {
				return err
			}
		} else {
			time.Sleep (time.Duration(t) * time.Millisecond)
			x = 0
		}
	}

	return err
}

/*Initialize Haptic service and register on the bus*/
func Init(log *log.Logger) error {

	logger = log

	if conn, err = dbus.Connect(dbus.SessionBus); err != nil {
		logger.Fatal("Connection error:", err)
		return err
	}

	nameAcquired := make(chan int, 1)
	name := conn.RequestName("com.canonical.usensord.haptic", dbus.NameFlagDoNotQueue, func(*dbus.BusName) { nameAcquired <- 0 }, nil)
	<-nameAcquired

	logger.Printf("Successfully registerd %s on the bus.\n", name)

	ch := make(chan *dbus.Message)
	go watchDBusMethodCalls(ch)
	conn.RegisterObjectPath("/com/canonical/usensord/haptic", ch)

	logger.Println("Connected to DBUS")

	return nil

}

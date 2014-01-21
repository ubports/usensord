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
	"time"
)

var	(
	conn   *dbus.Connection
	logger *log.Logger
)

const (
	HAPTIC_DBUS_IFACE = "com.canonical.usensord.haptic"
	HAPTIC_DEVICE     = "/sys/class/timed_output/vibrator/enable"
)

func watchDBusMethodCalls(msgChan <-chan *dbus.Message) {
	var reply *dbus.Message

	for msg := range msgChan {
		switch {
		case msg.Interface == HAPTIC_DBUS_IFACE && msg.Member == "Vibrate":
			var duration uint32
			msg.Args(&duration)
			logger.Printf("Received Vibrate() method call %d", duration)
			if err := Vibrate(duration); err != nil {
				reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
			} else {
				reply = dbus.NewMethodReturnMessage(msg)
			}
		case msg.Interface == HAPTIC_DBUS_IFACE && msg.Member == "VibratePattern":
			var pattern []uint32
			msg.Args(&pattern)
			logger.Print("Received VibratePattern() method call", pattern)
			if err := VibratePattern(pattern); err != nil {
				reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
			} else {
				reply = dbus.NewMethodReturnMessage(msg)
			}
		default:
			logger.Println("Received unkown method call on", msg.Interface,	msg.Member)
			reply = dbus.NewErrorMessage(msg, "org.freedesktop.DBus.Error.UnknownMethod", "Unknown method")
		}
		if err := conn.Send(reply); err != nil {
			logger.Println("Could not send reply:", err)
		}
	}
}

func Vibrate(duration uint32) error {
	return VibratePattern([]uint32{duration})
}

func VibratePattern(duration []uint32) (err error) {

	fi, err := os.Create(HAPTIC_DEVICE)
	if err != nil {
		logger.Println("Error opening haptic device")
		return err
	}
	x := true

	go func() {
		defer fi.Close()
		for _, t := range duration {
			if x {
				if _, err := fi.WriteString(fmt.Sprintf("%d", t)); err != nil {
					logger.Println(err)
				}
				x = false
			} else {
				x = true
			}
			time.Sleep(time.Duration(t) * time.Millisecond)
		}
	}()
	return nil
}

/*Initialize Haptic service and register on the bus*/
func Init(log *log.Logger) (err error) {

	logger = log
	if conn, err = dbus.Connect(dbus.SessionBus); err != nil {
		logger.Fatal("Connection error:", err)
		return err
	}

	nameAcquired := make(chan int, 1)
	name := conn.RequestName("com.canonical.usensord.haptic", dbus.NameFlagDoNotQueue, func(*dbus.BusName) { nameAcquired <- 0 }, nil)
	<-nameAcquired

	logger.Printf("Successfully registerd %s on the bus", name)

	ch := make(chan *dbus.Message)
	go watchDBusMethodCalls(ch)
	conn.RegisterObjectPath("/com/canonical/usensord/haptic", ch)

	logger.Println("Connected to DBUS")

	return nil
}

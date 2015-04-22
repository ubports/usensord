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
	"log"
	"os"
	"sync"
	"time"

	"launchpad.net/go-dbus/v1"
)

var (
	conn   *dbus.Connection
	sysbus *dbus.Connection
	logger *log.Logger
	wg     sync.WaitGroup
	powerd *dbus.ObjectProxy
	mutex  *sync.Mutex
	cookie string
	timer  *time.Timer
)

const (
	HAPTIC_DBUS_IFACE = "com.canonical.usensord.haptic"
	HAPTIC_DEVICE     = "/sys/class/timed_output/vibrator/enable"
)

func watchDBusMethodCalls(msgChan <-chan *dbus.Message) {
	for msg := range msgChan {
		var reply *dbus.Message

		if msg.Interface == HAPTIC_DBUS_IFACE {
			reply = handleHapticInterface(msg)
		} else {
			reply = dbus.NewErrorMessage(
				msg,
				"org.freedesktop.DBus.Error.UnknownInterface",
				fmt.Sprintf("No such interface '%s' at object path '%s'", msg.Interface, msg.Path))
		}

		if err := conn.Send(reply); err != nil {
			logger.Println("Could not send reply:", err)
		}
	}
}

func handleHapticInterface(msg *dbus.Message) (reply *dbus.Message) {
	switch msg.Member {
	case "Vibrate":
		var duration uint32
		msg.Args(&duration)
		logger.Printf("Received Vibrate() method call %d", duration)
		if err := Vibrate(duration); err != nil {
			reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
		} else {
			reply = dbus.NewMethodReturnMessage(msg)
		}
	case "VibratePattern":
		var pattern []uint32
		var repeat uint32
		msg.Args(&pattern, &repeat)
		logger.Print("Received VibratePattern() method call ", pattern, " ", repeat)
		if err := VibratePattern(pattern, repeat); err != nil {
			reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
		} else {
			reply = dbus.NewMethodReturnMessage(msg)
		}
	default:
		logger.Println("Received unkown method call on", msg.Interface, msg.Member)
		reply = dbus.NewErrorMessage(msg, "org.freedesktop.DBus.Error.UnknownMethod", "Unknown method")
	}
	return reply
}

// Vibrate generates a vibration with the specified duration
// If the haptic device used to generate the vibration cannot be opened
// an error is returned in err.
func Vibrate(duration uint32) error {
	return VibratePattern([]uint32{duration}, 1)
}

// VibratePattern generates a vibration in the form of a Pattern set in
// duration a pattern of on off states, repeat specifies the ammount of times
// the pattern should be repeated.
// A repeat value of 0 is a nop, not an error.
// If the haptic device used to generate the vibration cannot be opened
// an error is returned in err.
func VibratePattern(duration []uint32, repeat uint32) (err error) {

	fi, err := os.Create(HAPTIC_DEVICE)
	if err != nil {
		logger.Println("Error opening haptic device")
		return err
	}

	wg.Add(1)
	go func() {
		defer fi.Close()
		defer wg.Done()
		for n := uint32(0); n < repeat; n++ {
			x := true
			for _, t := range duration {
				mutex.Lock();
				if x {
					if (cookie == "") {
						reply, err := powerd.Call("com.canonical.powerd", "requestSysState", "usensord", int32(1))
						if err != nil {
							logger.Println("Cannot request Powerd system power state: ", err)
						} else {
							if err := reply.Args(&cookie); err == nil {
								logger.Println("Suspend blocker cookie: ", cookie)
								timer = time.NewTimer(1500 * time.Millisecond)
								go func() {
									<-timer.C
									logger.Println("Clearing suspend blocker")
									if cookie != "" {
										powerd.Call("com.canonical.powerd", "clearSysState", string(cookie))
										cookie = ""
									}
								}()
							}
						}
					} else {
						timer.Reset(1500 * time.Millisecond)
					}
					if _, err := fi.WriteString(fmt.Sprintf("%d", t)); err != nil {
						logger.Println(err)
					}
					x = false
				} else {
					x = true
				}
				mutex.Unlock()
				time.Sleep(time.Duration(t) * time.Millisecond)
			}
		}
	}()

	return nil
}

// Init exposes the haptic device object path on the bus.
func Init(log *log.Logger) (err error) {

	logger = log
	if conn, err = dbus.Connect(dbus.SessionBus); err != nil {
		logger.Fatal("Connection error:", err)
		return err
	}

	if sysbus, err = dbus.Connect(dbus.SystemBus); err != nil {
		logger.Fatal("Connection error:", err)
		return err
	}

	name := conn.RequestName("com.canonical.usensord", dbus.NameFlagDoNotQueue)
	logger.Printf("Successfully registered %s on the bus", name.Name)

	powerd = sysbus.Object("com.canonical.powerd", "/com/canonical/powerd")
	mutex = &sync.Mutex{}

	ch := make(chan *dbus.Message)
	go watchDBusMethodCalls(ch)
	conn.RegisterObjectPath("/com/canonical/usensord/haptic", ch)

	logger.Println("Connected to DBUS")

	return nil
}

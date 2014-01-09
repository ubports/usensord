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
	"launchpad.net/usensord/dbus"
	"log"
	"os"
	"testing"
)

func init() {
	logger = log.New(os.Stderr, "uSensord: ", log.Ldate|log.Ltime|log.Lshortfile)
	var err error
	conn, err = dbus.Connect(dbus.SessionBus)
	if err != nil {
		logger.Fatal("Connection error:", err)
	}

	err = Init(logger)
	if err != nil {
		logger.Fatal("Error: %s\n", err)
	}
}

func TestHapticDBUS(t *testing.T) {
	obj := conn.Object("com.canonical.usensord.haptic", "/com/canonical/usensord/haptic")

	reply, err := obj.Call("com.canonical.usensord.haptic", "Vibrate", uint32(10))
	if err != nil || reply.Type == dbus.TypeError {
		logger.Println("FAILED")
		t.Errorf("Notification error: %s", err)
	}
}

func TestPatternHapticDBUS(t *testing.T) {
	pattern := []uint32{uint32(10), uint32(100), uint32(200), uint32(10)}

	obj := conn.Object("com.canonical.usensord.haptic", "/com/canonical/usensord/haptic")

	reply, err := obj.Call("com.canonical.usensord.haptic", "VibratePattern", pattern)
	if err != nil || reply.Type == dbus.TypeError {
		logger.Println("FAILED")
		t.Errorf("Notification error: %s", err)
	}
}

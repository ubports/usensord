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
	"launchpad.net/~jamesh/go-dbus/trunk"
	"log"
	"os"
	"testing"
)


func TestHapticDBUS(t *testing.T) {

	logger = log.New(os.Stderr, "uSensord: ", log.Ldate | log.Ltime | log.Lshortfile)
	var conn *dbus.Connection

	if conn, err = dbus.Connect(dbus.SessionBus); err != nil {
		t.Errorf("Connection error:", err)
	}

	err = Init(logger)

	if err != nil {
		t.Errorf("Error: %s\n", err)
	}


	obj := conn.Object("com.canonical.usensord.haptic", "/com/canonical/usensord/haptic")
	
	reply, err := obj.Call("com.canonical.usensord.haptic", "On", uint32(10))

	if err != nil || reply == nil {
		t.Errorf("Notification error: %s", err)
	}

	reply, err = obj.Call("com.canonical.usensord.haptic", "Off")

	if err != nil || reply == nil {
		t.Errorf("Notification error: %s", err)
	}
	
}

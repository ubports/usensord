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
        "encoding/json"
	"fmt"
	"log"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"launchpad.net/go-dbus/v1"
)
// #cgo CFLAGS: -I/usr/include
// #cgo linux,ppc LDFLAGS: -L/usr/lib/powerpc-linux-gnu -lapparmor
// #cgo linux,ppc64le LDFLAGS: -L/usr/lib/powerpc64le-linux-gnu -lapparmor
// #cgo linux,s390x LDFLAGS: -L/usr/lib/s390x-linux-gnu -lapparmor
// #cgo linux,386 LDFLAGS: -L/usr/lib/i386-linux-gnu -lapparmor
// #cgo linux,amd64 LDFLAGS: -L/usr/lib/x86_64-linux-gnu -lapparmor
// #cgo linux,arm LDFLAGS: -L/usr/lib/arm-linux-gnueabihf -lapparmor
// #cgo linux,arm64 LDFLAGS: -L/usr/lib/aarch64-linux-gnu -lapparmor
//#include <sys/apparmor.h>
//#include <errno.h>
import "C"

type Prop struct {
    OtherVibrate uint32
}

var (
        conn       *dbus.Connection
        sesconn    *dbus.Connection
        sysbus     *dbus.Connection
        messageBus *dbus.ObjectProxy
        logger     *log.Logger
        wg         sync.WaitGroup
        powerd     *dbus.ObjectProxy
        mutex      *sync.Mutex
        cookie     string
        timer      *time.Timer
        pvalue     uint32
        configFile string
        vibrateScale uint32
)

const (
	HAPTIC_DBUS_IFACE = "com.canonical.usensord.haptic"
	HAPTIC_DEVICE     = "/sys/class/timed_output/vibrator/enable"
        PROP_DBUS_IFACE = "org.freedesktop.DBus.Properties"
        OSK_PROCESS_NAME = "/usr/bin/maliit-server"
        TSA_PROCESS_NAME = "/usr/bin/telephony-service-approver"
        TSI_PROCESS_NAME = "/usr/bin/telephony-service-indicator"
        UNCONFINED_PROFILE = "unconfined"
)

func watchDBusMethodCalls(msgChan <-chan *dbus.Message) {
	for msg := range msgChan {
		var reply *dbus.Message

		if msg.Interface == HAPTIC_DBUS_IFACE {
			reply = handleHapticInterface(msg)
		} else if msg.Interface == PROP_DBUS_IFACE {
			reply = handlePropInterface(msg)
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

func handlePropInterface(msg *dbus.Message) (reply *dbus.Message) {
        switch msg.Member {
        case "Get":
                var iname, pname string
                msg.Args(&iname, &pname)
                if iname == HAPTIC_DBUS_IFACE && pname == "OtherVibrate" {
                        reply = dbus.NewMethodReturnMessage(msg)
                        reply.AppendArgs(dbus.Variant{uint32(pvalue)})
                } else {
                        reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", "interface or property not correct")
                }
        case "GetAll":
                var iname string
                msg.Args(&iname)
                if iname == HAPTIC_DBUS_IFACE {
                        reply = dbus.NewMethodReturnMessage(msg)                        
                        reply.AppendArgs(dbus.Variant{uint32(pvalue)})
                } else {
                        reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", "interface or property not correct")
                }
        case "Set":
                var iname, pname string
                msg.Args(&iname, &pname, &pvalue)
                if iname == HAPTIC_DBUS_IFACE && pname == "OtherVibrate" && (pvalue == 1 || pvalue == 0) {
                        //save the property value
                        prop := Prop{OtherVibrate: pvalue,}
                        propJson, _ := json.Marshal(prop)
                        errwrite := ioutil.WriteFile(configFile, propJson, 0644)
                        if errwrite != nil {
                            logger.Println("WriteFile error:", errwrite)
                        }

                        reply = dbus.NewMethodReturnMessage(msg)
                        reply.AppendArgs(dbus.Variant{uint32(pvalue)})
                } else {
                        reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", "interface or property not correct")
                }
        default:
                logger.Println("Received unknown method call on", msg.Interface, msg.Member)
                reply = dbus.NewErrorMessage(msg, "org.freedesktop.DBus.Error.UnknownMethod", "Unknown method")
        }
        return reply
}

func handleHapticInterface(msg *dbus.Message) (reply *dbus.Message) {
        processreply, err := messageBus.Call("org.freedesktop.DBus", "GetConnectionCredentials", msg.Sender)
        if err != nil {
                reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
                return reply
        }
        var credentials map[string]dbus.Variant
        if err := processreply.Args(&credentials); err != nil {
                reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
                return reply
        }
        pid := credentials["ProcessID"].Value.(uint32)
        var profile string
        ret, error := C.aa_is_enabled()
        if ret == 1 {
                 label := credentials["LinuxSecurityLabel"].Value.([]interface{})
                 var bb []uint8
                 for _, f := range label {
                         bb = append(bb, f.(uint8))
                 }
                 profile = strings.TrimSpace(string(bb))
                 //LinuxSecurityLabel ends with null
                 profile = profile[:len(profile)-1]
        } else {
                logger.Println("aa_is_enabled failed:", error)
                profile = UNCONFINED_PROFILE
        }
        isPrivileged := false
        if profile == UNCONFINED_PROFILE {
                file := "/proc/" + strconv.FormatUint(uint64(pid), 10) + "/exe"
                _, err := os.Lstat(file)
                if err != nil {
                        logger.Println("error while calling os.Lstat", err)
                }
                exe, erreadexe := os.Readlink(file)
                if erreadexe != nil {
                        logger.Printf("fail to read %s with error:", file, erreadexe.Error())
                } else {
                        pname := strings.TrimSpace(string(exe))
                        if pname == OSK_PROCESS_NAME || pname == TSA_PROCESS_NAME || pname == TSI_PROCESS_NAME {
                                isPrivileged = true
                        }
                }
        }
        if !isPrivileged && pvalue == 0 {
                reply = dbus.NewMethodReturnMessage(msg)
                return reply
        }
	switch msg.Member {
	case "Vibrate":
		var duration uint32
		msg.Args(&duration)
		if err := Vibrate(duration + vibrateScale); err != nil {
			reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
		} else {
			reply = dbus.NewMethodReturnMessage(msg)
		}
	case "VibratePattern":
		var pattern []uint32
		var repeat uint32
		msg.Args(&pattern, &repeat)
		if err := VibratePattern(pattern, repeat); err != nil {
			reply = dbus.NewErrorMessage(msg, "com.canonical.usensord.Error", err.Error())
		} else {
			reply = dbus.NewMethodReturnMessage(msg)
		}
	default:
                logger.Println("Received unknown method call on", msg.Interface, msg.Member)
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
				if x {
					mutex.Lock();
					if (cookie == "") {
						reply, err := powerd.Call("com.canonical.powerd", "requestSysState", "usensord", int32(1))
						if err != nil {
							logger.Println("Cannot request Powerd system power state: ", err)
						} else {
							if err := reply.Args(&cookie); err == nil {
								timer = time.NewTimer(time.Duration(t + 1500) * time.Millisecond)
								go func() {
									<-timer.C
									if cookie != "" {
										powerd.Call("com.canonical.powerd", "clearSysState", string(cookie))
										cookie = ""
									}
								}()
							}
						}
					} else {
						timer.Reset(time.Duration(t + 1500) * time.Millisecond)
					}
					mutex.Unlock()
					if _, err := fi.WriteString(fmt.Sprintf("%d", t)); err != nil {
						logger.Println(err)
					}
					x = false
				} else {
					x = true
				}
				time.Sleep(time.Duration(t) * time.Millisecond)
			}
		}
	}()

	return nil
}

// Init exposes the haptic device object path on the bus.
func Init(log *log.Logger, scale uint32) (err error) {

	logger = log
	vibrateScale = scale
	if conn, err = dbus.Connect(dbus.SessionBus); err != nil {
		logger.Fatal("Connection error:", err)
		return err
	}

	if sysbus, err = dbus.Connect(dbus.SystemBus); err != nil {
		logger.Fatal("Connection error:", err)
		return err
	}
        if sesconn, err = dbus.Connect(dbus.SessionBus); err != nil {
                logger.Fatal("Connection error:", err)
                return err
        }
	name := conn.RequestName("com.canonical.usensord", dbus.NameFlagDoNotQueue)
        logger.Printf("Successfully registered %s on the bus", name.Name)

	powerd = sysbus.Object("com.canonical.powerd", "/com/canonical/powerd")
        messageBus = sesconn.Object("org.freedesktop.DBus", "/org/freedesktop/DBus")
	mutex = &sync.Mutex{}

        //save and load the property value
        u, err := user.Current()
        configPath := path.Join(u.HomeDir, ".config", "usensord")
        configFile = path.Join(u.HomeDir, ".config", "usensord", "prop.json")
        os.MkdirAll(configPath, 0755)
        b, errread := ioutil.ReadFile(configFile)
        if errread != nil {
                pvalue = 1
                prop := Prop{OtherVibrate: pvalue,}
                propJson, _ := json.Marshal(prop)
                errwrite := ioutil.WriteFile(configFile, propJson, 0644)
                if errwrite != nil {
                    logger.Fatal("WriteFile error:", errwrite)
                    return errwrite
                }
        } else {
                var prop Prop
                err := json.Unmarshal(b, &prop)
                if err == nil {
                        pvalue = prop.OtherVibrate
                        log.Println("pvalueb is", pvalue)
                } else {
                        log.Println("err is", err)
                        pvalue = 0
                }
        }

	ch := make(chan *dbus.Message)
	go watchDBusMethodCalls(ch)
	conn.RegisterObjectPath("/com/canonical/usensord/haptic", ch)

	logger.Println("Connected to DBUS")

	return nil
}

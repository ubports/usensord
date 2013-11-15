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

package main

import (
	"haptic"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var logger *log.Logger
var done = false


type Mainloop struct {
	sigchan  chan os.Signal
	termchan chan int
	Bindings map[os.Signal]func()
}




/*
Start the mainloop.

This method will block its current thread. The best spot for calling this
method is right near the bottom of your application's main() function.
*/
func (m *Mainloop) Start() {
	sigs := make([]os.Signal, len(m.Bindings))
	for s, _ := range m.Bindings {
		sigs = append(sigs, s)
	}
	signal.Notify(m.sigchan, sigs...)
	for {
		select {
		case sig := <-m.sigchan:
			m.Bindings[sig]()
		case _ = <-m.termchan:
			break
		}
	}
	return
}

/*
Stops the mainloop.
*/
func (m *Mainloop) Stop() {
	go func() { m.termchan <- 1 }()
	return
}


func HupHandler() {

	syscall.Exit(1)

}

func IntHandler() {

	syscall.Exit(1)
}


func init() {
	logger = log.New(os.Stderr, "uSensord: ", log.Ldate | log.Ltime | log.Lshortfile)
}


func main() {

	err := haptic.Init(logger)
	if err != nil {
		logger.Println("Error starting haptic service")
	}

	logger.Println("uSensord starting...")

	m := Mainloop{
		sigchan:  make(chan os.Signal),
		termchan: make(chan int),
		Bindings: make(map[os.Signal]func())}



	m.Bindings[syscall.SIGHUP] = HupHandler
	m.Bindings[syscall.SIGINT] = IntHandler
	m.Start()

}

/*
This is an example of application that will use the
engine package to test things out
*/
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spaghettifunk/anima/engine"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/testbed"
)

func main() {
	tb, err := testbed.NewTestGame()
	if err != nil {
		panic(err.Error())
	}

	engine, err := engine.New(tb.Game)
	if err != nil {
		panic(err.Error())
	}

	if err := engine.Initialize(); err != nil {
		panic(err.Error())
	}

	// signal channel to capture system calls
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// start shutdown goroutine
	go func() {
		// capture sigterm and other system call here
		<-sigCh
		if err := engine.Shutdown(); err != nil {
			panic(err.Error())
		}
		core.LogInfo("engine shutdown. Bye Bye")
	}()

	// run engine
	if err := engine.Run(); err != nil {
		panic(err.Error())
	}
}

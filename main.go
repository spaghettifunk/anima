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
	done := make(chan bool, 1)

	tb := testbed.NewTestGame()

	engine, err := engine.New(tb.Game)
	if err != nil {
		core.LogError(err.Error())
		close(done)
	}

	if err := engine.Initialize(); err != nil {
		core.LogError(err.Error())
		close(done)
	}

	// signal channel to capture system calls
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// start shutdown goroutine
	go func() {
		// capture sigterm and other system call here
		<-sigCh
		if err := engine.Shutdown(); err != nil {
			core.LogError(err.Error())
			return
		}
		core.LogInfo("engine shutdown. Bye Bye")
		close(done)
	}()

	// run engine
	if err := engine.Run(); err != nil {
		core.LogError(err.Error())
		close(done)
	}

	// time to say goodbye
	<-done
}

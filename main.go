/*
This is an example of application that will use the
engine package to test things out
*/
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/spaghettifunk/alaska-engine/engine"
	"github.com/spaghettifunk/alaska-engine/testbed"
)

func main() {
	tb := testbed.NewTestGame()

	engine, err := engine.New(tb)
	if err != nil {
		panic(err)
	}

	if err := engine.Initialize(); err != nil {
		panic(err)
	}

	// signal channel to capture system calls
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// start shutdown goroutine
	go func() {
		// capture sigterm and other system call here
		<-sigCh
		_ = engine.Shutdown()
	}()

	// run engine
	if err := engine.Run(); err != nil {
		panic(err)
	}
}

//go:build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/mg"
)

type Run mg.Namespace

// Runs go mod download and then installs the binary.
func (Run) Engine() error {
	if err := buildShaders(); err != nil {
		return err
	}
	fmt.Println("Run engine...")
	if _, err := executeCmd("go", withArgs("run", "main.go"), withStream()); err != nil {
		return err
	}
	return nil
}

//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
)

type Build mg.Namespace

// Runs go mod download and then installs the binary.
func (Build) Shaders() error {
	if _, err := executeCmd("glslc", withArgs("shaders/shader.vert", "-o", "shaders/vert.spv"), withStream()); err != nil {
		return err
	}
	if _, err := executeCmd("glslc", withArgs("shaders/shader.frag", "-o", "shaders/frag.spv"), withStream()); err != nil {
		return err
	}
	return nil
}

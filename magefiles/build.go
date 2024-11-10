//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/mg"
)

type Build mg.Namespace

func buildShaders() error {
	fmt.Println("Build shaders...")
	vkSDKPath := os.Getenv("VULKAN_SDK")
	if _, err := executeCmd(fmt.Sprintf("%s/bin/glslc", vkSDKPath), withArgs("-fshader-stage=vert", "assets/shaders/Builtin.MaterialShader.vert.glsl", "-o", "assets/shaders/Builtin.MaterialShader.vert.spv"), withStream()); err != nil {
		return err
	}
	if _, err := executeCmd(fmt.Sprintf("%s/bin/glslc", vkSDKPath), withArgs("-fshader-stage=frag", "assets/shaders/Builtin.MaterialShader.frag.glsl", "-o", "assets/shaders/Builtin.MaterialShader.frag.spv"), withStream()); err != nil {
		return err
	}
	if _, err := executeCmd(fmt.Sprintf("%s/bin/glslc", vkSDKPath), withArgs("-fshader-stage=vert", "assets/shaders/Builtin.SkyboxShader.vert.glsl", "-o", "assets/shaders/Builtin.SkyboxShader.vert.spv"), withStream()); err != nil {
		return err
	}
	if _, err := executeCmd(fmt.Sprintf("%s/bin/glslc", vkSDKPath), withArgs("-fshader-stage=frag", "assets/shaders/Builtin.SkyboxShader.frag.glsl", "-o", "assets/shaders/Builtin.SkyboxShader.frag.spv"), withStream()); err != nil {
		return err
	}
	if _, err := executeCmd(fmt.Sprintf("%s/bin/glslc", vkSDKPath), withArgs("-fshader-stage=vert", "assets/shaders/Builtin.UIShader.vert.glsl", "-o", "assets/shaders/Builtin.UIShader.vert.spv"), withStream()); err != nil {
		return err
	}
	if _, err := executeCmd(fmt.Sprintf("%s/bin/glslc", vkSDKPath), withArgs("-fshader-stage=frag", "assets/shaders/Builtin.UIShader.frag.glsl", "-o", "assets/shaders/Builtin.UIShader.frag.spv"), withStream()); err != nil {
		return err
	}
	return nil
}

// Runs go mod download and then installs the binary.
func (Build) Shaders() error {
	return buildShaders()
}

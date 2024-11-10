package loaders

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/pelletier/go-toml/v2"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type ShaderLoader struct{}

type tmpShaderConfig struct {
	Version     string      `toml:"version"`
	Name        string      `toml:"name"`
	CullMode    string      `toml:"cull_mode"`
	Renderpass  string      `toml:"renderpass"`
	Stages      []string    `toml:"stages"`
	StageFiles  []string    `toml:"stagefiles"`
	UseInstance bool        `toml:"use_instance"`
	UseLocal    bool        `toml:"use_local"`
	Attributes  []attribute `toml:"attribute"`
	Uniforms    []uniform   `toml:"uniform"`
}

// attribute represents a single attribute entry
type attribute struct {
	Type string `toml:"type"`
	Name string `toml:"name"`
}

// uniform represents a single uniform entry
type uniform struct {
	Type  string `toml:"type"`
	Scope int    `toml:"scope"`
	Name  string `toml:"name"`
}

// Validate checks for duplicate names in Attributes and Uniforms
func (config *tmpShaderConfig) Validate() error {
	attrNames := make(map[string]bool)
	for _, attr := range config.Attributes {
		if attrNames[attr.Name] {
			return fmt.Errorf("duplicate attribute name found: %s", attr.Name)
		}
		attrNames[attr.Name] = true
	}
	uniformNames := make(map[string]bool)
	for _, uniform := range config.Uniforms {
		if uniformNames[uniform.Name] {
			return fmt.Errorf("duplicate uniform name found: %s", uniform.Name)
		}
		uniformNames[uniform.Name] = true
	}
	return nil
}

func (config *tmpShaderConfig) TransformToShaderConfig() (*metadata.ShaderConfig, error) {
	shaderCfg := &metadata.ShaderConfig{
		Name:           config.Name,
		RenderpassName: config.Renderpass,
		StageNames:     config.StageFiles,
		StageFilenames: config.StageFiles,
	}

	stages := make([]metadata.ShaderStage, len(config.Stages))
	for i, st := range config.Stages {
		s, err := metadata.ShaderStageFromString(st)
		if err != nil {
			return nil, err
		}
		stages[i] = s
	}
	shaderCfg.Stages = stages

	attributes := make([]*metadata.ShaderAttributeConfig, len(config.Attributes))
	for i, att := range config.Attributes {
		t, size, err := metadata.ShaderAttributeTypeFromString(att.Type)
		if err != nil {
			return nil, err
		}
		attributes[i] = &metadata.ShaderAttributeConfig{
			Name:                att.Name,
			ShaderAttributeType: t,
			Size:                size,
		}
	}
	shaderCfg.Attributes = attributes

	uniforms := make([]*metadata.ShaderUniformConfig, len(config.Uniforms))
	for i, unif := range config.Uniforms {
		t, size, err := metadata.ShaderUniformTypeFromString(unif.Type)
		if err != nil {
			return nil, err
		}
		uniforms[i] = &metadata.ShaderUniformConfig{
			Name:              unif.Name,
			Size:              size,
			ShaderUniformType: t,
			Location:          0,
			Scope:             metadata.ShaderScope(unif.Scope),
		}
	}
	shaderCfg.Uniforms = uniforms

	if config.CullMode != "" {
		cm, err := metadata.CullModeFromString(config.CullMode)
		if err != nil {
			return nil, err
		}
		shaderCfg.CullMode = cm
	}

	return shaderCfg, nil
}

func (sl *ShaderLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	tmpShaderConfig := tmpShaderConfig{}
	cfg, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = toml.Unmarshal([]byte(cfg), &tmpShaderConfig)
	if err != nil {
		return nil, err
	}

	if err := tmpShaderConfig.Validate(); err != nil {
		return nil, err
	}

	shaderCfg, err := tmpShaderConfig.TransformToShaderConfig()
	if err != nil {
		return nil, err
	}

	return &metadata.Resource{
		Name:     shaderCfg.Name,
		FullPath: path,
		DataSize: uint64(unsafe.Sizeof(shaderCfg)),
		Data:     shaderCfg,
	}, nil
}

func (sl *ShaderLoader) Unload(resource *metadata.Resource) error {
	return nil
}

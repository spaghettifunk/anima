package loaders

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/math"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type MaterialLoader struct{}

func (ml *MaterialLoader) Load(path string, assetType metadata.ResourceType, params interface{}) (*metadata.Resource, error) {
	mCfg, err := parseAMTFile(path)
	if err != nil {
		return nil, err
	}
	return &metadata.Resource{
		Name:     "material",
		FullPath: path,
		DataSize: uint64(unsafe.Sizeof(metadata.MaterialConfig{})),
		Data:     mCfg,
	}, nil
}

func parseAMTFile(filename string) (*metadata.MaterialConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	materialConfig := &metadata.MaterialConfig{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Split key-value pairs by the first "=" sign
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Skipping invalid line: %s\n", line)
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Parse each field based on the key
		switch key {
		case "name":
			materialConfig.Name = value
		case "shader":
			materialConfig.ShaderName = value
		case "diffuse_colour":
			colourValues := strings.Fields(value)
			if len(colourValues) != 4 {
				err := fmt.Errorf("invalid diffuse_colour, expected 4 values: %s", line)
				return nil, err
			}
			for i, v := range colourValues {
				f, err := strconv.ParseFloat(v, 32)
				if err != nil {
					err := fmt.Errorf("invalid diffuse_colour value: %s", v)
					return nil, err
				}
				switch i {
				case 0:
					materialConfig.DiffuseColour.X = float32(f)
				case 1:
					materialConfig.DiffuseColour.Y = float32(f)
				case 2:
					materialConfig.DiffuseColour.Z = float32(f)
				case 3:
					materialConfig.DiffuseColour.W = float32(f)
				}
			}
		case "shininess":
			shininess, err := strconv.ParseFloat(value, 32)
			if err != nil {
				err := fmt.Errorf("invalid shininess value: %s", value)
				return nil, err
			}
			materialConfig.Shininess = float32(shininess)
		case "diffuse_map_name":
			materialConfig.DiffuseMapName = value
		case "specular_map_name":
			materialConfig.SpecularMapName = value
		case "normal_map_name":
			materialConfig.NormalMapName = value
		case "autorelease":
			autoRelease, err := strconv.ParseBool(value)
			if err != nil {
				err := fmt.Errorf("invalid autorelease value: %s", value)
				return nil, err
			}
			materialConfig.AutoRelease = autoRelease
		default:
			core.LogError("Unknown key '%s' found in file. Skipping...", key)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	// Perform validation
	if err := validateMaterial(materialConfig); err != nil {
		return nil, err
	}
	return materialConfig, nil
}

func validateMaterial(material *metadata.MaterialConfig) error {
	if material.Name == "" {
		return fmt.Errorf("material name is required")
	}

	if material.ShaderName == "" {
		return fmt.Errorf("shader name is required")
	}

	// Check that DiffuseColour values are within [0.0, 1.0] range
	if !isValidVec4(material.DiffuseColour) {
		return fmt.Errorf("diffuse_colour values must be between 0.0 and 1.0")
	}

	// Check shininess for a non-negative value
	if material.Shininess < 0 {
		return fmt.Errorf("shininess must be a non-negative value")
	}

	// Check texture map names if they are present
	if material.DiffuseMapName != "" && !isValidTextureName(material.DiffuseMapName) {
		return fmt.Errorf("invalid diffuse map name: %s", material.DiffuseMapName)
	}

	if material.SpecularMapName != "" && !isValidTextureName(material.SpecularMapName) {
		return fmt.Errorf("invalid specular map name: %s", material.SpecularMapName)
	}

	if material.NormalMapName != "" && !isValidTextureName(material.NormalMapName) {
		return fmt.Errorf("invalid normal map name: %s", material.NormalMapName)
	}

	return nil
}

// Helper function to validate Vec4 fields (must be between 0.0 and 1.0)
func isValidVec4(v math.Vec4) bool {
	return inRange(v.X) && inRange(v.Y) && inRange(v.Z) && inRange(v.W)
}

// Check if a float32 value is within [0.0, 1.0]
func inRange(value float32) bool {
	return value >= 0.0 && value <= 1.0
}

// Mock validation function for texture names
func isValidTextureName(name string) bool {
	// Add custom logic to validate texture map names
	return len(name) > 0
}

func (ml *MaterialLoader) Unload(*metadata.Resource) error {
	return nil
}

package systems

import (
	"fmt"

	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
)

type MeshLoaderSystem struct {
	geometrySystem *GeometrySystem
	resourceSystem *ResourceSystem
}

func NewMeshLoaderSystem(gs *GeometrySystem, rs *ResourceSystem) (*MeshLoaderSystem, error) {
	return &MeshLoaderSystem{
		geometrySystem: gs,
		resourceSystem: rs,
	}, nil
}

func (mls *MeshLoaderSystem) Shutdown() error {
	return nil
}

func (mls *MeshLoaderSystem) LoadFromResource(resourceName string, mesh *metadata.Mesh) bool {
	return false
}

func (mls *MeshLoaderSystem) Unload(mesh *metadata.Mesh) {

}

/**
 * @brief Called when the job completes successfully.
 *
 * @param params The parameters passed from the job after completion.
 */
func (mls *MeshLoaderSystem) meshLoadJobSuccess(params interface{}) error {
	mesh_params, ok := params.(metadata.MeshLoadParams)
	if !ok {
		err := fmt.Errorf("failed to cast params to metadata.MeshLoadParams")
		core.LogError(err.Error())
		return err
	}

	// This also handles the GPU upload. Can't be jobified until the renderer is multithreaded.
	configs := mesh_params.MeshResource.Data.([]*metadata.GeometryConfig)
	mesh_params.OutMesh.GeometryCount = uint16(mesh_params.MeshResource.DataSize)

	mesh_params.OutMesh.Geometries = make([]*metadata.Geometry, mesh_params.OutMesh.GeometryCount)

	for i := uint16(0); i < mesh_params.OutMesh.GeometryCount; i++ {
		g, err := mls.geometrySystem.AcquireFromConfig(configs[i], true)
		if err != nil {
			core.LogError(err.Error())
			return err
		}
		mesh_params.OutMesh.Geometries[i] = g
	}
	mesh_params.OutMesh.Generation++

	core.LogDebug("Successfully loaded mesh '%s'.", mesh_params.ResourceName)

	return mls.resourceSystem.Unload(mesh_params.MeshResource)
}

/**
 * @brief Called when the job fails.
 *
 * @param params Parameters passed when a job fails.
 */
func (mls *MeshLoaderSystem) meshLoadJobFail(params interface{}) {
	meshParams := params.(metadata.MeshLoadParams)
	core.LogError("Failed to load mesh '%s'.", meshParams.ResourceName)
	if err := mls.resourceSystem.Unload(meshParams.MeshResource); err != nil {
		core.LogError(err.Error())
	}
}

/**
 * @brief Called when a mesh loading job begins.
 *
 * @param params Mesh loading parameters.
 * @param result_data Result data passed to the completion callback.
 * @return True on job success; otherwise false.
 */
func (mls *MeshLoaderSystem) meshLoadJobStart(params interface{}) (*metadata.Resource, error) {
	load_params, ok := params.(*metadata.MeshLoadParams)
	if !ok {
		err := fmt.Errorf("failed to cast params to `*metadata.MeshLoadParams`")
		core.LogError(err.Error())
		return nil, err
	}
	mesh, err := mls.resourceSystem.Load(load_params.ResourceName, metadata.ResourceTypeMesh, 0)
	if err != nil {
		core.LogError(err.Error())
		return nil, err
	}
	return mesh, nil
}

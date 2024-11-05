package systems

import (
	"github.com/spaghettifunk/anima/engine/assets/loaders"
	"github.com/spaghettifunk/anima/engine/core"
	"github.com/spaghettifunk/anima/engine/renderer/metadata"
	"github.com/spaghettifunk/anima/engine/renderer/views"
)

/** @brief The configuration for the render view system. */
type RenderViewSystemConfig struct {
	/** @brief The maximum number of views that can be registered with the system. */
	MaxViewCount uint16
}

type RenderViewSystem struct {
	Lookup          map[string]uint16
	MaxViewCount    uint32
	RegisteredViews []*metadata.RenderView
	// subsystems
	renderer *RendererSystem
}

/**
 * @brief Initializes the render view system. Call twice; once to obtain memory
 * requirement (where state=0) and a second time with allocated memory passed to state.
 *
 * @param memory_requirement A pointer to hold the memory requirement in bytes.
 * @param state A block of memory to be used for the state.
 * @param config Configuration for the system.
 * @return True on success; otherwise false.
 */
func NewRenderViewSystem(config RenderViewSystemConfig, r *RendererSystem) (*RenderViewSystem, error) {
	if config.MaxViewCount == 0 {
		core.LogError("render_view_system_initialize - config.MaxViewCount must be > 0.")
		return nil, nil
	}

	rvs := &RenderViewSystem{
		MaxViewCount:    uint32(config.MaxViewCount),
		Lookup:          make(map[string]uint16),
		RegisteredViews: make([]*metadata.RenderView, config.MaxViewCount),
		renderer:        r,
	}
	// Fill the array with invalid entries.
	for i := uint32(0); i < rvs.MaxViewCount; i++ {
		rvs.Lookup[metadata.GenerateNewHash()] = loaders.InvalidIDUint16
		rvs.RegisteredViews[i] = &metadata.RenderView{
			ID: loaders.InvalidIDUint16,
		}
	}
	return rvs, nil
}

/**
 * @brief Shuts the render view system down.
 */
func (rvs *RenderViewSystem) Shutdown() error {
	return nil
}

/**
 * @brief Creates a new view using the provided config. The new
 * view may then be obtained via a call to render_view_system_get.
 *
 * @param config A constant pointer to the view configuration.
 * @return True on success; otherwise false.
 */
func (rvs *RenderViewSystem) Create(config *metadata.RenderViewConfig) bool {
	if config == nil {
		core.LogError("render_view_system_create requires a pointer to a valid config.")
		return false
	}

	if config.Name == "" {
		core.LogError("render_view_system_create: name is required")
		return false
	}

	if config.PassCount < 1 {
		core.LogError("render_view_system_create - Config must have at least one renderpass.")
		return false
	}

	// Make sure there is not already an entry with this name already registered.
	id, ok := rvs.Lookup[config.Name]
	if ok && id != loaders.InvalidIDUint16 {
		core.LogError("render_view_system_create - A view named '%s' already exists. A new one will not be created.", config.Name)
		return false
	}

	// Find a new id.
	for i := uint32(0); i < rvs.MaxViewCount; i++ {
		if rvs.RegisteredViews[i].ID == loaders.InvalidIDUint16 {
			id = uint16(i)
			break
		}
	}

	// Make sure a valid entry was found.
	if id == loaders.InvalidIDUint16 {
		core.LogError("render_view_system_create - No available space for a new view. Change system config to account for more.")
		return false
	}

	view := rvs.RegisteredViews[id]
	view.ID = id
	view.RenderViewType = config.RenderViewType
	// TODO: Leaking the name, create a destroy method and kill this.
	view.Name = config.Name
	view.CustomShaderName = config.CustomShaderName
	view.RenderpassCount = config.PassCount
	view.Passes = make([]*metadata.RenderPass, view.RenderpassCount)

	for i := uint8(0); i < view.RenderpassCount; i++ {
		view.Passes[i] = rvs.renderer.RenderPassGet(config.Passes[i].Name)
		if view.Passes[i] != nil {
			core.LogError("render_view_system_create - renderpass not found: '%s'.", config.Passes[i].Name)
			return false
		}
	}

	// TODO: Assign these function pointers to known functions based on the view type.
	// TODO: Factory pattern (with register, etc. for each type)?
	if config.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_WORLD {
		view.OnBuildPacket = views.RenderViewWorldOnBuildPacket     // For building the packet
		view.OnDestroyPacket = views.RenderViewWorldOnDestroyPacket // For destroying the packet.
		view.OnRender = views.RenderViewWorldOnRender               // For rendering the packet
		view.OnCreate = views.RenderViewWorldOnCreate
		view.OnDestroy = views.RenderViewWorldOnDestroy
		view.OnResize = views.RenderViewWorldOnResize
	} else if config.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_UI {
		view.OnBuildPacket = views.RenderViewUIOnBuildPacket     // For building the packet
		view.OnDestroyPacket = views.RenderViewUIOnDestroyPacket // For destroying the packet.
		view.OnRender = views.RenderViewUIOnRender               // For rendering the packet
		view.OnCreate = views.RenderViewUIOnCreate
		view.OnDestroy = views.RenderViewUIOnDestroy
		view.OnResize = views.RenderViewUIOnResize
	} else if config.RenderViewType == metadata.RENDERER_VIEW_KNOWN_TYPE_SKYBOX {
		view.OnBuildPacket = views.RenderViewSkyboxOnBuildPacket     // For building the packet
		view.OnDestroyPacket = views.RenderViewSkyboxOnDestroyPacket // For destroying the packet.
		view.OnRender = views.RenderViewSkyboxOnRender               // For rendering the packet
		view.OnCreate = views.RenderViewSkyboxOnCreate
		view.OnDestroy = views.RenderViewSkyboxOnDestroy
		view.OnResize = views.RenderViewSkyboxOnResize
	}

	// Call the on create
	if !view.OnCreate() {
		core.LogError("Failed to create view.")
		rvs.RegisteredViews[id] = nil
		return false
	}

	// Update the hashtable entry.
	rvs.Lookup[config.Name] = id

	return true
}

/**
 * @brief Called when the owner of this view (i.e. the window) is resized.
 *
 * @param width The new width in pixels.
 * @param width The new height in pixels.
 */
func (rvs *RenderViewSystem) OnWindowResize(width, height uint32) {
	// Send to all views
	for i := uint32(0); i < rvs.MaxViewCount; i++ {
		if rvs.RegisteredViews[i].ID != loaders.InvalidIDUint16 {
			rvs.RegisteredViews[i].OnResize(width, height)
		}
	}
}

/**
 * @brief Obtains a pointer to a view with the given name.
 *
 * @param name The name of the view.
 * @return A pointer to a view if found; otherwise 0.
 */
func (rvs *RenderViewSystem) Get(name string) *metadata.RenderView {
	if id, ok := rvs.Lookup[name]; ok && id != loaders.InvalidIDUint16 {
		return rvs.RegisteredViews[id]
	}
	return nil
}

/**
 * @brief Builds a render view packet using the provided view and meshes.
 *
 * @param view A pointer to the view to use.
 * @param data Freeform data used to build the packet.
 * @param out_packet A pointer to hold the generated packet.
 * @return True on success; otherwise false.
 */
func (rvs *RenderViewSystem) BuildPacket(view *metadata.RenderView, data interface{}) *metadata.RenderViewPacket {
	if view != nil {
		op, err := view.OnBuildPacket(data)
		if err != nil {
			core.LogError(err.Error())
			return nil
		}
		return op
	}
	core.LogError("render_view_system_build_packet requires valid pointers to a view and a packet.")
	return nil
}

/**
 * @brief Uses the given view and packet to render the contents therein.
 *
 * @param view A pointer to the view to use.
 * @param packet A pointer to the packet whose data is to be rendered.
 * @param frame_number The current renderer frame number, typically used for data synchronization.
 * @param render_target_index The current render target index for renderers that use multiple render targets at once (i.e. Vulkan).
 * @return True on success; otherwise false.
 */
func (rvs *RenderViewSystem) OnRender(view *metadata.RenderView, packet *metadata.RenderViewPacket, frameNumber, renderTargetIndex uint64) bool {
	if view != nil && packet != nil {
		return view.OnRender(packet, frameNumber, renderTargetIndex)
	}
	core.LogError("render_view_system_on_render requires a valid pointer to a data.")
	return false
}

package entityresolver

import (
	"context"
	"time"
)

const (
	EntityTypeService            = "Service"
	EntityTypeServiceInstance    = "ServiceInstance"
	EntityTypeEndpoint           = "Endpoint"
	EntityTypePod                = "Pod"
	EntityTypeNode               = "Node"
	EntityTypeContainer          = "Container"
	EntityTypeHost               = "Host"
	EntityTypeNamespace          = "Namespace"
	EntityTypeCluster            = "Cluster"
	EntityTypeEnvironment        = "Environment"
	EntityTypeRegion             = "Region"
	EntityTypeAvailabilityZone   = "AvailabilityZone"
	EntityTypeDeployment         = "Deployment"
	EntityTypeDatabase           = "Database"
	EntityTypeMessagingTopic     = "MessagingTopic"
	EntityTypeExternalDependency = "ExternalDependency"
	EntityTypeChangeEvent        = "ChangeEvent"

	RelationshipServiceHasInstance        = "SERVICE_HAS_INSTANCE"
	RelationshipServiceExposesEndpoint    = "SERVICE_EXPOSES_ENDPOINT"
	RelationshipServiceCallsService       = "SERVICE_CALLS_SERVICE"
	RelationshipServiceCallsDatabase      = "SERVICE_CALLS_DATABASE"
	RelationshipServiceProducesTopic      = "SERVICE_PRODUCES_TOPIC"
	RelationshipServiceConsumesTopic      = "SERVICE_CONSUMES_TOPIC"
	RelationshipInstanceRunsOnNode        = "INSTANCE_RUNS_ON_NODE"
	RelationshipPodRunsOnNode             = "POD_RUNS_ON_NODE"
	RelationshipContainerInPod            = "CONTAINER_IN_POD"
	RelationshipPodBelongsToNamespace     = "POD_BELONGS_TO_NAMESPACE"
	RelationshipNodeBelongsToCluster      = "NODE_BELONGS_TO_CLUSTER"
	RelationshipServiceDeployedInEnv      = "SERVICE_DEPLOYED_IN_ENVIRONMENT"
	RelationshipServiceAffectedByChange   = "SERVICE_AFFECTED_BY_CHANGE"
	RelationshipChangeTargetsService      = "CHANGE_TARGETS_SERVICE"
	RelationshipEndpointBelongsToService  = "ENDPOINT_BELONGS_TO_SERVICE"
	RelationshipDeploymentBelongsToEnv    = "DEPLOYMENT_BELONGS_TO_ENVIRONMENT"
	RelationshipRegionBelongsToProvider   = "REGION_BELONGS_TO_PROVIDER"
	RelationshipAvailabilityZoneInRegion  = "AVAILABILITY_ZONE_IN_REGION"
	RelationshipServiceRunsDeployment     = "SERVICE_RUNS_DEPLOYMENT"
	RelationshipHostBelongsToAvailability = "HOST_BELONGS_TO_AVAILABILITY_ZONE"
)

const (
	ChangeTypeDeployment   = "DEPLOYMENT"
	ChangeTypeConfigChange = "CONFIG_CHANGE"
	ChangeTypeImageChange  = "IMAGE_CHANGE"
	ChangeTypePodRestart   = "POD_RESTART"
	ChangeTypeInfraChange  = "INFRA_CHANGE"
	ChangeTypeUnknown      = "UNKNOWN_CHANGE"
)

type Entity struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	Cluster     string            `json:"cluster,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Provider    string            `json:"provider,omitempty"`
	Region      string            `json:"region,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	FirstSeenAt time.Time         `json:"first_seen_at"`
	LastSeenAt  time.Time         `json:"last_seen_at"`
	Source      string            `json:"source"`
}

type EntityRef struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type Relationship struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	SourceEntityID string            `json:"source_entity_id"`
	TargetEntityID string            `json:"target_entity_id"`
	Attributes     map[string]string `json:"attributes,omitempty"`
	FirstSeenAt    time.Time         `json:"first_seen_at"`
	LastSeenAt     time.Time         `json:"last_seen_at"`
	Source         string            `json:"source"`
	EvidenceCount  int64             `json:"evidence_count,omitempty"`
}

type EntityObservation struct {
	EntityID   string            `json:"entity_id"`
	EntityType string            `json:"entity_type"`
	ObservedAt time.Time         `json:"observed_at"`
	Source     string            `json:"source"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type ServiceHealthWindow struct {
	ServiceID      string    `json:"service_id"`
	Window         string    `json:"window"`
	WindowStart    time.Time `json:"window_start"`
	WindowEnd      time.Time `json:"window_end"`
	RequestCount   int64     `json:"request_count"`
	ErrorCount     int64     `json:"error_count"`
	ErrorRate      float64   `json:"error_rate"`
	DurationP50Ms  float64   `json:"duration_p50_ms"`
	DurationP95Ms  float64   `json:"duration_p95_ms"`
	DurationP99Ms  float64   `json:"duration_p99_ms"`
	LogErrorCount  int64     `json:"log_error_count"`
	SpanErrorCount int64     `json:"span_error_count"`
	Source         string    `json:"source"`
}

type ChangeEvent struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	TargetID   string            `json:"target_id"`
	TargetType string            `json:"target_type"`
	Timestamp  time.Time         `json:"timestamp"`
	Source     string            `json:"source"`
	Version    string            `json:"version,omitempty"`
	Revision   string            `json:"revision,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type ResolverResult struct {
	Entities      []Entity              `json:"entities"`
	Relationships []Relationship        `json:"relationships"`
	Observations  []EntityObservation   `json:"observations,omitempty"`
	HealthWindows []ServiceHealthWindow `json:"health_windows,omitempty"`
	ChangeEvents  []ChangeEvent         `json:"change_events,omitempty"`
}

type TopologyGraph struct {
	Nodes []Entity       `json:"nodes"`
	Edges []Relationship `json:"edges"`
}

type EntityGraphStore interface {
	Initialize(ctx context.Context) error
	Health(ctx context.Context) error
	UpsertEntities(ctx context.Context, entities []Entity) error
	UpsertRelationships(ctx context.Context, relationships []Relationship) error
	LinkChangeEvents(ctx context.Context, events []ChangeEvent) error
	GetEntity(ctx context.Context, id string) (*Entity, error)
	ListEntities(ctx context.Context, entityType string) ([]Entity, error)
	GetEntityRelationships(ctx context.Context, id string, depth int) ([]Relationship, error)
	GetServiceTopology(ctx context.Context, serviceID string, depth int) (*TopologyGraph, error)
	Shutdown(ctx context.Context) error
}

type EntityAnalyticsStore interface {
	InsertServiceHealthWindows(ctx context.Context, windows []ServiceHealthWindow) error
	InsertChangeEvents(ctx context.Context, events []ChangeEvent) error
	GetServiceHealth(ctx context.Context, serviceID string, window string, since time.Time) ([]ServiceHealthWindow, error)
	GetChangeEvents(ctx context.Context, targetID string, since time.Time) ([]ChangeEvent, error)
}

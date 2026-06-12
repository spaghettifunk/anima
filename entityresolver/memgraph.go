package entityresolver

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/spaghettifunk/anima/internal/config"
)

type MemgraphStore struct {
	uri      string
	username string
	password string
	database string
	tls      bool
	driver   neo4j.Driver
}

func NewMemgraphStore(uri, username, password, database string, tls bool) *MemgraphStore {
	return &MemgraphStore{uri: uri, username: username, password: password, database: database, tls: tls}
}

func NewMemgraphStoreFromConfig(cfg config.Config) *MemgraphStore {
	return NewMemgraphStore(cfg.Memgraph.Address, cfg.Memgraph.Username, cfg.Memgraph.Password, cfg.Memgraph.Database, cfg.Memgraph.TLS)
}

func (s *MemgraphStore) Initialize(ctx context.Context) error {
	if strings.TrimSpace(s.uri) == "" {
		return fmt.Errorf("memgraph address is required")
	}
	driver, err := neo4j.NewDriver(s.driverURI(), neo4j.BasicAuth(s.username, s.password, ""))
	if err != nil {
		return err
	}
	s.driver = driver
	if err := s.driver.VerifyConnectivity(ctx); err != nil {
		_ = s.driver.Close(ctx)
		s.driver = nil
		return err
	}
	return s.ensureIndexes(ctx)
}

func (s *MemgraphStore) Health(ctx context.Context) error {
	if s == nil || s.driver == nil {
		return fmt.Errorf("memgraph store is not initialized")
	}
	return s.driver.VerifyConnectivity(ctx)
}

func (s *MemgraphStore) UpsertEntities(ctx context.Context, entities []Entity) error {
	if len(entities) == 0 {
		return nil
	}
	session := s.session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, entity := range entities {
			label, err := labelForEntityType(entity.Type)
			if err != nil {
				return nil, err
			}
			query := fmt.Sprintf(`
MERGE (e:%s {id: $id})
ON CREATE SET e.type = $type, e.name = $name, e.first_seen_at = $first_seen_at, e.source = $source
SET e.last_seen_at = $last_seen_at,
    e.name = $name,
    e.namespace = $namespace,
    e.cluster = $cluster,
    e.environment = $environment,
    e.provider = $provider,
    e.region = $region,
    e.attributes = $attributes,
    e.source = $source
`, label)
			if _, err := tx.Run(ctx, query, entityParams(entity)); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (s *MemgraphStore) UpsertRelationships(ctx context.Context, relationships []Relationship) error {
	if len(relationships) == 0 {
		return nil
	}
	session := s.session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, relationship := range relationships {
			relType, err := cypherRelationshipType(relationship.Type)
			if err != nil {
				return nil, err
			}
			query := fmt.Sprintf(`
MATCH (source {id: $source_id})
MATCH (target {id: $target_id})
MERGE (source)-[r:%s {id: $id}]->(target)
ON CREATE SET r.type = $type, r.first_seen_at = $first_seen_at, r.source = $source, r.evidence_count = 0
SET r.last_seen_at = $last_seen_at,
    r.attributes = $attributes,
    r.source = $source,
    r.evidence_count = coalesce(r.evidence_count, 0) + $evidence_count
`, relType)
			if _, err := tx.Run(ctx, query, relationshipParams(relationship)); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

func (s *MemgraphStore) LinkChangeEvents(ctx context.Context, events []ChangeEvent) error {
	entities := make([]Entity, 0, len(events))
	relationships := make([]Relationship, 0, len(events)*2)
	for _, event := range events {
		entities = append(entities, Entity{ID: event.ID, Type: EntityTypeChangeEvent, Name: NormalizeName(event.Type), Attributes: event.Attributes, FirstSeenAt: event.Timestamp, LastSeenAt: event.Timestamp, Source: event.Source})
		relationships = append(relationships,
			Relationship{ID: RelationshipID(RelationshipChangeTargetsService, event.ID, event.TargetID), Type: RelationshipChangeTargetsService, SourceEntityID: event.ID, TargetEntityID: event.TargetID, FirstSeenAt: event.Timestamp, LastSeenAt: event.Timestamp, Source: event.Source, EvidenceCount: 1},
			Relationship{ID: RelationshipID(RelationshipServiceAffectedByChange, event.TargetID, event.ID), Type: RelationshipServiceAffectedByChange, SourceEntityID: event.TargetID, TargetEntityID: event.ID, FirstSeenAt: event.Timestamp, LastSeenAt: event.Timestamp, Source: event.Source, EvidenceCount: 1},
		)
	}
	if err := s.UpsertEntities(ctx, entities); err != nil {
		return err
	}
	return s.UpsertRelationships(ctx, relationships)
}

func (s *MemgraphStore) GetEntity(ctx context.Context, id string) (*Entity, error) {
	session := s.session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx)
	result, err := session.Run(ctx, `MATCH (e {id: $id}) RETURN properties(e) AS props LIMIT 1`, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	if !result.Next(ctx) {
		if err := result.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("entity not found")
	}
	entity := entityFromRecord(result.Record().Values[0])
	return &entity, nil
}

func (s *MemgraphStore) ListEntities(ctx context.Context, entityType string) ([]Entity, error) {
	label := ""
	var err error
	if strings.TrimSpace(entityType) != "" {
		label, err = labelForEntityType(normalizeEntityType(entityType))
		if err != nil {
			return nil, err
		}
		label = ":" + label
	}
	session := s.session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx)
	result, err := session.Run(ctx, fmt.Sprintf(`MATCH (e%s) RETURN properties(e) AS props ORDER BY e.id`, label), nil)
	if err != nil {
		return nil, err
	}
	var out []Entity
	for result.Next(ctx) {
		out = append(out, entityFromRecord(result.Record().Values[0]))
	}
	return out, result.Err()
}

func (s *MemgraphStore) GetEntityRelationships(ctx context.Context, id string, depth int) ([]Relationship, error) {
	depth = normalizeDepth(depth)
	session := s.session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx)
	query := fmt.Sprintf(`
MATCH (e {id: $id})-[r*1..%d]-(n)
UNWIND r AS rel
RETURN DISTINCT properties(rel) AS props, startNode(rel).id AS source_id, endNode(rel).id AS target_id
ORDER BY props.id
`, depth)
	result, err := session.Run(ctx, query, map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	var out []Relationship
	for result.Next(ctx) {
		record := result.Record()
		rel := relationshipFromRecord(record.Values[0])
		if source, ok := record.Values[1].(string); ok {
			rel.SourceEntityID = source
		}
		if target, ok := record.Values[2].(string); ok {
			rel.TargetEntityID = target
		}
		out = append(out, rel)
	}
	return out, result.Err()
}

func (s *MemgraphStore) GetServiceTopology(ctx context.Context, serviceID string, depth int) (*TopologyGraph, error) {
	depth = normalizeDepth(depth)
	session := s.session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx)
	query := fmt.Sprintf(`
MATCH path = (service {id: $id})-[*1..%d]-(n)
WHERE service.type = 'Service'
UNWIND nodes(path) AS node
UNWIND relationships(path) AS rel
RETURN collect(DISTINCT properties(node)) AS nodes,
       collect(DISTINCT {props: properties(rel), source_id: startNode(rel).id, target_id: endNode(rel).id}) AS edges
`, depth)
	result, err := session.Run(ctx, query, map[string]any{"id": serviceID})
	if err != nil {
		return nil, err
	}
	graph := &TopologyGraph{}
	if result.Next(ctx) {
		record := result.Record()
		graph.Nodes = entitiesFromList(record.Values[0])
		graph.Edges = relationshipsFromList(record.Values[1])
	}
	if err := result.Err(); err != nil {
		return nil, err
	}
	return graph, nil
}

func (s *MemgraphStore) Shutdown(ctx context.Context) error {
	if s != nil && s.driver != nil {
		err := s.driver.Close(ctx)
		s.driver = nil
		return err
	}
	return nil
}

func (s *MemgraphStore) ensureIndexes(ctx context.Context) error {
	session := s.session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, label := range entityLabels {
			if _, err := tx.Run(ctx, fmt.Sprintf("CREATE INDEX ON :%s(id)", label), nil); err != nil {
				if !strings.Contains(strings.ToLower(err.Error()), "already") {
					return nil, err
				}
			}
		}
		return nil, nil
	})
	return err
}

func (s *MemgraphStore) session(ctx context.Context, mode neo4j.AccessMode) neo4j.SessionWithContext {
	return s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: mode, DatabaseName: s.database})
}

func (s *MemgraphStore) driverURI() string {
	if !s.tls {
		return s.uri
	}
	parsed, err := url.Parse(s.uri)
	if err != nil || parsed.Scheme == "" {
		return s.uri
	}
	if parsed.Scheme == "bolt" {
		parsed.Scheme = "bolt+s"
	}
	return parsed.String()
}

func entityParams(entity Entity) map[string]any {
	return map[string]any{
		"id": entity.ID, "type": entity.Type, "name": entity.Name,
		"namespace": entity.Namespace, "cluster": entity.Cluster, "environment": entity.Environment,
		"provider": entity.Provider, "region": entity.Region, "attributes": entity.Attributes,
		"first_seen_at": entity.FirstSeenAt.UTC(), "last_seen_at": entity.LastSeenAt.UTC(), "source": entity.Source,
	}
}

func relationshipParams(relationship Relationship) map[string]any {
	evidenceCount := relationship.EvidenceCount
	if evidenceCount <= 0 {
		evidenceCount = 1
	}
	return map[string]any{
		"id": relationship.ID, "type": relationship.Type, "source_id": relationship.SourceEntityID, "target_id": relationship.TargetEntityID,
		"attributes": relationship.Attributes, "first_seen_at": relationship.FirstSeenAt.UTC(), "last_seen_at": relationship.LastSeenAt.UTC(),
		"source": relationship.Source, "evidence_count": evidenceCount,
	}
}

var entityLabels = []string{
	EntityTypeService, EntityTypeServiceInstance, EntityTypeEndpoint, EntityTypePod, EntityTypeNode,
	EntityTypeContainer, EntityTypeHost, EntityTypeNamespace, EntityTypeCluster, EntityTypeEnvironment,
	EntityTypeRegion, EntityTypeAvailabilityZone, EntityTypeDeployment, EntityTypeDatabase,
	EntityTypeMessagingTopic, EntityTypeExternalDependency, EntityTypeChangeEvent,
}

func labelForEntityType(entityType string) (string, error) {
	for _, label := range entityLabels {
		if label == entityType {
			return label, nil
		}
	}
	return "", fmt.Errorf("unsupported entity type %q", entityType)
}

func normalizeEntityType(value string) string {
	value = strings.TrimSpace(value)
	for _, label := range entityLabels {
		if strings.EqualFold(label, value) {
			return label
		}
	}
	if strings.EqualFold(value, "service") {
		return EntityTypeService
	}
	return value
}

func cypherRelationshipType(value string) (string, error) {
	switch value {
	case RelationshipServiceHasInstance, RelationshipServiceExposesEndpoint, RelationshipServiceCallsService,
		RelationshipServiceCallsDatabase, RelationshipServiceProducesTopic, RelationshipServiceConsumesTopic,
		RelationshipInstanceRunsOnNode, RelationshipPodRunsOnNode, RelationshipContainerInPod,
		RelationshipPodBelongsToNamespace, RelationshipNodeBelongsToCluster, RelationshipServiceDeployedInEnv,
		RelationshipServiceAffectedByChange, RelationshipChangeTargetsService, RelationshipEndpointBelongsToService,
		RelationshipDeploymentBelongsToEnv, RelationshipRegionBelongsToProvider, RelationshipAvailabilityZoneInRegion,
		RelationshipServiceRunsDeployment, RelationshipHostBelongsToAvailability:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported relationship type %q", value)
	}
}

func normalizeDepth(depth int) int {
	if depth <= 0 {
		return 1
	}
	if depth > 3 {
		return 3
	}
	return depth
}

func entityFromRecord(value any) Entity {
	props := anyMap(value)
	return Entity{
		ID:          stringProp(props, "id"),
		Type:        stringProp(props, "type"),
		Name:        stringProp(props, "name"),
		Namespace:   stringProp(props, "namespace"),
		Cluster:     stringProp(props, "cluster"),
		Environment: stringProp(props, "environment"),
		Provider:    stringProp(props, "provider"),
		Region:      stringProp(props, "region"),
		Attributes:  stringMapProp(props, "attributes"),
		FirstSeenAt: timeProp(props, "first_seen_at"),
		LastSeenAt:  timeProp(props, "last_seen_at"),
		Source:      stringProp(props, "source"),
	}
}

func relationshipFromRecord(value any) Relationship {
	props := anyMap(value)
	return Relationship{
		ID:             stringProp(props, "id"),
		Type:           stringProp(props, "type"),
		SourceEntityID: stringProp(props, "source_id"),
		TargetEntityID: stringProp(props, "target_id"),
		Attributes:     stringMapProp(props, "attributes"),
		FirstSeenAt:    timeProp(props, "first_seen_at"),
		LastSeenAt:     timeProp(props, "last_seen_at"),
		Source:         stringProp(props, "source"),
		EvidenceCount:  int64Prop(props, "evidence_count"),
	}
}

func entitiesFromList(value any) []Entity {
	values, _ := value.([]any)
	out := make([]Entity, 0, len(values))
	for _, item := range values {
		out = append(out, entityFromRecord(item))
	}
	return out
}

func relationshipsFromList(value any) []Relationship {
	values, _ := value.([]any)
	out := make([]Relationship, 0, len(values))
	for _, item := range values {
		props := anyMap(item)
		rel := relationshipFromRecord(props["props"])
		rel.SourceEntityID = stringProp(props, "source_id")
		rel.TargetEntityID = stringProp(props, "target_id")
		out = append(out, rel)
	}
	return out
}

func anyMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func stringProp(props map[string]any, key string) string {
	if value, ok := props[key].(string); ok {
		return value
	}
	return ""
}

func stringMapProp(props map[string]any, key string) map[string]string {
	out := map[string]string{}
	switch typed := props[key].(type) {
	case map[string]string:
		return typed
	case map[string]any:
		for k, v := range typed {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func timeProp(props map[string]any, key string) time.Time {
	if value, ok := props[key].(time.Time); ok {
		return value
	}
	return time.Time{}
}

func int64Prop(props map[string]any, key string) int64 {
	switch value := props[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	}
	return 0
}

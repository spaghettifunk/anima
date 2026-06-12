package entityresolver

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const sourceOTLP = "otlp"

func ResolveFromMetrics(md pmetric.Metrics) ResolverResult {
	builder := newResultBuilder()
	now := time.Now().UTC()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resourceAttrs := attrsToMap(rm.Resource().Attributes())
		ctx := builder.addResourceEntities(resourceAttrs, now)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			metrics := rm.ScopeMetrics().At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				for _, attrs := range metricPointAttrs(metrics.At(k)) {
					builder.addEndpointFromAttrs(ctx, mergeAttrs(resourceAttrs, attrs), now)
				}
			}
		}
	}
	return builder.result()
}

func ResolveFromLogs(ld plog.Logs) ResolverResult {
	builder := newResultBuilder()
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		resourceAttrs := attrsToMap(rl.Resource().Attributes())
		ctx := builder.addResourceEntities(resourceAttrs, time.Now().UTC())
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			records := rl.ScopeLogs().At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				record := records.At(k)
				observedAt := timestampOrNow(record.Timestamp())
				attrs := mergeAttrs(resourceAttrs, attrsToMap(record.Attributes()))
				builder.addChangeEvent(ctx, attrs, observedAt)
				if isErrorSeverity(record.SeverityNumber(), record.SeverityText(), attrs) && ctx.Service.ID != "" {
					builder.addLogError(ctx.Service.ID, observedAt)
				}
			}
		}
	}
	return builder.result()
}

func ResolveFromTraces(td ptrace.Traces) ResolverResult {
	builder := newResultBuilder()
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		resourceAttrs := attrsToMap(rs.Resource().Attributes())
		resourceCtx := builder.addResourceEntities(resourceAttrs, time.Now().UTC())
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			spans := rs.ScopeSpans().At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				observedAt := timestampOrNow(span.StartTimestamp())
				attrs := mergeAttrs(resourceAttrs, attrsToMap(span.Attributes()))
				spanCtx := resourceCtx
				if overrideService := strings.TrimSpace(attrs["service.name"]); overrideService != "" && overrideService != resourceCtx.Service.Name {
					spanCtx = builder.addResourceEntities(attrs, observedAt)
				}
				builder.addEndpointFromSpan(spanCtx, span, attrs, observedAt)
				builder.addDependencyFromSpan(spanCtx, span, attrs, observedAt)
				builder.addSpanHealth(spanCtx, span, attrs, observedAt)
				builder.addChangeEvent(spanCtx, attrs, observedAt)
			}
		}
	}
	return builder.result()
}

type resourceContext struct {
	Service     Entity
	Instance    Entity
	Pod         Entity
	Node        Entity
	Container   Entity
	Namespace   Entity
	Cluster     Entity
	Environment Entity
}

type resultBuilder struct {
	entities      map[string]Entity
	relationships map[string]Relationship
	events        map[string]ChangeEvent
	health        *healthAccumulator
}

func newResultBuilder() *resultBuilder {
	return &resultBuilder{
		entities:      map[string]Entity{},
		relationships: map[string]Relationship{},
		events:        map[string]ChangeEvent{},
		health:        newHealthAccumulator([]time.Duration{time.Minute, 5 * time.Minute}),
	}
}

func (b *resultBuilder) result() ResolverResult {
	entities := make([]Entity, 0, len(b.entities))
	for _, entity := range b.entities {
		entities = append(entities, entity)
	}
	sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })

	relationships := make([]Relationship, 0, len(b.relationships))
	for _, relationship := range b.relationships {
		relationships = append(relationships, relationship)
	}
	sort.Slice(relationships, func(i, j int) bool { return relationships[i].ID < relationships[j].ID })

	events := make([]ChangeEvent, 0, len(b.events))
	for _, event := range b.events {
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].ID < events[j].ID })

	return ResolverResult{
		Entities:      entities,
		Relationships: relationships,
		HealthWindows: b.health.windows(),
		ChangeEvents:  events,
	}
}

func (b *resultBuilder) addResourceEntities(attrs map[string]string, observedAt time.Time) resourceContext {
	cluster := firstAttr(attrs, "k8s.cluster.name", "cluster", "clusterName")
	namespace := firstAttr(attrs, "service.namespace", "k8s.namespace.name", "namespace", "kubernetes_namespace")
	serviceName := firstAttr(attrs, "service.name", "service_name", "service", "job")
	environment := firstAttr(attrs, "deployment.environment.name", "deployment.environment", "environment")
	provider := firstAttr(attrs, "cloud.provider")
	region := firstAttr(attrs, "cloud.region")
	zone := firstAttr(attrs, "cloud.availability_zone")

	ctx := resourceContext{}
	if strings.TrimSpace(serviceName) != "" {
		ctx.Service = b.upsertEntity(Entity{
			ID:          ServiceID(cluster, namespace, serviceName),
			Type:        EntityTypeService,
			Name:        NormalizeName(serviceName),
			Namespace:   NormalizeField(namespace),
			Cluster:     NormalizeField(cluster),
			Environment: NormalizeField(environment),
			Provider:    strings.TrimSpace(provider),
			Region:      strings.TrimSpace(region),
			Attributes:  pickAttrs(attrs, "service.name", "service.namespace", "service.version", "deployment.environment.name", "deployment.environment"),
			FirstSeenAt: observedAt,
			LastSeenAt:  observedAt,
			Source:      sourceOTLP,
		})
	}
	if strings.TrimSpace(cluster) != "" {
		ctx.Cluster = b.upsertEntity(Entity{ID: ClusterID(cluster), Type: EntityTypeCluster, Name: NormalizeName(cluster), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	}
	if strings.TrimSpace(namespace) != "" {
		ctx.Namespace = b.upsertEntity(Entity{ID: NamespaceID(cluster, namespace), Type: EntityTypeNamespace, Name: NormalizeName(namespace), Cluster: NormalizeField(cluster), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	}
	if strings.TrimSpace(environment) != "" {
		ctx.Environment = b.upsertEntity(Entity{ID: EnvironmentID(environment), Type: EntityTypeEnvironment, Name: NormalizeName(environment), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	}
	if strings.TrimSpace(region) != "" {
		b.upsertEntity(Entity{ID: RegionID(provider, region), Type: EntityTypeRegion, Name: NormalizeName(region), Provider: strings.TrimSpace(provider), Region: NormalizeField(region), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	}
	if strings.TrimSpace(zone) != "" {
		b.upsertEntity(Entity{ID: AvailabilityZoneID(provider, region, zone), Type: EntityTypeAvailabilityZone, Name: NormalizeName(zone), Provider: strings.TrimSpace(provider), Region: strings.TrimSpace(region), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	}

	instanceID := firstAttr(attrs, "service.instance.id")
	if ctx.Service.ID != "" && strings.TrimSpace(instanceID) != "" {
		ctx.Instance = b.upsertEntity(Entity{ID: ServiceInstanceID(ctx.Service.ID, instanceID), Type: EntityTypeServiceInstance, Name: NormalizeName(instanceID), Namespace: NormalizeField(namespace), Cluster: NormalizeField(cluster), Environment: NormalizeField(environment), Attributes: pickAttrs(attrs, "service.instance.id", "service.version"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		b.addRelationship(RelationshipServiceHasInstance, ctx.Service.ID, ctx.Instance.ID, nil, observedAt)
	}

	nodeName := firstAttr(attrs, "k8s.node.name")
	if strings.TrimSpace(nodeName) != "" {
		ctx.Node = b.upsertEntity(Entity{ID: NodeID(cluster, nodeName), Type: EntityTypeNode, Name: NormalizeName(nodeName), Cluster: NormalizeField(cluster), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		if ctx.Cluster.ID != "" {
			b.addRelationship(RelationshipNodeBelongsToCluster, ctx.Node.ID, ctx.Cluster.ID, nil, observedAt)
		}
	}
	podName := firstAttr(attrs, "k8s.pod.name")
	podUID := firstAttr(attrs, "k8s.pod.uid")
	if strings.TrimSpace(podUID) != "" || strings.TrimSpace(podName) != "" {
		ctx.Pod = b.upsertEntity(Entity{ID: PodID(cluster, namespace, podUID, podName), Type: EntityTypePod, Name: NormalizeName(firstNonEmpty(podName, podUID)), Namespace: NormalizeField(namespace), Cluster: NormalizeField(cluster), Attributes: pickAttrs(attrs, "k8s.pod.name", "k8s.pod.uid"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		if ctx.Node.ID != "" {
			b.addRelationship(RelationshipPodRunsOnNode, ctx.Pod.ID, ctx.Node.ID, nil, observedAt)
		}
		if ctx.Namespace.ID != "" {
			b.addRelationship(RelationshipPodBelongsToNamespace, ctx.Pod.ID, ctx.Namespace.ID, nil, observedAt)
		}
	}
	containerName := firstAttr(attrs, "k8s.container.name", "container.name")
	containerID := firstAttr(attrs, "container.id")
	if strings.TrimSpace(containerID) != "" || strings.TrimSpace(containerName) != "" {
		ctx.Container = b.upsertEntity(Entity{ID: ContainerID(cluster, namespace, podUID, podName, containerID, containerName), Type: EntityTypeContainer, Name: NormalizeName(firstNonEmpty(containerName, containerID)), Namespace: NormalizeField(namespace), Cluster: NormalizeField(cluster), Attributes: pickAttrs(attrs, "container.id", "container.name", "k8s.container.name", "container.image.name", "container.image.tag"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		if ctx.Pod.ID != "" {
			b.addRelationship(RelationshipContainerInPod, ctx.Container.ID, ctx.Pod.ID, nil, observedAt)
		}
	}
	if ctx.Instance.ID != "" && ctx.Node.ID != "" {
		b.addRelationship(RelationshipInstanceRunsOnNode, ctx.Instance.ID, ctx.Node.ID, nil, observedAt)
	}
	if ctx.Service.ID != "" && ctx.Environment.ID != "" {
		b.addRelationship(RelationshipServiceDeployedInEnv, ctx.Service.ID, ctx.Environment.ID, nil, observedAt)
	}
	if deployment := firstAttr(attrs, "k8s.deployment.name"); strings.TrimSpace(deployment) != "" {
		dep := b.upsertEntity(Entity{ID: DeploymentID(cluster, namespace, deployment), Type: EntityTypeDeployment, Name: NormalizeName(deployment), Namespace: NormalizeField(namespace), Cluster: NormalizeField(cluster), Environment: NormalizeField(environment), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		if ctx.Service.ID != "" {
			b.addRelationship(RelationshipServiceRunsDeployment, ctx.Service.ID, dep.ID, nil, observedAt)
		}
	}
	if hostName := firstAttr(attrs, "host.name"); strings.TrimSpace(hostName) != "" {
		b.upsertEntity(Entity{ID: HostID(cluster, hostName), Type: EntityTypeHost, Name: NormalizeName(hostName), Cluster: NormalizeField(cluster), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	}
	return ctx
}

func (b *resultBuilder) addEndpointFromSpan(ctx resourceContext, span ptrace.Span, attrs map[string]string, observedAt time.Time) {
	if ctx.Service.ID == "" || span.Kind() != ptrace.SpanKindServer {
		return
	}
	method := firstAttr(attrs, "http.request.method", "rpc.method")
	route := firstAttr(attrs, "http.route", "url.path")
	if route == "" && strings.TrimSpace(method) != "" {
		route = span.Name()
	}
	if strings.TrimSpace(route) == "" {
		return
	}
	endpoint := b.upsertEntity(Entity{
		ID:          EndpointID(ctx.Service.ID, method, route),
		Type:        EntityTypeEndpoint,
		Name:        NormalizeName(strings.TrimSpace(firstNonEmpty(method, "ANY")) + " " + SanitizeRoute(route)),
		Namespace:   ctx.Service.Namespace,
		Cluster:     ctx.Service.Cluster,
		Environment: ctx.Service.Environment,
		Attributes:  pickAttrs(attrs, "http.request.method", "http.route", "url.path", "rpc.system", "rpc.service", "rpc.method"),
		FirstSeenAt: observedAt,
		LastSeenAt:  observedAt,
		Source:      sourceOTLP,
	})
	b.addRelationship(RelationshipServiceExposesEndpoint, ctx.Service.ID, endpoint.ID, nil, observedAt)
	b.addRelationship(RelationshipEndpointBelongsToService, endpoint.ID, ctx.Service.ID, nil, observedAt)
}

func (b *resultBuilder) addEndpointFromAttrs(ctx resourceContext, attrs map[string]string, observedAt time.Time) {
	if ctx.Service.ID == "" {
		return
	}
	method := firstAttr(attrs, "http.request.method", "rpc.method")
	route := firstAttr(attrs, "http.route", "url.path")
	if strings.TrimSpace(route) == "" {
		return
	}
	endpoint := b.upsertEntity(Entity{ID: EndpointID(ctx.Service.ID, method, route), Type: EntityTypeEndpoint, Name: NormalizeName(firstNonEmpty(method, "ANY") + " " + SanitizeRoute(route)), Namespace: ctx.Service.Namespace, Cluster: ctx.Service.Cluster, Environment: ctx.Service.Environment, Attributes: pickAttrs(attrs, "http.request.method", "http.route", "url.path", "rpc.system", "rpc.service", "rpc.method"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	b.addRelationship(RelationshipServiceExposesEndpoint, ctx.Service.ID, endpoint.ID, nil, observedAt)
	b.addRelationship(RelationshipEndpointBelongsToService, endpoint.ID, ctx.Service.ID, nil, observedAt)
}

func (b *resultBuilder) addDependencyFromSpan(ctx resourceContext, span ptrace.Span, attrs map[string]string, observedAt time.Time) {
	if ctx.Service.ID == "" {
		return
	}
	switch span.Kind() {
	case ptrace.SpanKindClient, ptrace.SpanKindProducer, ptrace.SpanKindConsumer:
	default:
		return
	}

	if dbName := firstAttr(attrs, "db.name", "db.namespace"); dbName != "" {
		dbSystem := firstAttr(attrs, "db.system.name", "db.system")
		db := b.upsertEntity(Entity{ID: DatabaseID(ctx.Service.Environment, dbSystem, dbName), Type: EntityTypeDatabase, Name: NormalizeName(dbName), Environment: ctx.Service.Environment, Attributes: pickAttrs(attrs, "db.system.name", "db.system", "db.name", "db.namespace", "db.operation.name"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		b.addRelationship(RelationshipServiceCallsDatabase, ctx.Service.ID, db.ID, evidenceAttrs(span, attrs), observedAt)
		return
	}

	if destination := firstAttr(attrs, "messaging.destination.name"); destination != "" {
		system := firstAttr(attrs, "messaging.system")
		topic := b.upsertEntity(Entity{ID: MessagingTopicID(ctx.Service.Environment, system, destination), Type: EntityTypeMessagingTopic, Name: NormalizeName(destination), Environment: ctx.Service.Environment, Attributes: pickAttrs(attrs, "messaging.system", "messaging.destination.name", "messaging.operation.name"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		relType := RelationshipServiceProducesTopic
		if span.Kind() == ptrace.SpanKindConsumer || strings.Contains(strings.ToLower(firstAttr(attrs, "messaging.operation.name")), "receive") || strings.Contains(strings.ToLower(firstAttr(attrs, "messaging.operation.name")), "process") {
			relType = RelationshipServiceConsumesTopic
		}
		b.addRelationship(relType, ctx.Service.ID, topic.ID, evidenceAttrs(span, attrs), observedAt)
		return
	}

	target := firstAttr(attrs, "peer.service", "rpc.service", "server.address", "net.peer.name")
	if strings.TrimSpace(target) == "" {
		return
	}
	if IsIP(target) || (firstAttr(attrs, "peer.service", "rpc.service") == "" && firstAttr(attrs, "server.address", "net.peer.name") != "") {
		external := b.upsertEntity(Entity{ID: ExternalDependencyID(ctx.Service.Environment, target), Type: EntityTypeExternalDependency, Name: NormalizeName(target), Environment: ctx.Service.Environment, Attributes: pickAttrs(attrs, "server.address", "server.port", "net.peer.name", "net.peer.port", "rpc.system"), FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
		b.addRelationship(RelationshipServiceCallsService, ctx.Service.ID, external.ID, evidenceAttrs(span, attrs), observedAt)
		return
	}
	targetService := b.upsertEntity(Entity{ID: ServiceID(ctx.Service.Cluster, ctx.Service.Namespace, target), Type: EntityTypeService, Name: NormalizeName(target), Namespace: ctx.Service.Namespace, Cluster: ctx.Service.Cluster, Environment: ctx.Service.Environment, FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	b.addRelationship(RelationshipServiceCallsService, ctx.Service.ID, targetService.ID, evidenceAttrs(span, attrs), observedAt)
}

func (b *resultBuilder) addSpanHealth(ctx resourceContext, span ptrace.Span, attrs map[string]string, observedAt time.Time) {
	if ctx.Service.ID == "" || span.Kind() != ptrace.SpanKindServer {
		return
	}
	duration := span.EndTimestamp().AsTime().Sub(span.StartTimestamp().AsTime())
	isError := span.Status().Code() == ptrace.StatusCodeError || statusCodeIs5xx(firstAttr(attrs, "http.response.status_code"))
	b.health.addSpan(ctx.Service.ID, observedAt, duration, isError)
}

func (b *resultBuilder) addLogError(serviceID string, observedAt time.Time) {
	b.health.addLogError(serviceID, observedAt)
}

func (b *resultBuilder) addChangeEvent(ctx resourceContext, attrs map[string]string, observedAt time.Time) {
	if ctx.Service.ID == "" {
		return
	}
	version := firstAttr(attrs, "service.version", "container.image.tag")
	revision := firstAttr(attrs, "git.sha", "vcs.revision")
	eventName := strings.ToLower(firstAttr(attrs, "event.name", "event.domain"))
	imageName := firstAttr(attrs, "container.image.name")

	changeType := ""
	switch {
	case strings.Contains(eventName, "config"):
		changeType = ChangeTypeConfigChange
	case strings.Contains(eventName, "restart"):
		changeType = ChangeTypePodRestart
	case strings.TrimSpace(imageName) != "":
		changeType = ChangeTypeImageChange
	case strings.TrimSpace(version) != "" || strings.TrimSpace(revision) != "" || strings.TrimSpace(firstAttr(attrs, "k8s.deployment.name")) != "":
		changeType = ChangeTypeDeployment
	case strings.Contains(eventName, "node") || strings.Contains(eventName, "infra"):
		changeType = ChangeTypeInfraChange
	}
	if changeType == "" {
		return
	}
	event := ChangeEvent{ID: ChangeEventID(changeType, ctx.Service.ID, version, revision), Type: changeType, TargetID: ctx.Service.ID, TargetType: EntityTypeService, Timestamp: observedAt, Source: sourceOTLP, Version: version, Revision: revision, Attributes: pickAttrs(attrs, "event.name", "event.domain", "service.version", "git.sha", "vcs.revision", "container.image.name", "container.image.tag", "k8s.deployment.name")}
	b.events[event.ID] = event
	changeEntity := b.upsertEntity(Entity{ID: event.ID, Type: EntityTypeChangeEvent, Name: NormalizeName(changeType + ":" + ctx.Service.Name), Attributes: event.Attributes, FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP})
	b.addRelationship(RelationshipServiceAffectedByChange, ctx.Service.ID, changeEntity.ID, nil, observedAt)
	b.addRelationship(RelationshipChangeTargetsService, changeEntity.ID, ctx.Service.ID, nil, observedAt)
}

func (b *resultBuilder) upsertEntity(entity Entity) Entity {
	if entity.Attributes == nil {
		entity.Attributes = map[string]string{}
	}
	if existing, ok := b.entities[entity.ID]; ok {
		if entity.FirstSeenAt.Before(existing.FirstSeenAt) {
			existing.FirstSeenAt = entity.FirstSeenAt
		}
		if entity.LastSeenAt.After(existing.LastSeenAt) {
			existing.LastSeenAt = entity.LastSeenAt
		}
		for key, value := range entity.Attributes {
			existing.Attributes[key] = value
		}
		b.entities[entity.ID] = existing
		return existing
	}
	b.entities[entity.ID] = entity
	return entity
}

func (b *resultBuilder) addRelationship(relType, sourceID, targetID string, attrs map[string]string, observedAt time.Time) {
	if sourceID == "" || targetID == "" {
		return
	}
	id := RelationshipID(relType, sourceID, targetID)
	if attrs == nil {
		attrs = map[string]string{}
	}
	if existing, ok := b.relationships[id]; ok {
		if observedAt.Before(existing.FirstSeenAt) {
			existing.FirstSeenAt = observedAt
		}
		if observedAt.After(existing.LastSeenAt) {
			existing.LastSeenAt = observedAt
		}
		existing.EvidenceCount++
		for key, value := range attrs {
			existing.Attributes[key] = value
		}
		b.relationships[id] = existing
		return
	}
	b.relationships[id] = Relationship{ID: id, Type: relType, SourceEntityID: sourceID, TargetEntityID: targetID, Attributes: attrs, FirstSeenAt: observedAt, LastSeenAt: observedAt, Source: sourceOTLP, EvidenceCount: 1}
}

func attrsToMap(attrs pcommon.Map) map[string]string {
	out := make(map[string]string, attrs.Len())
	attrs.Range(func(k string, v pcommon.Value) bool {
		out[k] = v.AsString()
		return true
	})
	return out
}

func metricPointAttrs(metric pmetric.Metric) []map[string]string {
	var out []map[string]string
	add := func(attrs pcommon.Map) { out = append(out, attrsToMap(attrs)) }
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			add(metric.Gauge().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			add(metric.Sum().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			add(metric.Histogram().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			add(metric.ExponentialHistogram().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
			add(metric.Summary().DataPoints().At(i).Attributes())
		}
	}
	return out
}

func mergeAttrs(a, b map[string]string) map[string]string {
	out := make(map[string]string, len(a)+len(b))
	for key, value := range a {
		out[key] = value
	}
	for key, value := range b {
		out[key] = value
	}
	return out
}

func firstAttr(attrs map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(attrs[key]); value != "" {
			return value
		}
	}
	return ""
}

func pickAttrs(attrs map[string]string, keys ...string) map[string]string {
	out := map[string]string{}
	for _, key := range keys {
		if value := strings.TrimSpace(attrs[key]); value != "" {
			out[key] = value
		}
	}
	return out
}

func evidenceAttrs(span ptrace.Span, attrs map[string]string) map[string]string {
	out := pickAttrs(attrs, "http.request.method", "http.route", "rpc.system", "rpc.service", "rpc.method", "db.system.name", "db.system", "db.operation.name", "messaging.system", "messaging.operation.name", "server.address", "net.peer.name", "peer.service")
	out["span.name"] = span.Name()
	out["span.kind"] = span.Kind().String()
	return out
}

func timestampOrNow(timestamp pcommon.Timestamp) time.Time {
	if timestamp == 0 {
		return time.Now().UTC()
	}
	return timestamp.AsTime().UTC()
}

func statusCodeIs5xx(value string) bool {
	code, err := strconv.Atoi(strings.TrimSpace(value))
	return err == nil && code >= 500 && code <= 599
}

func isErrorSeverity(number plog.SeverityNumber, text string, attrs map[string]string) bool {
	if number >= plog.SeverityNumberError {
		return true
	}
	value := strings.ToLower(firstNonEmpty(text, attrs["log.severity"], attrs["severity"]))
	return strings.Contains(value, "error") || strings.Contains(value, "fatal")
}

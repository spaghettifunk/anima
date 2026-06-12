package entityresolver

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestIdentityStabilityAndRouteSanitization(t *testing.T) {
	id1 := ServiceID("Prod", "Checkout", "API")
	id2 := ServiceID(" prod ", "checkout", "api")
	if id1 != id2 {
		t.Fatalf("ServiceID stability = %q/%q", id1, id2)
	}
	got := SanitizeRoute("/users/123/orders/550e8400-e29b-41d4-a716-446655440000/items/abcdefabcdefabcd")
	want := "/users/{id}/orders/{uuid}/items/{hex}"
	if got != want {
		t.Fatalf("SanitizeRoute() = %q, want %q", got, want)
	}
}

func TestResolveFromTracesBuildsTopologyAndHealth(t *testing.T) {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	setAttrs(rs.Resource().Attributes(), map[string]string{
		"service.name":                "checkout",
		"service.namespace":           "shop",
		"service.instance.id":         "instance-a",
		"deployment.environment.name": "prod",
		"k8s.cluster.name":            "cluster-a",
		"k8s.namespace.name":          "shop",
		"k8s.pod.uid":                 "pod-uid",
		"k8s.pod.name":                "checkout-abc",
		"k8s.node.name":               "node-a",
		"k8s.container.name":          "checkout",
	})
	spans := rs.ScopeSpans().AppendEmpty().Spans()
	now := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

	server := spans.AppendEmpty()
	server.SetName("GET /orders/{id}")
	server.SetKind(ptrace.SpanKindServer)
	server.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	server.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(120 * time.Millisecond)))
	setAttrs(server.Attributes(), map[string]string{
		"http.request.method":       "GET",
		"http.route":                "/orders/{id}",
		"http.response.status_code": "503",
	})

	client := spans.AppendEmpty()
	client.SetName("call payments")
	client.SetKind(ptrace.SpanKindClient)
	client.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	client.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(20 * time.Millisecond)))
	setAttrs(client.Attributes(), map[string]string{"peer.service": "payments", "rpc.system": "grpc"})

	db := spans.AppendEmpty()
	db.SetName("SELECT")
	db.SetKind(ptrace.SpanKindClient)
	db.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	db.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(10 * time.Millisecond)))
	setAttrs(db.Attributes(), map[string]string{"db.system.name": "postgresql", "db.name": "orders"})

	msg := spans.AppendEmpty()
	msg.SetName("publish orders")
	msg.SetKind(ptrace.SpanKindProducer)
	msg.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	msg.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(5 * time.Millisecond)))
	setAttrs(msg.Attributes(), map[string]string{"messaging.system": "kafka", "messaging.destination.name": "orders"})

	result := ResolveFromTraces(td)
	assertEntity(t, result, ServiceID("cluster-a", "shop", "checkout"))
	assertEntity(t, result, PodID("cluster-a", "shop", "pod-uid", "checkout-abc"))
	assertRelationship(t, result, RelationshipServiceCallsService)
	assertRelationship(t, result, RelationshipServiceCallsDatabase)
	assertRelationship(t, result, RelationshipServiceProducesTopic)
	assertRelationship(t, result, RelationshipPodRunsOnNode)
	assertRelationship(t, result, RelationshipContainerInPod)
	if len(result.HealthWindows) == 0 {
		t.Fatal("expected health windows")
	}
	if result.HealthWindows[0].RequestCount != 1 || result.HealthWindows[0].ErrorCount != 1 || result.HealthWindows[0].SpanErrorCount != 1 {
		t.Fatalf("health window = %#v, want one request/span error", result.HealthWindows[0])
	}
}

func TestResolveFromLogsBuildsChangeEventAndLogHealth(t *testing.T) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	setAttrs(rl.Resource().Attributes(), map[string]string{
		"service.name":      "checkout",
		"service.namespace": "shop",
		"k8s.cluster.name":  "cluster-a",
	})
	record := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	record.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2026, 6, 12, 12, 1, 0, 0, time.UTC)))
	record.SetSeverityNumber(plog.SeverityNumberError)
	setAttrs(record.Attributes(), map[string]string{
		"service.version":     "v2",
		"git.sha":             "abc123",
		"k8s.deployment.name": "checkout",
	})

	result := ResolveFromLogs(ld)
	if len(result.ChangeEvents) != 1 {
		t.Fatalf("ChangeEvents len = %d, want 1", len(result.ChangeEvents))
	}
	if result.ChangeEvents[0].Type != ChangeTypeDeployment {
		t.Fatalf("ChangeEvent type = %q, want deployment", result.ChangeEvents[0].Type)
	}
	if len(result.HealthWindows) == 0 || result.HealthWindows[0].LogErrorCount != 1 {
		t.Fatalf("health windows = %#v, want log error", result.HealthWindows)
	}
}

func TestUnsupportedRelationshipTypesAreRejected(t *testing.T) {
	if _, err := cypherRelationshipType("BAD_RELATIONSHIP"); err == nil {
		t.Fatal("cypherRelationshipType() error = nil, want unsupported relationship error")
	}
}

func TestDepthIsCapped(t *testing.T) {
	if got := normalizeDepth(0); got != 1 {
		t.Fatalf("normalizeDepth(0) = %d, want 1", got)
	}
	if got := normalizeDepth(99); got != 3 {
		t.Fatalf("normalizeDepth(99) = %d, want 3", got)
	}
}

func assertEntity(t *testing.T, result ResolverResult, id string) {
	t.Helper()
	for _, entity := range result.Entities {
		if entity.ID == id {
			return
		}
	}
	t.Fatalf("entity %q not found in %#v", id, result.Entities)
}

func assertRelationship(t *testing.T, result ResolverResult, relType string) {
	t.Helper()
	for _, relationship := range result.Relationships {
		if relationship.Type == relType {
			return
		}
	}
	t.Fatalf("relationship type %q not found in %#v", relType, result.Relationships)
}

func setAttrs(attrs pcommon.Map, values map[string]string) {
	for key, value := range values {
		attrs.PutStr(key, value)
	}
}

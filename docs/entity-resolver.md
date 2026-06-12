# Entity Resolver

The entity resolver consumes raw OTLP metrics, logs, and traces from Kafka and
builds the current operational topology.

## Inputs

- `collector.raw_metrics_topic`
- `collector.raw_logs_topic`
- `collector.raw_traces_topic`

Payloads may be OTLP JSON or protobuf. The resolver chooses the decoder from
the topic and payload shape.

## Extracted Entities

The first version extracts services, service instances, endpoints, Kubernetes
pods/nodes/containers/namespaces/clusters, environments, regions, availability
zones, deployments, databases, messaging topics, external dependencies, and
change events.

## Relationships

The resolver writes explicit relationships such as service instances, exposed
endpoints, service calls, database calls, messaging produce/consume edges,
Kubernetes placement edges, deployment/environment links, and change-to-service
links.

## Identity

Entity IDs are deterministic. Stable resource identifiers are preferred, such
as `k8s.pod.uid` over pod names. Empty fields become `unknown`, enum-like
fields are lowercased, and endpoint URL paths are sanitized by replacing UUIDs,
numbers, and long hex segments with placeholders.

Examples:

```text
service:<cluster>:<namespace>:<service.name>
pod:<cluster>:<namespace>:<k8s.pod.uid>
endpoint:<service-id>:<method>:<route>
database:<environment>:<db.system>:<db.name>
topic:<environment>:<messaging.system>:<destination.name>
```

## Storage

Memgraph is the source of truth for current topology and relationships.
Postgres stores service health windows and historical change events. Raw OTLP
payloads are not stored by the resolver.

## Run Locally

Start dependencies:

```sh
docker compose up -d
```

Run the resolver:

```sh
mage entityResolver
```

Useful reporter endpoints:

```sh
curl "http://localhost:8600/api/v1/entities?type=service"
curl "http://localhost:8600/api/v1/services/<service-id>/topology?depth=1"
curl "http://localhost:8600/api/v1/services/<service-id>/health?window=5m"
curl "http://localhost:8600/api/v1/changes?target_id=<service-id>"
```

## Limitations

- Dependency edges are intentionally conservative.
- IP-only dependencies become external dependencies.
- Health aggregation is in-process and windowed at `1m` and `5m`.
- Incident correlation is not implemented in this version.

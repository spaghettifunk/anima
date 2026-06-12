package entityresolver

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	uuidSegmentRE    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	longHexSegmentRE = regexp.MustCompile(`(?i)^[0-9a-f]{12,}$`)
	numberSegmentRE  = regexp.MustCompile(`^[0-9]+$`)
	spaceRE          = regexp.MustCompile(`\s+`)
)

func NormalizeField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func NormalizeEnum(value string) string {
	return strings.ToLower(NormalizeField(value))
}

func NormalizeName(value string) string {
	value = strings.TrimSpace(value)
	value = spaceRE.ReplaceAllString(value, "-")
	if value == "" {
		return "unknown"
	}
	return value
}

func ServiceID(cluster, namespace, serviceName string) string {
	return "service:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(namespace) + ":" + NormalizeEnum(serviceName)
}

func ServiceInstanceID(serviceID, instanceID string) string {
	return "service_instance:" + NormalizeEnum(serviceID) + ":" + hashPart(NormalizeField(instanceID))
}

func PodID(cluster, namespace, podUID, podName string) string {
	stable := firstNonEmpty(podUID, podName)
	return "pod:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(namespace) + ":" + NormalizeEnum(stable)
}

func NodeID(cluster, nodeName string) string {
	return "node:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(nodeName)
}

func ContainerID(cluster, namespace, podUID, podName, containerID, containerName string) string {
	stable := firstNonEmpty(containerID, containerName)
	return "container:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(namespace) + ":" + NormalizeEnum(firstNonEmpty(podUID, podName)) + ":" + hashPart(NormalizeField(stable))
}

func HostID(cluster, hostName string) string {
	return "host:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(hostName)
}

func NamespaceID(cluster, namespace string) string {
	return "namespace:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(namespace)
}

func ClusterID(cluster string) string {
	return "cluster:" + NormalizeEnum(cluster)
}

func EnvironmentID(environment string) string {
	return "environment:" + NormalizeEnum(environment)
}

func RegionID(provider, region string) string {
	return "region:" + NormalizeEnum(provider) + ":" + NormalizeEnum(region)
}

func AvailabilityZoneID(provider, region, zone string) string {
	return "availability_zone:" + NormalizeEnum(provider) + ":" + NormalizeEnum(region) + ":" + NormalizeEnum(zone)
}

func DeploymentID(cluster, namespace, deployment string) string {
	return "deployment:" + NormalizeEnum(cluster) + ":" + NormalizeEnum(namespace) + ":" + NormalizeEnum(deployment)
}

func EndpointID(serviceID, method, route string) string {
	return "endpoint:" + NormalizeEnum(serviceID) + ":" + NormalizeEnum(method) + ":" + NormalizeEnum(SanitizeRoute(route))
}

func DatabaseID(environment, system, name string) string {
	return "database:" + NormalizeEnum(environment) + ":" + NormalizeEnum(system) + ":" + NormalizeEnum(name)
}

func MessagingTopicID(environment, system, destination string) string {
	return "topic:" + NormalizeEnum(environment) + ":" + NormalizeEnum(system) + ":" + NormalizeEnum(destination)
}

func ExternalDependencyID(environment, dependency string) string {
	return "external_dependency:" + NormalizeEnum(environment) + ":" + NormalizeEnum(dependency)
}

func ChangeEventID(changeType, targetID, version, revision string) string {
	parts := strings.Join([]string{NormalizeEnum(changeType), NormalizeEnum(targetID), NormalizeField(version), NormalizeField(revision)}, "\x00")
	return "change:" + hashPart(parts)
}

func RelationshipID(relType, sourceID, targetID string) string {
	return "relationship:" + NormalizeEnum(relType) + ":" + NormalizeEnum(sourceID) + ":" + NormalizeEnum(targetID)
}

func SanitizeRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return "unknown"
	}
	if parsed, err := url.Parse(route); err == nil && parsed.Path != "" {
		route = parsed.Path
	}
	parts := strings.Split(route, "/")
	for i, part := range parts {
		switch {
		case part == "":
		case uuidSegmentRE.MatchString(part):
			parts[i] = "{uuid}"
		case longHexSegmentRE.MatchString(part):
			parts[i] = "{hex}"
		case numberSegmentRE.MatchString(part):
			parts[i] = "{id}"
		}
	}
	out := strings.Join(parts, "/")
	if out == "" {
		return "/"
	}
	return out
}

func IsIP(value string) bool {
	host := strings.TrimSpace(value)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return net.ParseIP(host) != nil
}

func hashPart(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

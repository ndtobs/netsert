package assertion

import (
	"regexp"
	"strings"
)

// PathPrefix defines a short path prefix and its expansion
type PathPrefix struct {
	// Pattern to match (e.g., "bgp[")
	Pattern string
	// Regex for extracting the instance/key
	Regex *regexp.Regexp
	// Template for expansion, use {instance} for the captured value
	Template string
}

// pathPrefixes defines the known short path prefixes and their expansions
var pathPrefixes = []PathPrefix{
	{
		// bgp[<network-instance>]/... -> /network-instances/network-instance[name=<ni>]/protocols/protocol[identifier=BGP][name=BGP]/bgp/...
		Pattern:  "bgp[",
		Regex:    regexp.MustCompile(`^bgp\[([^\]]+)\]/(.*)$`),
		Template: "/network-instances/network-instance[name={instance}]/protocols/protocol[identifier=BGP][name=BGP]/bgp/{rest}",
	},
	{
		// ospf[<network-instance>]/... -> /network-instances/network-instance[name=<ni>]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/...
		Pattern:  "ospf[",
		Regex:    regexp.MustCompile(`^ospf\[([^\]]+)\]/(.*)$`),
		Template: "/network-instances/network-instance[name={instance}]/protocols/protocol[identifier=OSPF][name=OSPF]/ospf/{rest}",
	},
	{
		// isis[<network-instance>]/... -> /network-instances/network-instance[name=<ni>]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/...
		Pattern:  "isis[",
		Regex:    regexp.MustCompile(`^isis\[([^\]]+)\]/(.*)$`),
		Template: "/network-instances/network-instance[name={instance}]/protocols/protocol[identifier=ISIS][name=ISIS]/isis/{rest}",
	},
	{
		// interface[<name>]/... -> /interfaces/interface[name=<name>]/...
		Pattern:  "interface[",
		Regex:    regexp.MustCompile(`^interface\[([^\]]+)\]/(.*)$`),
		Template: "/interfaces/interface[name={instance}]/{rest}",
	},
	{
		// lldp/... -> /lldp/...
		Pattern:  "lldp/",
		Regex:    regexp.MustCompile(`^lldp/(.*)$`),
		Template: "/lldp/{instance}",
	},
	{
		// system/... -> /system/...
		Pattern:  "system/",
		Regex:    regexp.MustCompile(`^system/(.*)$`),
		Template: "/system/{instance}",
	},
	{
		// network-instance[<name>]/... -> /network-instances/network-instance[name=<name>]/...
		Pattern:  "network-instance[",
		Regex:    regexp.MustCompile(`^network-instance\[([^\]]+)\]/(.*)$`),
		Template: "/network-instances/network-instance[name={instance}]/{rest}",
	},
}

// ExpandPath expands a short path to its full OpenConfig form.
// Paths starting with "/" are returned unchanged (already absolute).
// Short paths are matched against known prefixes and expanded.
func ExpandPath(path string) string {
	// Absolute paths pass through unchanged
	if strings.HasPrefix(path, "/") {
		return path
	}

	// Try each prefix
	for _, prefix := range pathPrefixes {
		if strings.HasPrefix(path, prefix.Pattern) {
			matches := prefix.Regex.FindStringSubmatch(path)
			if matches != nil {
				result := prefix.Template
				if len(matches) > 1 {
					result = strings.Replace(result, "{instance}", matches[1], 1)
				}
				if len(matches) > 2 {
					result = strings.Replace(result, "{rest}", matches[2], 1)
				} else {
					result = strings.Replace(result, "{rest}", "", 1)
				}
				return result
			}
		}
	}

	// No prefix matched - return with leading slash (assume root-relative)
	return "/" + path
}

// CompactPath converts a full OpenConfig path to its short form if possible.
// This is the inverse of ExpandPath.
func CompactPath(path string) string {
	// Try to match against expanded templates
	
	// BGP
	bgpRegex := regexp.MustCompile(`^/network-instances/network-instance\[name=([^\]]+)\]/protocols/protocol\[identifier=BGP\]\[name=BGP\]/bgp/(.*)$`)
	if matches := bgpRegex.FindStringSubmatch(path); matches != nil {
		return "bgp[" + matches[1] + "]/" + matches[2]
	}

	// OSPF
	ospfRegex := regexp.MustCompile(`^/network-instances/network-instance\[name=([^\]]+)\]/protocols/protocol\[identifier=OSPF\]\[name=OSPF\]/ospf/(.*)$`)
	if matches := ospfRegex.FindStringSubmatch(path); matches != nil {
		return "ospf[" + matches[1] + "]/" + matches[2]
	}

	// ISIS
	isisRegex := regexp.MustCompile(`^/network-instances/network-instance\[name=([^\]]+)\]/protocols/protocol\[identifier=ISIS\]\[name=ISIS\]/isis/(.*)$`)
	if matches := isisRegex.FindStringSubmatch(path); matches != nil {
		return "isis[" + matches[1] + "]/" + matches[2]
	}

	// Interface
	ifaceRegex := regexp.MustCompile(`^/interfaces/interface\[name=([^\]]+)\]/(.*)$`)
	if matches := ifaceRegex.FindStringSubmatch(path); matches != nil {
		return "interface[" + matches[1] + "]/" + matches[2]
	}

	// LLDP
	if strings.HasPrefix(path, "/lldp/") {
		return "lldp/" + strings.TrimPrefix(path, "/lldp/")
	}

	// System
	if strings.HasPrefix(path, "/system/") {
		return "system/" + strings.TrimPrefix(path, "/system/")
	}

	// Network instance (generic)
	niRegex := regexp.MustCompile(`^/network-instances/network-instance\[name=([^\]]+)\]/(.*)$`)
	if matches := niRegex.FindStringSubmatch(path); matches != nil {
		return "network-instance[" + matches[1] + "]/" + matches[2]
	}

	// No compaction possible
	return path
}

// IsShortPath returns true if the path is in short form (doesn't start with /)
func IsShortPath(path string) bool {
	return !strings.HasPrefix(path, "/")
}

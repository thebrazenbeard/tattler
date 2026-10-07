package protocol

import (
	"fmt"
	"strings"

	"github.com/thebrazenbeard/tattler/internal/model"
)

type portRule struct {
	name  string
	layer string
}

var tcpRules = map[uint16][]portRule{
	22:   {{name: "ssh", layer: "application"}},
	53:   {{name: "dns", layer: "application"}},
	80:   {{name: "http", layer: "application"}},
	443:  {{name: "tls", layer: "security"}, {name: "https", layer: "application"}},
	445:  {{name: "smb", layer: "application"}},
	853:  {{name: "tls", layer: "security"}, {name: "dot", layer: "application"}},
	2049: {{name: "nfs", layer: "application"}},
	8080: {{name: "http", layer: "application"}},
	8443: {{name: "tls", layer: "security"}, {name: "https", layer: "application"}},
}

var udpRules = map[uint16][]portRule{
	53:   {{name: "dns", layer: "application"}},
	67:   {{name: "dhcp", layer: "application"}},
	68:   {{name: "dhcp", layer: "application"}},
	123:  {{name: "ntp", layer: "application"}},
	161:  {{name: "snmp", layer: "application"}},
	162:  {{name: "snmp-trap", layer: "application"}},
	443:  {{name: "quic", layer: "transport_overlay"}},
	853:  {{name: "quic", layer: "transport_overlay"}, {name: "doq", layer: "application"}},
	1900: {{name: "ssdp", layer: "application"}},
	5353: {{name: "mdns", layer: "application"}},
}

func Classify(c model.Connection) []model.ProtocolEvidence {
	proto := strings.ToLower(c.Protocol)
	switch {
	case strings.HasPrefix(proto, "tcp"):
		return classifyTCP(c)
	case strings.HasPrefix(proto, "udp"):
		return classifyUDP(c)
	default:
		return nil
	}
}

func classifyTCP(c model.Connection) []model.ProtocolEvidence {
	port, role := servicePort(c)
	if port == 0 {
		return nil
	}
	rules := tcpRules[port]
	return evidenceFor(rules, port, role)
}

func classifyUDP(c model.Connection) []model.ProtocolEvidence {
	if c.Kind == model.ObservationUDPEndpoint {
		// A bound endpoint without a kernel-reported peer does not establish a
		// remote application protocol. Known local service ports are useful
		// only as a listener/service hint.
		port := c.Local.Port()
		rules := udpRules[port]
		if len(rules) == 0 || port == 0 {
			return nil
		}
		return evidenceFor(rules, port, "local")
	}
	if c.Kind != model.ObservationUDPFlow {
		return nil
	}
	if c.Direction == "inbound" {
		if rules := udpRules[c.Local.Port()]; len(rules) > 0 {
			return evidenceFor(rules, c.Local.Port(), "local")
		}
	}
	if c.Remote.Port() != 0 && c.Remote.Addr().IsValid() && !c.Remote.Addr().IsUnspecified() {
		if rules := udpRules[c.Remote.Port()]; len(rules) > 0 {
			return evidenceFor(rules, c.Remote.Port(), "remote")
		}
	}
	if c.Local.Port() != 0 {
		return evidenceFor(udpRules[c.Local.Port()], c.Local.Port(), "local")
	}
	return nil
}

func servicePort(c model.Connection) (uint16, string) {
	switch c.Kind {
	case model.ObservationTCPListener:
		return c.Local.Port(), "local"
	case model.ObservationTCPSession:
		if c.Direction == "inbound" {
			return c.Local.Port(), "local"
		}
		if c.Remote.Port() != 0 {
			return c.Remote.Port(), "remote"
		}
		return c.Local.Port(), "local"
	default:
		return 0, ""
	}
}

func evidenceFor(rules []portRule, port uint16, role string) []model.ProtocolEvidence {
	if len(rules) == 0 {
		return nil
	}
	out := make([]model.ProtocolEvidence, 0, len(rules))
	for _, rule := range rules {
		out = append(out, model.ProtocolEvidence{
			Name:       rule.name,
			Layer:      rule.layer,
			Confidence: model.ProtocolConfidenceHeuristic,
			Source:     model.ProtocolSourceWellKnownPort,
			Reason:     fmt.Sprintf("%s port %d", role, port),
		})
	}
	return out
}

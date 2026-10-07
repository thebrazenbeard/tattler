package protocol

import (
	"net/netip"
	"testing"

	"github.com/thebrazenbeard/tattler/internal/model"
)

func names(v []model.ProtocolEvidence) map[string]model.ProtocolEvidence {
	out := make(map[string]model.ProtocolEvidence, len(v))
	for _, item := range v {
		out[item.Name] = item
	}
	return out
}

func TestClassifyTCPHTTPAndHTTPSAsHeuristics(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		want    []string
		notWant []string
	}{
		{name: "http", remote: "1.2.3.4:80", want: []string{"http"}},
		{name: "https", remote: "1.2.3.4:443", want: []string{"tls", "https"}, notWant: []string{"http3", "websocket", "grpc", "webhook"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := model.Connection{Kind: model.ObservationTCPSession, Protocol: "tcp", Direction: "outbound", Local: netip.MustParseAddrPort("10.0.0.2:50000"), Remote: netip.MustParseAddrPort(tc.remote)}
			got := names(Classify(c))
			for _, name := range tc.want {
				ev, ok := got[name]
				if !ok {
					t.Fatalf("missing %s in %+v", name, got)
				}
				if ev.Confidence != model.ProtocolConfidenceHeuristic || ev.Source != model.ProtocolSourceWellKnownPort {
					t.Fatalf("%s evidence=%+v", name, ev)
				}
			}
			for _, name := range tc.notWant {
				if _, ok := got[name]; ok {
					t.Fatalf("unexpected %s in %+v", name, got)
				}
			}
		})
	}
}

func TestClassifyUDPFlowQUICWithoutInventingHTTP3(t *testing.T) {
	c := model.Connection{Kind: model.ObservationUDPFlow, Protocol: "udp", Local: netip.MustParseAddrPort("10.0.0.2:53000"), Remote: netip.MustParseAddrPort("1.2.3.4:443")}
	got := names(Classify(c))
	if ev, ok := got["quic"]; !ok || ev.Confidence != model.ProtocolConfidenceHeuristic {
		t.Fatalf("quic=%+v ok=%v", ev, ok)
	}
	if _, ok := got["http3"]; ok {
		t.Fatal("UDP/443 must not imply HTTP/3")
	}
}

func TestClassifyDNSDoTDoQAndNASProtocols(t *testing.T) {
	cases := []struct {
		proto, remote string
		kind          string
		want          []string
	}{
		{"udp", "8.8.8.8:53", model.ObservationUDPFlow, []string{"dns"}},
		{"tcp", "1.1.1.1:853", model.ObservationTCPSession, []string{"tls", "dot"}},
		{"udp", "1.1.1.1:853", model.ObservationUDPFlow, []string{"quic", "doq"}},
		{"tcp", "10.0.0.20:445", model.ObservationTCPSession, []string{"smb"}},
		{"tcp", "10.0.0.20:2049", model.ObservationTCPSession, []string{"nfs"}},
	}
	for _, tc := range cases {
		c := model.Connection{Kind: tc.kind, Protocol: tc.proto, Direction: "outbound", Local: netip.MustParseAddrPort("10.0.0.2:50000"), Remote: netip.MustParseAddrPort(tc.remote)}
		got := names(Classify(c))
		for _, want := range tc.want {
			if _, ok := got[want]; !ok {
				t.Fatalf("%s/%s missing %s: %+v", tc.proto, tc.remote, want, got)
			}
		}
	}
}

func TestUnconnectedUDPEndpointDoesNotInventRemoteApplicationProtocol(t *testing.T) {
	c := model.Connection{Kind: model.ObservationUDPEndpoint, Protocol: "udp", Local: netip.MustParseAddrPort("0.0.0.0:53000"), Remote: netip.MustParseAddrPort("0.0.0.0:0")}
	if got := Classify(c); len(got) != 0 {
		t.Fatalf("got=%+v, want no hints", got)
	}
}

func TestClassifyUsefulLocalUDPServicesAndQUICServer(t *testing.T) {
	mdns := model.Connection{
		Kind: model.ObservationUDPEndpoint, Protocol: "udp",
		Local:  netip.MustParseAddrPort("0.0.0.0:5353"),
		Remote: netip.MustParseAddrPort("0.0.0.0:0"),
	}
	if _, ok := names(Classify(mdns))["mdns"]; !ok {
		t.Fatalf("mDNS endpoint not classified: %+v", Classify(mdns))
	}

	quicServer := model.Connection{
		Kind: model.ObservationUDPFlow, Protocol: "udp",
		Local:  netip.MustParseAddrPort("10.0.0.2:443"),
		Remote: netip.MustParseAddrPort("10.0.0.8:53000"),
	}
	if _, ok := names(Classify(quicServer))["quic"]; !ok {
		t.Fatalf("local UDP/443 flow not classified as QUIC heuristic: %+v", Classify(quicServer))
	}
}

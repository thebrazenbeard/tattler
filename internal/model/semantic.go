package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	SemanticHTTPTransaction  = "http_transaction"
	SemanticWebhookDelivery  = "webhook_delivery"
	SemanticWebSocketSession = "websocket_session"
	SemanticGRPCRPC          = "grpc_rpc"
)

type SemanticObservation struct {
	SchemaVersion  int              `json:"schema_version"`
	ID             string           `json:"event_id"`
	ObservedAt     time.Time        `json:"observed_at"`
	Kind           string           `json:"kind"`
	Protocol       string           `json:"protocol"`
	Direction      string           `json:"direction,omitempty"`
	Peer           string           `json:"peer,omitempty"`
	Method         string           `json:"method,omitempty"`
	Route          string           `json:"route,omitempty"`
	Status         int              `json:"status,omitempty"`
	DurationMS     int64            `json:"duration_ms,omitempty"`
	BytesIn        int64            `json:"bytes_in,omitempty"`
	BytesOut       int64            `json:"bytes_out,omitempty"`
	Reporter       string           `json:"reporter,omitempty"`
	Provider       string           `json:"provider,omitempty"`
	EventType      string           `json:"event_type,omitempty"`
	DeliveryID     string           `json:"delivery_id,omitempty"`
	Retry          int              `json:"retry,omitempty"`
	SignatureValid *bool            `json:"signature_valid,omitempty"`
	ErrorType      string           `json:"error_type,omitempty"`
	Evidence       ProtocolEvidence `json:"evidence"`
}

func NewSemanticObservation(at time.Time, in SemanticObservation) SemanticObservation {
	at = at.UTC()
	in.SchemaVersion = 1
	in.ObservedAt = at
	in.Evidence = ProtocolEvidence{
		Name:       in.Protocol,
		Layer:      "application",
		Confidence: ProtocolConfidenceReported,
		Source:     ProtocolSourceReported,
		Reason:     "reported through loopback semantic API",
	}
	raw := fmt.Sprintf(
		"semantic-v1|%s|%s|%s|%s|%s|%s|%d|%d|%s|%s|%s|%s|%s|%s|%d",
		at.Format(time.RFC3339Nano), in.Kind, in.Protocol, in.Direction,
		in.Peer, in.Method, in.Status, in.DurationMS, in.Route,
		in.Reporter, in.Provider, in.EventType, in.DeliveryID, in.ErrorType, in.Retry,
	)
	sum := sha256.Sum256([]byte(raw))
	in.ID = hex.EncodeToString(sum[:])
	return in
}

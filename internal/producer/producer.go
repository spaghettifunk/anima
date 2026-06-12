package producer

import "context"

// PacketTelemetry defines the generic payload to transmit to the producer
type PacketTelemetry struct {
	// Name can be either a table or a topic
	Name string
	// Message is the byte representation of the JSON UDP Packet
	Message []byte
	// Metadata is used to pass extra information to the packet
	Metadata map[string]string
}

// Producer is the interface for the different type of producer system
type Producer interface {
	Write(PacketTelemetry) error
	Health(context.Context) error
	Shutdown() error
	// this is used for the go-stream package
	In() chan<- any
}

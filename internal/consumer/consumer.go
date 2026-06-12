package consumer

// Consumer is the interface for the different type of consumer system
type Consumer interface {
	Initialize(topics []string, ready chan bool, handler interface{}) error
	GetMessages() error
	ToggleConsumptionFlow(isPaused *bool)
	Shutdown() error
}

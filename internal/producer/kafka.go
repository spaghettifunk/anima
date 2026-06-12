package producer

import (
	"context"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

var (
	ackMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "anima",
		Subsystem: "adaptive_telemetry",
		Name:      "kafka_ack_counter",
		Help:      "Number of Acked messages sent to Kafka.",
	})
	errorMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "anima",
		Subsystem: "adaptive_telemetry",
		Name:      "kafka_errors_counter",
		Help:      "Number of error messages received from Kafka.",
	})
)

// Client is the object for sending the logs to Kafa
type KafkaProducer struct {
	Producer       sarama.AsyncProducer
	internalConfig *sarama.Config
	brokers        []string
	// this is used for implementing the interface for go-streams pkg
	in chan any
}

// SASLMechanism functional option for the kafka configuration
// Values accepted: "OAUTHBEARER", "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512", "GSSAPI"
func SASLMechanism(m string) func(*sarama.Config) {
	return func(cfg *sarama.Config) {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.Mechanism = sarama.SASLMechanism(m)
	}
}

// Credentials functional option for the kafka configuration
// For functional options see:
// https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis
func Credentials(username, password string) func(*sarama.Config) {
	return func(cfg *sarama.Config) {
		cfg.Net.SASL.User = username
		cfg.Net.SASL.Password = password
	}
}

// New create a new object for interacting with Kafka
func NewKafkaProducer(brokers string, options ...func(*sarama.Config)) (KafkaProducer, error) {
	bs := strings.Split(brokers, ",")

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Errors = true
	cfg.Producer.Return.Successes = true

	for _, opt := range options {
		opt(cfg)
	}

	// register prometheus metrics
	prometheus.MustRegister(ackMessages)
	prometheus.MustRegister(errorMessages)

	producer, err := sarama.NewAsyncProducer(bs, cfg)
	if err != nil {
		return KafkaProducer{}, err
	}

	return KafkaProducer{
		Producer:       producer,
		internalConfig: cfg,
		brokers:        bs,
		in:             make(chan any),
	}, nil
}

func (k KafkaProducer) Write(pl PacketTelemetry) error {
	if pl.Message == nil {
		log.Warn().Msgf("packet was nil: this indicates that the marshalling phase failed for topic `%s`", pl.Name)
		return nil
	}
	// store metadata as sarama headers
	headers := []sarama.RecordHeader{}
	for k, v := range pl.Metadata {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte(k),
			Value: []byte(v),
		})
	}
	// send to kafka topic
	k.Producer.Input() <- &sarama.ProducerMessage{
		Topic:     pl.Name,
		Value:     sarama.StringEncoder(pl.Message),
		Timestamp: time.Now(),
		Headers:   headers,
	}
	return nil
}

func (k KafkaProducer) Health(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		client, err := sarama.NewClient(k.brokers, k.internalConfig)
		if err != nil {
			errCh <- err
			return
		}
		errCh <- client.Close()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ProcessResponse grabs results and errors from the producer asynchronously
func (k KafkaProducer) ProcessResponse() {
	for {
		select {
		case <-k.Producer.Successes():
			ackMessages.Inc()
			log.Debug().Msgf("message produced")
		case err := <-k.Producer.Errors():
			errorMessages.Inc()
			log.Error().Err(err).Msgf("failed to produce message")
		}
	}
}

// Shutdown closes the producer object
func (k KafkaProducer) Shutdown() error {
	return k.Producer.Close()
}

// In returns an input channel for receiving data
func (k KafkaProducer) In() chan<- any {
	return k.in
}

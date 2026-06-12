package consumer

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/rs/zerolog/log"
)

var (
	// FlushIntervalInSec is the amount of time before executing the Flush operation in case the buffer is not full
	FlushIntervalInSec = 10
)

// Client is the object for sending the logs to Kafa
type KafkaConsumer struct {
	Consumer     sarama.ConsumerGroup
	clusterAdmin sarama.ClusterAdmin

	// below are used for consuming messages
	wg     *sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	// this is used for implementing the go-streams interface pkg
	out chan any
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
func Credentials(username, password string) func(*sarama.Config) {
	return func(cfg *sarama.Config) {
		cfg.Net.SASL.User = username
		cfg.Net.SASL.Password = password
	}
}

// Assignor functional option for the kafka rebalance group strategy
// Values accepted: "sticky", "range", "roundrobin" (default)
func Assignor(strategy string) func(*sarama.Config) {
	return func(cfg *sarama.Config) {
		switch strategy {
		case "sticky":
			cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}
		case "range":
			cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
		case "roundrobin":
		default:
			cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
		}
	}
}

// InitalOffset functional option for the Kafka consumer offset
// Values accepted: "oldest", "newest" (default)
func InitialOffset(offset string) func(*sarama.Config) {
	return func(cfg *sarama.Config) {
		switch offset {
		case "oldest":
			cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
		case "newest":
		default:
			cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
		}
	}
}

func NewKafkaConsumer(brokers string, consumerID string, ctx context.Context, cancel context.CancelFunc, options ...func(*sarama.Config)) (KafkaConsumer, error) {
	bs := strings.Split(brokers, ",")

	// configuration with default values
	cfg := sarama.NewConfig()
	cfg.Version = sarama.MaxVersion // latest version available
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	cfg.Consumer.Offsets.AutoCommit.Enable = false // manually committing to avoid potential duplicates
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	cfg.Consumer.Return.Errors = true

	for _, opt := range options {
		opt(cfg)
	}

	admin, err := sarama.NewClusterAdmin(bs, cfg)
	if err != nil {
		return KafkaConsumer{}, err
	}

	consumer, err := sarama.NewConsumerGroup(bs, consumerID, cfg)
	if err != nil {
		return KafkaConsumer{}, err
	}

	return KafkaConsumer{
		Consumer:     consumer,
		clusterAdmin: admin,
		wg:           &sync.WaitGroup{},
		ctx:          ctx,
		cancel:       cancel,
		out:          make(chan any),
	}, nil
}

func (kc KafkaConsumer) Initialize(topics []string, ready chan bool, handler interface{}) error {
	hnl, ok := handler.(sarama.ConsumerGroupHandler)
	if !ok {
		log.Panic().Msg("handler is not a compatible `sarama.ConsumerGroupHandler`")
	}

	kc.wg.Add(1)
	wg := sync.WaitGroup{}
	wg.Go(func() {
		defer kc.wg.Done()
		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims
			if err := kc.Consumer.Consume(kc.ctx, topics, hnl); err != nil {
				log.Panic().Msgf("Error from consumer: %v", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if kc.ctx.Err() != nil {
				return
			}
		}
	})

	<-ready // Await till the consumer has been set up
	log.Info().Msg("kafka consumer up and running...")

	return nil
}

// GetMessages needs to be called from a goroutine
func (kc KafkaConsumer) GetMessages() error {
	keepRunning := true
	consumptionIsPaused := false

	// TODO: this should come from an API
	sigusr1 := make(chan os.Signal, 1)
	signal.Notify(sigusr1, syscall.SIGUSR1)

	// TODO: this should come from an API
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)

	for keepRunning {
		select {
		case <-kc.ctx.Done():
			log.Debug().Msg("terminating: context cancelled")
			keepRunning = false
		case <-sigterm:
			log.Debug().Msg("terminating: via signal")
			keepRunning = false
		case <-sigusr1:
			kc.ToggleConsumptionFlow(&consumptionIsPaused)
		}
	}

	kc.cancel()
	kc.wg.Wait()

	return nil
}

func (kc KafkaConsumer) ToggleConsumptionFlow(isPaused *bool) {
	if !*isPaused {
		kc.Consumer.ResumeAll()
		log.Debug().Msg("resuming consumption")
	} else {
		kc.Consumer.PauseAll()
		log.Debug().Msg("pausing consumption")
	}
	*isPaused = !*isPaused
}

// Shutdown the consumer object
func (kc KafkaConsumer) Shutdown() error {
	if err := kc.clusterAdmin.Close(); err != nil {
		return err
	}
	// close the go-streams
	close(kc.out)

	return kc.Consumer.Close()
}

package emitter

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/care/orion/internal/config"
	"github.com/care/orion/internal/types"
)

// MQTTEmitter publishes inferences to MQTT broker
type MQTTEmitter struct {
	cfg    *config.Config
	Client mqtt.Client // Exported for control plane

	mu            sync.RWMutex
	published     map[string]uint64 // count per topic
	errors        uint64
	connected     bool
}

// NewMQTTEmitter creates a new MQTT emitter
func NewMQTTEmitter(cfg *config.Config) *MQTTEmitter {
	return &MQTTEmitter{
		cfg:       cfg,
		published: make(map[string]uint64),
	}
}

// Connect establishes connection to MQTT broker
func (e *MQTTEmitter) Connect(ctx context.Context) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", e.cfg.MQTT.Broker))
	opts.SetClientID(e.cfg.InstanceID)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(2 * time.Second)
	opts.SetMaxReconnectInterval(30 * time.Second)

	// Connection handlers
	opts.OnConnect = func(c mqtt.Client) {
		e.mu.Lock()
		e.connected = true
		e.mu.Unlock()
		slog.Info("mqtt connection established",
			"broker", e.cfg.MQTT.Broker,
			"client_id", e.cfg.InstanceID,
			"auto_reconnect", "enabled")
	}

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		e.mu.Lock()
		e.connected = false
		e.mu.Unlock()
		slog.Warn("mqtt connection lost, will auto-reconnect",
			"error", err,
			"broker", e.cfg.MQTT.Broker,
			"max_retry_interval", "30s",
			"action", "waiting for automatic reconnection")
	}

	e.Client = mqtt.NewClient(opts)

	slog.Info("connecting to mqtt broker", "broker", e.cfg.MQTT.Broker)

	token := e.Client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("mqtt connection timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("mqtt connection failed: %w", err)
	}

	e.mu.Lock()
	e.connected = true
	e.mu.Unlock()

	return nil
}

// Publish publishes an inference to the appropriate MQTT topic
func (e *MQTTEmitter) Publish(inference types.Inference) error {
	if !e.isConnected() {
		e.mu.Lock()
		e.errors++
		e.mu.Unlock()
		return fmt.Errorf("mqtt not connected")
	}

	// Build topic: care/inferences/{instance_id}/{type}
	topic := fmt.Sprintf("%s/%s", e.cfg.MQTT.Topics.Inferences, inference.Type())

	// Get QoS for this inference type
	qos := e.getQoS(inference.Type())

	// Marshal inference to JSON
	payload, err := inference.ToJSON()
	if err != nil {
		e.mu.Lock()
		e.errors++
		e.mu.Unlock()
		return fmt.Errorf("failed to marshal inference: %w", err)
	}

	// Publish
	token := e.Client.Publish(topic, qos, false, payload)
	if !token.WaitTimeout(2 * time.Second) {
		e.mu.Lock()
		e.errors++
		e.mu.Unlock()
		return fmt.Errorf("publish timeout")
	}
	if err := token.Error(); err != nil {
		e.mu.Lock()
		e.errors++
		e.mu.Unlock()
		return fmt.Errorf("publish failed: %w", err)
	}

	// Update stats
	e.mu.Lock()
	e.published[topic]++
	e.mu.Unlock()

	slog.Debug("inference published",
		"topic", topic,
		"qos", qos,
		"size", len(payload),
	)

	return nil
}

// PublishHealth publishes a health message
func (e *MQTTEmitter) PublishHealth(payload []byte) error {
	if !e.isConnected() {
		return fmt.Errorf("mqtt not connected")
	}

	topic := e.cfg.MQTT.Topics.Health
	qos := e.cfg.MQTT.QoS["health"]

	token := e.Client.Publish(topic, qos, false, payload)
	if !token.WaitTimeout(2 * time.Second) {
		return fmt.Errorf("publish timeout")
	}

	return token.Error()
}

// Disconnect closes the MQTT connection
func (e *MQTTEmitter) Disconnect() error {
	if e.Client != nil && e.Client.IsConnected() {
		e.Client.Disconnect(250) // 250ms grace period
		slog.Info("mqtt disconnected")
	}

	e.mu.Lock()
	e.connected = false
	e.mu.Unlock()

	return nil
}

// Stats returns emitter statistics
func (e *MQTTEmitter) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	published := make(map[string]uint64)
	for k, v := range e.published {
		published[k] = v
	}

	return Stats{
		Connected: e.connected,
		Published: published,
		Errors:    e.errors,
	}
}

// Stats contains emitter statistics
type Stats struct {
	Connected bool
	Published map[string]uint64
	Errors    uint64
}

// isConnected returns connection status
func (e *MQTTEmitter) isConnected() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.connected
}

// getQoS returns the QoS level for a given inference type
func (e *MQTTEmitter) getQoS(inferenceType string) byte {
	if qos, ok := e.cfg.MQTT.QoS[inferenceType]; ok {
		return qos
	}
	return 0 // default QoS 0
}

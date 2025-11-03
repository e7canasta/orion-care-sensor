package control

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/care/orion/internal/config"
)

// Command represents a control plane command
type Command struct {
	Command string                 `json:"command"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// Response represents a command response
type Response struct {
	CommandAck string                 `json:"command_ack"`
	Status     string                 `json:"status"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Timestamp  string                 `json:"timestamp"`
}

// Handler handles control plane commands
type Handler struct {
	cfg      *config.Config
	client   mqtt.Client
	commands chan Command

	mu        sync.RWMutex
	isPaused  bool
	callbacks CommandCallbacks
}

// CommandCallbacks contains callback functions for commands
type CommandCallbacks struct {
	OnGetStatus        func() map[string]interface{}
	OnPause            func() error
	OnResume           func() error
	OnUpdateConfig     func(map[string]interface{}) error
	OnShutdown         func() error
	OnSetInferenceRate func(float64) error
	OnSetModelSize     func(string) error
	// Broadcast commands
	OnStartBroadcast      func(rtmpURL, filter, overlayMode string) error
	OnStopBroadcast       func() error
	OnSetBroadcastFilter  func(filter string) error
	OnSetBroadcastOverlay func(overlayMode string) error
	OnRestartBroadcast    func() error
	OnGetBroadcastStatus  func() map[string]interface{}
	// ROI Attention commands
	OnSetAttentionROIs   func(rois []map[string]interface{}) error
	OnClearAttentionROIs func() error
	OnGetAttentionROIs   func() map[string]interface{}
	// Auto-focus strategy commands
	OnSetAutoFocusStrategy  func(params map[string]interface{}) error
	OnGetAutoFocusStrategy  func() map[string]interface{}
	OnEnableAutoFocus       func() error
	OnDisableAutoFocus      func() error
	OnClearAutoFocusHistory func() error
	OnGetAutoFocusStats     func() map[string]interface{}
}

// NewHandler creates a new control plane handler
func NewHandler(cfg *config.Config, client mqtt.Client, callbacks CommandCallbacks) *Handler {
	return &Handler{
		cfg:       cfg,
		client:    client,
		commands:  make(chan Command, 10),
		callbacks: callbacks,
	}
}

// Start starts listening for control commands
func (h *Handler) Start(ctx context.Context) error {
	topic := h.cfg.MQTT.Topics.Control
	qos := h.cfg.MQTT.QoS["control"]

	slog.Info("subscribing to control plane", "topic", topic, "qos", qos)

	token := h.client.Subscribe(topic, qos, h.messageHandler)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("control plane subscription timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("control plane subscription failed: %w", err)
	}

	slog.Info("control plane handler started")

	// Process commands
	go h.processCommands(ctx)

	return nil
}

// Stop stops the control plane handler
func (h *Handler) Stop() error {
	topic := h.cfg.MQTT.Topics.Control

	if h.client != nil && h.client.IsConnected() {
		token := h.client.Unsubscribe(topic)
		token.Wait()
	}

	close(h.commands)

	slog.Info("control plane handler stopped")
	return nil
}

// messageHandler is called when a control message is received
func (h *Handler) messageHandler(client mqtt.Client, msg mqtt.Message) {
	var cmd Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		slog.Error("failed to parse control command", "error", err)
		h.sendResponse(Response{
			CommandAck: "unknown",
			Status:     "error",
			Error:      "invalid JSON",
		})
		return
	}

	slog.Info("control command received", "command", cmd.Command)

	// Send to processing channel
	select {
	case h.commands <- cmd:
	default:
		slog.Warn("command queue full, dropping command", "command", cmd.Command)
	}
}

// processCommands processes commands from the queue
func (h *Handler) processCommands(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd, ok := <-h.commands:
			if !ok {
				return
			}
			h.handleCommand(cmd)
		}
	}
}

// handleCommand executes a command
func (h *Handler) handleCommand(cmd Command) {
	var resp Response
	resp.CommandAck = cmd.Command

	switch cmd.Command {
	case "get_status":
		if h.callbacks.OnGetStatus != nil {
			resp.Status = "success"
			resp.Data = h.callbacks.OnGetStatus()
		} else {
			resp.Status = "error"
			resp.Error = "get_status not implemented"
		}

	case "pause_inference":
		if h.callbacks.OnPause != nil {
			if err := h.callbacks.OnPause(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				h.mu.Lock()
				h.isPaused = true
				h.mu.Unlock()
				resp.Status = "paused"
				resp.Data = map[string]interface{}{
					"inference_active": false,
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "pause not implemented"
		}

	case "resume_inference":
		if h.callbacks.OnResume != nil {
			if err := h.callbacks.OnResume(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				h.mu.Lock()
				h.isPaused = false
				h.mu.Unlock()
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"inference_active": true,
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "resume not implemented"
		}

	case "update_config":
		if h.callbacks.OnUpdateConfig != nil {
			if err := h.callbacks.OnUpdateConfig(cmd.Config); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"config_updated": true,
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "update_config not implemented"
		}

	case "set_inference_rate":
		if h.callbacks.OnSetInferenceRate != nil {
			// Extract rate from params
			rate, ok := cmd.Params["rate_hz"].(float64)
			if !ok {
				resp.Status = "error"
				resp.Error = "missing or invalid 'rate_hz' parameter (expected float)"
			} else {
				if err := h.callbacks.OnSetInferenceRate(rate); err != nil {
					resp.Status = "error"
					resp.Error = err.Error()
				} else {
					resp.Status = "success"
					resp.Data = map[string]interface{}{
						"inference_rate_hz": rate,
						"message":           "inference rate updated",
					}
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "set_inference_rate not implemented"
		}

	case "set_model_size":
		if h.callbacks.OnSetModelSize != nil {
			// Extract model size from params
			size, ok := cmd.Params["size"].(string)
			if !ok {
				resp.Status = "error"
				resp.Error = "missing or invalid 'size' parameter (expected string: n/s/m/l/x)"
			} else {
				if err := h.callbacks.OnSetModelSize(size); err != nil {
					resp.Status = "error"
					resp.Error = err.Error()
				} else {
					resp.Status = "success"
					resp.Data = map[string]interface{}{
						"model_size": size,
						"message":    "model size updated (reloading...)",
					}
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "set_model_size not implemented"
		}

	case "start_broadcast":
		if h.callbacks.OnStartBroadcast != nil {
			rtmpURL, okURL := cmd.Params["rtmp_url"].(string)
			filter, _ := cmd.Params["filter"].(string)       // Empty string if not provided
			overlayMode, _ := cmd.Params["overlay_mode"].(string) // Empty string if not provided

			if !okURL {
				resp.Status = "error"
				resp.Error = "missing or invalid 'rtmp_url' parameter (expected string)"
			} else {
				// Pass empty strings to let startBroadcast use stored config
				if err := h.callbacks.OnStartBroadcast(rtmpURL, filter, overlayMode); err != nil {
					resp.Status = "error"
					resp.Error = err.Error()
				} else {
					// Get actual config used (call getBroadcastStatus to get applied values)
					actualStatus := h.callbacks.OnGetBroadcastStatus()

					resp.Status = "success"
					resp.Data = map[string]interface{}{
						"broadcast_active": true,
						"rtmp_url":         rtmpURL,
						"filter":           actualStatus["filter"],
						"overlay_mode":     actualStatus["overlay_mode"],
						"message":          "broadcast started",
					}
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "start_broadcast not implemented"
		}

	case "stop_broadcast":
		if h.callbacks.OnStopBroadcast != nil {
			if err := h.callbacks.OnStopBroadcast(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"broadcast_active": false,
					"message":          "broadcast stopped",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "stop_broadcast not implemented"
		}

	case "set_broadcast_filter":
		if h.callbacks.OnSetBroadcastFilter != nil {
			filter, ok := cmd.Params["filter"].(string)
			if !ok {
				resp.Status = "error"
				resp.Error = "missing or invalid 'filter' parameter (expected string)"
			} else {
				if err := h.callbacks.OnSetBroadcastFilter(filter); err != nil {
					resp.Status = "error"
					resp.Error = err.Error()
				} else {
					resp.Status = "success"
					resp.Data = map[string]interface{}{
						"filter":  filter,
						"message": "broadcast filter updated (restart broadcast to apply)",
					}
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "set_broadcast_filter not implemented"
		}

	case "set_broadcast_overlay":
		if h.callbacks.OnSetBroadcastOverlay != nil {
			overlayMode, ok := cmd.Params["overlay_mode"].(string)
			if !ok {
				resp.Status = "error"
				resp.Error = "missing or invalid 'overlay_mode' parameter (expected string: none/box/crop)"
			} else {
				if err := h.callbacks.OnSetBroadcastOverlay(overlayMode); err != nil {
					resp.Status = "error"
					resp.Error = err.Error()
				} else {
					resp.Status = "success"
					resp.Data = map[string]interface{}{
						"overlay_mode": overlayMode,
						"message":      "broadcast overlay updated (restart broadcast to apply)",
					}
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "set_broadcast_overlay not implemented"
		}

	case "restart_broadcast":
		if h.callbacks.OnRestartBroadcast != nil {
			if err := h.callbacks.OnRestartBroadcast(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				actualStatus := h.callbacks.OnGetBroadcastStatus()
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"broadcast_restarted": true,
					"filter":              actualStatus["filter"],
					"overlay_mode":        actualStatus["overlay_mode"],
					"message":             "broadcast restarted with new configuration",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "restart_broadcast not implemented"
		}

	case "get_broadcast_status":
		if h.callbacks.OnGetBroadcastStatus != nil {
			resp.Status = "success"
			resp.Data = h.callbacks.OnGetBroadcastStatus()
		} else {
			resp.Status = "error"
			resp.Error = "get_broadcast_status not implemented"
		}

	case "set_attention_rois":
		if h.callbacks.OnSetAttentionROIs != nil {
			// Extract ROIs from params
			roisRaw, ok := cmd.Params["rois"].([]interface{})
			if !ok {
				resp.Status = "error"
				resp.Error = "missing or invalid 'rois' parameter (expected array of objects with x, y, width, height)"
			} else {
				// Convert []interface{} to []map[string]interface{}
				rois := make([]map[string]interface{}, len(roisRaw))
				for i, roiRaw := range roisRaw {
					roiMap, ok := roiRaw.(map[string]interface{})
					if !ok {
						resp.Status = "error"
						resp.Error = fmt.Sprintf("roi[%d] is not an object", i)
						break
					}
					rois[i] = roiMap
				}

				// Only call callback if no error occurred
				if resp.Status != "error" {
					if err := h.callbacks.OnSetAttentionROIs(rois); err != nil {
						resp.Status = "error"
						resp.Error = err.Error()
					} else {
						resp.Status = "success"
						resp.Data = map[string]interface{}{
							"attention_rois_set": true,
							"num_rois":           len(rois),
							"message":            "attention ROIs updated",
						}
					}
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "set_attention_rois not implemented"
		}

	case "clear_attention_rois":
		if h.callbacks.OnClearAttentionROIs != nil {
			if err := h.callbacks.OnClearAttentionROIs(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"attention_rois_cleared": true,
					"message":                "attention ROIs cleared - full frame processing",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "clear_attention_rois not implemented"
		}

	case "get_attention_rois":
		if h.callbacks.OnGetAttentionROIs != nil {
			resp.Status = "success"
			resp.Data = h.callbacks.OnGetAttentionROIs()
		} else {
			resp.Status = "error"
			resp.Error = "get_attention_rois not implemented"
		}

	case "set_auto_focus_strategy":
		if h.callbacks.OnSetAutoFocusStrategy != nil {
			if err := h.callbacks.OnSetAutoFocusStrategy(cmd.Params); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"auto_focus_strategy_updated": true,
					"strategy":                    cmd.Params["strategy"],
					"message":                     "auto-focus strategy updated",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "set_auto_focus_strategy not implemented"
		}

	case "get_auto_focus_strategy":
		if h.callbacks.OnGetAutoFocusStrategy != nil {
			resp.Status = "success"
			resp.Data = h.callbacks.OnGetAutoFocusStrategy()
		} else {
			resp.Status = "error"
			resp.Error = "get_auto_focus_strategy not implemented"
		}

	case "enable_auto_focus":
		if h.callbacks.OnEnableAutoFocus != nil {
			if err := h.callbacks.OnEnableAutoFocus(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"auto_focus_enabled": true,
					"message":            "auto-focus tracking enabled",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "enable_auto_focus not implemented"
		}

	case "disable_auto_focus":
		if h.callbacks.OnDisableAutoFocus != nil {
			if err := h.callbacks.OnDisableAutoFocus(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"auto_focus_enabled": false,
					"message":            "auto-focus tracking disabled",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "disable_auto_focus not implemented"
		}

	case "clear_auto_focus_history":
		if h.callbacks.OnClearAutoFocusHistory != nil {
			if err := h.callbacks.OnClearAutoFocusHistory(); err != nil {
				resp.Status = "error"
				resp.Error = err.Error()
			} else {
				resp.Status = "success"
				resp.Data = map[string]interface{}{
					"auto_focus_history_cleared": true,
					"message":                    "auto-focus history cleared",
				}
			}
		} else {
			resp.Status = "error"
			resp.Error = "clear_auto_focus_history not implemented"
		}

	case "get_auto_focus_stats":
		if h.callbacks.OnGetAutoFocusStats != nil {
			resp.Status = "success"
			resp.Data = h.callbacks.OnGetAutoFocusStats()
		} else {
			resp.Status = "error"
			resp.Error = "get_auto_focus_stats not implemented"
		}

	case "shutdown":
		if h.callbacks.OnShutdown != nil {
			slog.Warn("shutdown command received via MQTT control plane")
			resp.Status = "success"
			resp.Data = map[string]interface{}{
				"shutdown_initiated": true,
				"message":            "graceful shutdown in progress",
			}
			// Send response BEFORE triggering shutdown
			h.sendResponse(resp)

			// Trigger shutdown asynchronously
			go func() {
				time.Sleep(500 * time.Millisecond) // Brief delay to ensure response is sent
				if err := h.callbacks.OnShutdown(); err != nil {
					slog.Error("shutdown callback failed", "error", err)
				}
			}()
			return // Don't send response again
		} else {
			resp.Status = "error"
			resp.Error = "shutdown not implemented"
		}

	default:
		resp.Status = "error"
		resp.Error = fmt.Sprintf("unknown command: %s", cmd.Command)
	}

	h.sendResponse(resp)
}

// sendResponse sends a response to the health topic
func (h *Handler) sendResponse(resp Response) {
	resp.Timestamp = fmt.Sprintf("%d", 100000) // TODO: use proper timestamp

	payload, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		return
	}

	topic := h.cfg.MQTT.Topics.Health
	qos := h.cfg.MQTT.QoS["health"]

	token := h.client.Publish(topic, qos, false, payload)
	if !token.WaitTimeout(2 * time.Second) {
		slog.Error("response publish timeout")
		return
	}
	if err := token.Error(); err != nil {
		slog.Error("failed to publish response", "error", err)
		return
	}

	slog.Debug("response sent", "command_ack", resp.CommandAck, "status", resp.Status)
}

// IsPaused returns whether inference is paused
func (h *Handler) IsPaused() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.isPaused
}

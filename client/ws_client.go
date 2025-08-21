package client

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	Name           = "brinn-connector-go"
	Version        = "0.0.0"
	Keepalive      = true
	ReadLimitBytes = 655350
)

var (
	Timeout = time.Second * 40
)

// WsConfig webservice configuration
type WsConfig struct {
	endpoint             string
	handshakeTimeout     time.Duration
	enableCompression    bool
	keepConnectionAlive  bool
	maxReconnectAttempts int
	reconnectDelay       time.Duration
	reconnectBackoff     time.Duration
}

type WebsocketStreamClient struct {
	config *WsConfig
	logger *slog.Logger
}

type StreamClient interface {
	Connect(messageHandler MessageHandler, errorHandler ErrorHandler) (doneCh, stopCh chan struct{}, err error)
}

func NewWebsocketStreamClient(baseURL string, logger *slog.Logger) StreamClient {
	return &WebsocketStreamClient{
		config: &WsConfig{
			endpoint:             baseURL,
			handshakeTimeout:     Timeout,
			enableCompression:    false,
			keepConnectionAlive:  Keepalive,
			maxReconnectAttempts: 5,
			reconnectDelay:       2 * time.Second,
			reconnectBackoff:     5 * time.Second,
		},
		logger: logger,
	}
}

func (ws *WebsocketStreamClient) Connect(messageHandler MessageHandler, errorHandler ErrorHandler) (doneCh, stopCh chan struct{}, err error) {
	doneCh = make(chan struct{})
	stopCh = make(chan struct{})

	go ws.manageConnection(messageHandler, errorHandler, doneCh, stopCh)

	return doneCh, stopCh, nil
}

func (ws *WebsocketStreamClient) manageConnection(messageHandler MessageHandler, errorHandler ErrorHandler, doneCh, stopCh chan struct{}) {
	defer close(doneCh)

	reconnectionAttempts := 0
	reconnectDelay := ws.config.reconnectDelay

	for {
		// check if we stop
		select {
		case <-stopCh:
			return
		default:
		}

		conn, err := ws.connect()
		if err != nil {
			ws.logger.Error("Websocket connection failed", "error", err, "attempt", reconnectionAttempts+1)
			reconnectionAttempts++

			if reconnectionAttempts >= ws.config.maxReconnectAttempts {
				ws.logger.Error("Reached max reconnection attempts", "error", err, "attempt", reconnectionAttempts+1)
				errorHandler(fmt.Errorf("reached max reconnection attempts"))
				return
			}

			select {
			case <-stopCh:
				return
			case <-time.After(reconnectDelay):
				reconnectDelay = time.Duration(float64(reconnectDelay) * 1.5)
				continue
			}
		}

		// reached successful connection
		ws.logger.Info("WebSocket connected successfully", "endpoint", ws.config.endpoint)

		reconnectionAttempts = 0
		reconnectDelay = ws.config.reconnectDelay

		processingErr := ws.processIncomingMessages(conn, messageHandler, errorHandler, doneCh)
		if processingErr != nil {
			ws.logger.Debug("Websocket error - will continue to reconnection logic", "error", processingErr)
		}

		select {
		case <-stopCh:
			return
		default:
			// back to top of reconnection loop
		}
	}
}

func (ws *WebsocketStreamClient) connect() (*websocket.Conn, error) {
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  ws.config.handshakeTimeout,
		EnableCompression: ws.config.enableCompression,
	}

	headers := http.Header{}
	headers.Add("User-Agent", fmt.Sprintf("%s/%s", Name, Version))

	ws.logger.Debug("Connecting to websocket endpoint", "endpoint", ws.config.endpoint)
	conn, _, err := Dialer.Dial(ws.config.endpoint, headers)
	if err != nil {
		ws.logger.Error("Websocket connection failed", "error", err)
		return nil, err
	}

	conn.SetReadLimit(ReadLimitBytes)

	if ws.config.keepConnectionAlive {
		ws.keepAlive(conn)
	}

	return conn, nil
}

func (ws *WebsocketStreamClient) keepAlive(c *websocket.Conn) {
	// ping handler to keep connection alive
	c.SetPingHandler(func(pingData string) error {
		// Respond immediately with pong containing the same payload
		ws.logger.Debug("websocket client ping received", "pingData", pingData)
		err := c.WriteControl(websocket.PongMessage, []byte(pingData), time.Now().Add(time.Second))
		if err != nil {
			ws.logger.Warn("Failed to send pong response", "error", err)
		}
		return err
	})
}

func (ws *WebsocketStreamClient) processIncomingMessages(conn *websocket.Conn, messageHandler MessageHandler, errorHandler ErrorHandler, stopCh chan struct{}) error {
	// create a go routine to process incoming messages
	defer conn.Close()
	messageDone := make(chan struct{})

	go func() {
		defer close(messageDone)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				// Only treat unexpected closures as errors
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseAbnormalClosure,
					websocket.CloseNormalClosure, // Add this to ignore normal closures
				) {
					errorHandler(err)
				}
				return
			}
			messageHandler(message)
		}
	}()

	// creates a select statement that checks if we are stopped and closes message processing routine
	select {
	case <-stopCh:
		ws.logger.Debug("Received stop signal closing websocket connection")

		// Clean shutdown
		err := conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		if err != nil {
			ws.logger.Error("Failed to close websocket connection", "error", err)
		}
		<-messageDone
		return nil
	case <-messageDone:
		ws.logger.Debug("Message processing completed early")
		return fmt.Errorf("message processing completed early")
	}
}

package client

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	Name      = "brinn-connector-go"
	Version   = "0.0.0"
	Keepalive = true
)

var (
	Timeout = time.Second * 40
)

// WsConfig webservice configuration
type WsConfig struct {
	endpoint            string
	handshakeTimeout    time.Duration
	enableCompression   bool
	keepConnectionAlive bool
}

type WebsocketStreamClient struct {
	config *WsConfig
	logger *slog.Logger
}

type StreamClient interface {
	Subscribe(messageHandler MessageHandler, errorHandler ErrorHandler) (doneCh, stopCh chan struct{}, err error)
}

func NewWebsocketStreamClient(baseURL string, logger *slog.Logger) StreamClient {
	return &WebsocketStreamClient{
		config: &WsConfig{
			endpoint:            baseURL,
			handshakeTimeout:    Timeout,
			enableCompression:   false,
			keepConnectionAlive: Keepalive,
		},
		logger: logger,
	}
}

func (ws *WebsocketStreamClient) Subscribe(messageHandler MessageHandler, errorHandler ErrorHandler) (doneCh, stopCh chan struct{}, err error) {
	Dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  ws.config.handshakeTimeout,
		EnableCompression: ws.config.enableCompression,
	}
	headers := http.Header{}
	headers.Add("User-Agent", fmt.Sprintf("%s/%s", Name, Version))
	ws.logger.Debug("Connecting to websocket endpoint", "endpoint", ws.config.endpoint)
	c, _, err := Dialer.Dial(ws.config.endpoint, headers)
	if err != nil {
		ws.logger.Error("Websocket connection failed", "error", err)
		return nil, nil, err
	}
	c.SetReadLimit(655350)
	doneCh = make(chan struct{})
	stopCh = make(chan struct{})

	go func() {
		defer close(doneCh)
		defer func(c *websocket.Conn) {
			err := c.Close()
			if err != nil {
				ws.logger.Error("Websocket connection close failed", "error", err)
			}
		}(c)

		if ws.config.keepConnectionAlive {
			ws.keepAlive(c)
		}

		// Start message reading goroutine
		messageDone := make(chan struct{})
		go func() {
			defer close(messageDone)
			for {
				_, message, err := c.ReadMessage()
				if err != nil {
					// Only treat unexpected closures as errors
					if websocket.IsUnexpectedCloseError(err,
						websocket.CloseGoingAway,
						websocket.CloseAbnormalClosure,
						websocket.CloseNormalClosure, // Add this to ignore normal closures
					) {
						errorHandler(err)
					}

					// normal close is not an actual error
					return
				}
				messageHandler(message)
			}
		}()

		// Wait for stop signal from caller
		<-stopCh
		ws.logger.Debug("Received stop signal closing websocket connection")

		// Clean shutdown
		err = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		if err != nil {
			ws.logger.Error("Failed to close websocket connection", "error", err)
		}
		<-messageDone
	}()

	return doneCh, stopCh, nil
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

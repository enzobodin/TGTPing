package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func (app *App) connectWebSocket() error {
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 15 * time.Second

	headers := http.Header{}
	headers.Set("User-Agent", "TwitchNotificationBot/1.0")

	conn, resp, err := dialer.Dial("wss://eventsub.wss.twitch.tv/ws", headers)
	if err != nil {
		if resp != nil {
			log.Printf("WebSocket handshake failed with status: %d", resp.StatusCode)
		}
		return fmt.Errorf("failed to connect to EventSub WebSocket: %v", err)
	}

	if resp != nil {
		resp.Body.Close()
	}

	conn.SetReadLimit(1024 * 1024)
	conn.SetPongHandler(func(appData string) error {
		return nil
	})

	app.wsConn = conn
	log.Println("WebSocket connection established")
	return nil
}

func (app *App) handleWebSocketMessages() {
	defer func() {
		if app.wsConn != nil {
			app.wsConn.Close()
		}
	}()

	consecutiveErrors := 0
	maxConsecutiveErrors := 5

	for {
		select {
		case <-app.ctx.Done():
			log.Println("WebSocket handler stopping...")
			return
		default:
			if app.wsConn == nil {
				webSocketStreamers := app.streamerManager.getWebSocketStreamers()
				if len(webSocketStreamers) == 0 {
					log.Println("No high-priority streamers - stopping WebSocket handler")
					return
				}
				log.Println("WebSocket connection is nil, attempting to reconnect...")
				if err := app.reconnectWebSocket(); err != nil {
					log.Printf("Failed to reconnect: %v", err)
					time.Sleep(10 * time.Second)
				}
				continue
			}

			app.wsConn.SetReadDeadline(time.Now().Add(70 * time.Second))

			var message EventSubMessage
			err := app.wsConn.ReadJSON(&message)
			if err != nil {
				consecutiveErrors++
				log.Printf("Error reading WebSocket message: %v", err)

				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("Too many consecutive errors (%d), backing off for 30 seconds", consecutiveErrors)
					time.Sleep(30 * time.Second)
					consecutiveErrors = 0
				}

				webSocketStreamers := app.streamerManager.getWebSocketStreamers()
				if len(webSocketStreamers) == 0 {
					log.Println("No high-priority streamers - stopping WebSocket handler after error")
					return
				}

				if err := app.reconnectWebSocket(); err != nil {
					log.Printf("Failed to reconnect: %v", err)
					time.Sleep(5 * time.Second)
				}
				continue
			}

			consecutiveErrors = 0

			if err := app.processWebSocketMessage(message); err != nil {
				log.Printf("Error processing WebSocket message: %v", err)
			}
		}
	}
}

func (app *App) reconnectWebSocket() error {
	app.wsMutex.Lock()
	defer app.wsMutex.Unlock()

	if app.reconnecting {
		return fmt.Errorf("reconnection already in progress")
	}

	app.reconnecting = true
	defer func() {
		app.reconnecting = false
	}()

	log.Println("Attempting to reconnect WebSocket...")

	if app.wsConn != nil {
		app.wsConn.Close()
		app.wsConn = nil
	}

	app.sessionID = ""

	time.Sleep(2 * time.Second)

	if err := app.connectWebSocket(); err != nil {
		return err
	}

	log.Println("WebSocket reconnection successful")
	return nil
}

func (app *App) processWebSocketMessage(message EventSubMessage) error {
	switch message.Metadata.MessageType {
	case "session_welcome":
		if message.Payload.Session != nil {
			app.sessionID = message.Payload.Session.ID
			log.Printf("WebSocket session established: %s", app.sessionID)

			if err := app.getTwitchUserToken(); err != nil {
				log.Printf("No user access token available for EventSub subscriptions: %v", err)

				if app.wsConn != nil {
					log.Println("Closing unused WebSocket connection")
					app.wsConn.Close()
					app.wsConn = nil
					app.sessionID = ""
				}
			} else {
				go app.subscribeToExistingStreamers()
			}
		}

	case "notification":
		if message.Payload.Subscription != nil && message.Payload.Event != nil {
			return app.handleEventNotification(message.Payload.Subscription, message.Payload.Event)
		}

	case "session_keepalive":
		// nothing
	case "session_reconnect":
		if message.Payload.Session != nil && message.Payload.Session.ReconnectURL != "" {
			log.Printf("Received reconnect message: %s", message.Payload.Session.ReconnectURL)
		}

	case "revocation":
		if message.Payload.Subscription != nil {
			log.Printf("Subscription revoked: %s", message.Payload.Subscription.ID)
			app.removeSubscription(message.Payload.Subscription.Condition.BroadcasterUserID)
		}

	default:
		log.Printf("Unknown message type: %s", message.Metadata.MessageType)
	}

	return nil
}

func (app *App) handleEventNotification(subscription *EventSubSubscription, event *EventSubEvent) error {
	switch subscription.Type {
	case "stream.online":
		streamer := app.streamerManager.getStreamerByUserID(event.BroadcasterUserID)
		if streamer == nil {
			log.Printf("Received stream.online for unknown streamer: %s", event.BroadcasterUserID)
			return nil
		}

		if !streamer.IsLive {
			log.Printf("Stream online: %s (%s)", streamer.DisplayName, streamer.Username)

			streamData, err := app.getStreamInfo(event.BroadcasterUserID)
			if err != nil {
				log.Printf("Error getting stream info for %s: %v", streamer.Username, err)
			}

			if err := app.sendNotification(streamer, streamData); err != nil {
				log.Printf("Error sending notification for %s: %v", streamer.Username, err)
			}

			app.streamerManager.updateStreamerStatus(event.BroadcasterUserID, true)
		}

	case "stream.offline":
		streamer := app.streamerManager.getStreamerByUserID(event.BroadcasterUserID)
		if streamer != nil {
			log.Printf("Stream offline: %s (%s)", streamer.DisplayName, streamer.Username)
			app.streamerManager.updateStreamerStatus(event.BroadcasterUserID, false)
		}

	default:
		log.Printf("Unknown subscription type: %s", subscription.Type)
	}

	return nil
}

func (app *App) subscribeToEvents(userID string) error {
	if app.sessionID == "" {
		return fmt.Errorf("no WebSocket session available")
	}

	if app.wsConn == nil {
		return fmt.Errorf("WebSocket connection is not established")
	}

	app.subMutex.RLock()
	_, hasOnline := app.subscriptions[userID+"_online"]
	_, hasOffline := app.subscriptions[userID+"_offline"]
	app.subMutex.RUnlock()

	if hasOnline && hasOffline {
		log.Printf("User %s already has active subscriptions", userID)
		return nil
	}

	onlineReq := EventSubSubscriptionRequest{
		Type:    "stream.online",
		Version: "1",
		Condition: EventSubCondition{
			BroadcasterUserID: userID,
		},
		Transport: EventSubTransport{
			Method:    "websocket",
			SessionID: app.sessionID,
		},
	}

	onlineSubID, err := app.createSubscription(onlineReq)
	if err != nil {
		return fmt.Errorf("failed to subscribe to stream.online: %v", err)
	}

	offlineReq := EventSubSubscriptionRequest{
		Type:    "stream.offline",
		Version: "1",
		Condition: EventSubCondition{
			BroadcasterUserID: userID,
		},
		Transport: EventSubTransport{
			Method:    "websocket",
			SessionID: app.sessionID,
		},
	}

	offlineSubID, err := app.createSubscription(offlineReq)
	if err != nil {
		return fmt.Errorf("failed to subscribe to stream.offline: %v", err)
	}

	app.subMutex.Lock()
	app.subscriptions[userID+"_online"] = onlineSubID
	app.subscriptions[userID+"_offline"] = offlineSubID
	app.subMutex.Unlock()

	log.Printf("Successfully subscribed to events for user %s", userID)
	return nil
}

func (app *App) createSubscription(req EventSubSubscriptionRequest) (string, error) {
	if err := app.getTwitchUserToken(); err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.twitch.tv/helix/eventsub/subscriptions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Client-ID", app.config.TwitchClientID)
	httpReq.Header.Set("Authorization", "Bearer "+app.twitchUserToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("subscription failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []EventSubSubscription `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if len(result.Data) == 0 {
		return "", fmt.Errorf("no subscription created")
	}

	return result.Data[0].ID, nil
}

func (app *App) removeSubscription(userID string) {
	app.subMutex.Lock()
	defer app.subMutex.Unlock()

	delete(app.subscriptions, userID+"_online")
	delete(app.subscriptions, userID+"_offline")
}

func (app *App) subscribeToExistingStreamers() {
	webSocketStreamers := app.streamerManager.getWebSocketStreamers()
	log.Printf("Subscribing to %d high-priority streamers via WebSocket", len(webSocketStreamers))

	if len(webSocketStreamers) == 0 {
		return
	}

	app.subMutex.Lock()
	app.subscriptions = make(map[string]string)
	app.subMutex.Unlock()

	for _, streamer := range webSocketStreamers {
		if err := app.subscribeToEvents(streamer.UserID); err != nil {
			log.Printf("Failed to subscribe to events for %s: %v", streamer.Username, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (app *App) ensureWebSocketConnection() error {
	webSocketStreamers := app.streamerManager.getWebSocketStreamers()
	hasWebSocketStreamers := len(webSocketStreamers) > 0

	if !hasWebSocketStreamers && app.wsConn != nil {
		log.Println("No high-priority streamers - closing unused WebSocket connection")
		app.disconnectWebSocket()
		return nil
	}

	if hasWebSocketStreamers && (app.wsConn == nil || app.sessionID == "") {
		if err := app.getTwitchUserToken(); err != nil {
			return fmt.Errorf("no user access token available: %v", err)
		}

		log.Printf("Connecting to WebSocket for %d high-priority streamers", len(webSocketStreamers))
		if err := app.connectWebSocket(); err != nil {
			return fmt.Errorf("failed to connect to WebSocket: %v", err)
		}

		go app.handleWebSocketMessages()
		return nil
	}

	return nil
}

func (app *App) disconnectWebSocket() {
	app.wsMutex.Lock()
	defer app.wsMutex.Unlock()

	if app.wsConn != nil {
		app.wsConn.Close()
		app.wsConn = nil
	}
	app.sessionID = ""

	app.subMutex.Lock()
	app.subscriptions = make(map[string]string)
	app.subMutex.Unlock()

	log.Println("WebSocket connection closed")
}

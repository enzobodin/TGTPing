package main

import (
	"context"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gorilla/websocket"
)

type Config struct {
	TwitchClientID        string
	TwitchClientSecret    string
	TelegramBotToken      string
	TelegramChatID        int64
	OAuthCallbackURL      string
	MaxWebSocketStreamers int
	PollingInterval       time.Duration
}

type TokenData struct {
	UserAccessToken string    `json:"user_access_token"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Streamer struct {
	Username         string    `json:"username"`
	DisplayName      string    `json:"display_name"`
	UserID           string    `json:"user_id"`
	IsLive           bool      `json:"is_live"`
	LastChecked      time.Time `json:"last_checked"`
	Priority         string    `json:"priority"`
	NotificationMode string    `json:"notification_mode"`
}

type StreamerManager struct {
	streamers map[string]*Streamer
	mutex     sync.RWMutex
	filename  string
}

// Twitch API types
type TwitchTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type TwitchUserResponse struct {
	Data []struct {
		ID          string `json:"id"`
		Login       string `json:"login"`
		DisplayName string `json:"display_name"`
	} `json:"data"`
}

type TwitchStreamResponse struct {
	Data []struct {
		UserID      string `json:"user_id"`
		UserLogin   string `json:"user_login"`
		UserName    string `json:"user_name"`
		GameName    string `json:"game_name"`
		Title       string `json:"title"`
		ViewerCount int    `json:"viewer_count"`
		StartedAt   string `json:"started_at"`
	} `json:"data"`
}

type EventSubMessage struct {
	Metadata EventSubMetadata `json:"metadata"`
	Payload  EventSubPayload  `json:"payload"`
}

type EventSubMetadata struct {
	MessageID        string `json:"message_id"`
	MessageType      string `json:"message_type"`
	MessageTimestamp string `json:"message_timestamp"`
}

type EventSubPayload struct {
	Session      *EventSubSession      `json:"session,omitempty"`
	Subscription *EventSubSubscription `json:"subscription,omitempty"`
	Event        *EventSubEvent        `json:"event,omitempty"`
}

type EventSubSession struct {
	ID                   string `json:"id"`
	Status               string `json:"status"`
	ConnectedAt          string `json:"connected_at"`
	KeepaliveTimeoutSecs int    `json:"keepalive_timeout_seconds"`
	ReconnectURL         string `json:"reconnect_url"`
}

type EventSubSubscription struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Cost      int               `json:"cost"`
	Condition EventSubCondition `json:"condition"`
	Transport EventSubTransport `json:"transport"`
	CreatedAt string            `json:"created_at"`
}

type EventSubCondition struct {
	BroadcasterUserID string `json:"broadcaster_user_id"`
}

type EventSubTransport struct {
	Method    string `json:"method"`
	SessionID string `json:"session_id"`
}

type EventSubEvent struct {
	BroadcasterUserID    string `json:"broadcaster_user_id"`
	BroadcasterUserLogin string `json:"broadcaster_user_login"`
	BroadcasterUserName  string `json:"broadcaster_user_name"`
	ID                   string `json:"id,omitempty"`
	Type                 string `json:"type,omitempty"`
	StartedAt            string `json:"started_at,omitempty"`
}

type EventSubSubscriptionRequest struct {
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition EventSubCondition `json:"condition"`
	Transport EventSubTransport `json:"transport"`
}

type App struct {
	config          Config
	streamerManager *StreamerManager
	bot             *tgbotapi.BotAPI
	twitchToken     string
	twitchUserToken string
	tokenExpiry     time.Time
	userTokenExpiry time.Time
	wsConn          *websocket.Conn
	sessionID       string
	subscriptions   map[string]string
	subMutex        sync.RWMutex
	wsMutex         sync.Mutex
	reconnecting    bool
	ctx             context.Context
	cancel          context.CancelFunc
	pollingTicker   *time.Ticker
	pollingMutex    sync.Mutex
}

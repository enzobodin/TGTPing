package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Config struct {
	TwitchClientID     string
	TwitchClientSecret string
	TelegramBotToken   string
	TelegramChatID     int64
	PollingInterval    time.Duration
}

type Streamer struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	UserID      string    `json:"user_id"`
	IsLive      bool      `json:"is_live"`
	LastChecked time.Time `json:"last_checked"`
}

type StreamerManager struct {
	streamers map[string]*Streamer
	mutex     sync.RWMutex
	filename  string
}

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
	Data []TwitchStreamData `json:"data"`
}

type TwitchStreamData struct {
	UserID      string `json:"user_id"`
	UserLogin   string `json:"user_login"`
	UserName    string `json:"user_name"`
	GameName    string `json:"game_name"`
	Title       string `json:"title"`
	ViewerCount int    `json:"viewer_count"`
	StartedAt   string `json:"started_at"`
}

type App struct {
	config          Config
	streamerManager *StreamerManager
	bot             *tgbotapi.BotAPI
	twitchToken     string
	tokenExpiry     time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	pollingTicker   *time.Ticker
	pollingMutex    sync.Mutex
	httpClient      *http.Client
}

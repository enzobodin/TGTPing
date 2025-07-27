package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

func loadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	chatID, err := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
	if err != nil {
		log.Fatal("Invalid TELEGRAM_CHAT_ID:", err)
	}

	oauthURL := os.Getenv("OAUTH_CALLBACK_URL")

	maxWebSocketStreamers := 5
	if env := os.Getenv("MAX_WEBSOCKET_STREAMERS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil && val > 0 {
			maxWebSocketStreamers = val
		}
	}

	pollingInterval := 90 * time.Second
	if env := os.Getenv("POLLING_INTERVAL_SECONDS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil && val >= 30 {
			pollingInterval = time.Duration(val) * time.Second
		}
	}

	return Config{
		TwitchClientID:        os.Getenv("TWITCH_CLIENT_ID"),
		TwitchClientSecret:    os.Getenv("TWITCH_CLIENT_SECRET"),
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:        chatID,
		OAuthCallbackURL:      oauthURL,
		MaxWebSocketStreamers: maxWebSocketStreamers,
		PollingInterval:       pollingInterval,
	}
}

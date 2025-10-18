package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	DefaultPollingInterval = 90 * time.Second
	MinPollingInterval     = 30 * time.Second
	DefaultHTTPTimeout     = 10 * time.Second
	StreamersFilePath      = "/data/streamers.json"
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

	pollingInterval := DefaultPollingInterval
	if env := os.Getenv("POLLING_INTERVAL_SECONDS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil && val >= int(MinPollingInterval.Seconds()) {
			pollingInterval = time.Duration(val) * time.Second
		}
	}

	return Config{
		TwitchClientID:     os.Getenv("TWITCH_CLIENT_ID"),
		TwitchClientSecret: os.Getenv("TWITCH_CLIENT_SECRET"),
		TelegramBotToken:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:     chatID,
		PollingInterval:    pollingInterval,
	}
}

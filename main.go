package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	config := loadConfig()

	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		log.Fatal("Failed to create Telegram bot:", err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	streamerManager := NewStreamerManager(StreamersFilePath)
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:          config,
		streamerManager: streamerManager,
		bot:             bot,
		ctx:             ctx,
		cancel:          cancel,
		httpClient:      &http.Client{},
	}

	app.initialize()
	app.waitForShutdown()
}

func (app *App) initialize() {
	app.startPollingManager()
	go app.handleTelegramUpdates()
}

func (app *App) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	app.cancel()
	app.stopPollingManager()

	log.Println("Shutdown complete")
}

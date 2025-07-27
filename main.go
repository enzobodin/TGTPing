package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gorilla/mux"
)

func main() {
	config := loadConfig()

	bot, err := tgbotapi.NewBotAPI(config.TelegramBotToken)
	if err != nil {
		log.Fatal("Failed to create Telegram bot:", err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	streamerManager := NewStreamerManager("/data/streamers.json")
	ctx, cancel := context.WithCancel(context.Background())

	app := &App{
		config:          config,
		streamerManager: streamerManager,
		bot:             bot,
		subscriptions:   make(map[string]string),
		reconnecting:    false,
		ctx:             ctx,
		cancel:          cancel,
	}

	webSocketStreamers := app.streamerManager.getWebSocketStreamers()
	if len(webSocketStreamers) > config.MaxWebSocketStreamers {
		if err := app.streamerManager.assignNotificationModes(config.MaxWebSocketStreamers); err != nil {
			log.Printf("Error reassigning notification modes: %v", err)
		}
		webSocketStreamers = app.streamerManager.getWebSocketStreamers()
	}

	if len(webSocketStreamers) > 0 && app.getTwitchUserToken() == nil {
		if err := app.connectWebSocket(); err != nil {
			log.Printf("Failed to connect to EventSub WebSocket: %v", err)
		} else {
			go app.handleWebSocketMessages()
		}
	}

	app.startPollingManager()
	go app.handleTelegramUpdates()

	r := mux.NewRouter()
	r.HandleFunc("/oauth/callback", app.oauthHandler).Methods("GET")
	r.HandleFunc("/", app.oauthHandler).Methods("GET")

	srv := &http.Server{Addr: ":8080", Handler: r}
	go func() {
		log.Println("Starting HTTP server on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	cancel()
	app.stopPollingManager()
	if app.wsConn != nil {
		app.wsConn.Close()
	}
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	log.Println("Shutdown complete")
}

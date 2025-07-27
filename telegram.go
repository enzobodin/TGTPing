package main

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (app *App) sendNotification(streamer *Streamer, streamData *TwitchStreamResponse) error {
	var message string
	if streamData != nil && len(streamData.Data) > 0 {
		stream := streamData.Data[0]
		message = fmt.Sprintf("🔴 %s is now live!\n\n📺 %s\n🎮 %s\n👥 %d viewers\n\n🔗 https://twitch.tv/%s",
			streamer.DisplayName,
			stream.Title,
			stream.GameName,
			stream.ViewerCount,
			streamer.Username)
	} else {
		message = fmt.Sprintf("🔴 %s is now live!\n\n🔗 https://twitch.tv/%s",
			streamer.DisplayName,
			streamer.Username)
	}

	msg := tgbotapi.NewMessage(app.config.TelegramChatID, message)
	_, err := app.bot.Send(msg)
	return err
}

func (app *App) handleTelegramUpdates() {
	log.Println("Starting Telegram updates handler")
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := app.bot.GetUpdatesChan(u)

	for update := range updates {
		select {
		case <-app.ctx.Done():
			log.Println("Telegram updates handler stopping")
			return
		default:
		}

		if update.Message == nil {
			continue
		}

		if update.Message.Chat.ID != app.config.TelegramChatID {
			log.Printf("Ignoring message from unauthorized chat: %d", update.Message.Chat.ID)
			continue
		}

		go app.handleTelegramCommand(update.Message)
	}
}

func (app *App) handleTelegramCommand(message *tgbotapi.Message) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in command handler: %v", r)
			msg := tgbotapi.NewMessage(message.Chat.ID, "❌ Internal error processing command. Please try again.")
			app.bot.Send(msg)
		}
	}()

	command := message.Command()
	args := message.CommandArguments()

	var responseText string
	switch command {
	case "add":
		responseText = app.handleAddCommand(args)
	case "remove", "delete":
		responseText = app.handleRemoveCommand(args)
	case "list":
		responseText = app.handleListCommand()
	case "check":
		responseText = app.handleCheckCommand()
	case "priority":
		responseText = app.handlePriorityCommand(args)
	case "status":
		responseText = app.handleStatusCommand()
	case "help":
		responseText = app.getHelpText()
	default:
		if command != "" {
			responseText = "Unknown command. Use /help to see available commands."
		}
	}

	if responseText != "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, responseText)
		if _, err := app.bot.Send(msg); err != nil {
			log.Printf("Error sending Telegram message: %v", err)
		}
	}
}

func (app *App) handleAddCommand(args string) string {
	if args == "" {
		return "Usage: /add <twitch_username>"
	}

	username := strings.ToLower(strings.TrimSpace(args))

	streamers := app.streamerManager.getStreamers()
	for _, s := range streamers {
		if s.Username == username {
			return fmt.Sprintf("⚠️ %s is already in the notification list", s.DisplayName)
		}
	}

	streamer, err := app.getTwitchUser(username)
	if err != nil {
		log.Printf("Error getting Twitch user %s: %v", username, err)
		return fmt.Sprintf("❌ Error: Could not find Twitch user '%s'. Please check the username and try again.", username)
	}

	for _, s := range streamers {
		if s.UserID == streamer.UserID {
			return fmt.Sprintf("⚠️ %s is already in the notification list (same as %s)", streamer.DisplayName, s.DisplayName)
		}
	}

	streamInfo, err := app.getStreamInfo(streamer.UserID)
	if err != nil {
		log.Printf("Error checking stream status for %s: %v", streamer.Username, err)
	} else if streamInfo != nil && len(streamInfo.Data) > 0 {
		streamer.IsLive = true
	} else {
		streamer.IsLive = false
	}

	if err := app.streamerManager.addStreamer(streamer); err != nil {
		log.Printf("Error adding streamer %s: %v", username, err)
		return fmt.Sprintf("❌ Error adding streamer: %v", err)
	}

	if err := app.streamerManager.assignNotificationModes(app.config.MaxWebSocketStreamers); err != nil {
		log.Printf("Error reassigning notification modes: %v", err)
	}

	go func() {
		if err := app.ensureWebSocketConnection(); err != nil {
			log.Printf("Error managing WebSocket connection: %v", err)
		}
	}()

	return fmt.Sprintf("✅ Added %s (%s) to notifications (normal priority)", streamer.DisplayName, streamer.Username)
}

func (app *App) handleRemoveCommand(args string) string {
	if args == "" {
		return "Usage: /remove <twitch_username>"
	}

	username := strings.ToLower(strings.TrimSpace(args))

	streamers := app.streamerManager.getStreamers()
	var removedStreamer *Streamer
	for _, s := range streamers {
		if s.Username == username {
			removedStreamer = s
			break
		}
	}

	if removedStreamer == nil {
		return fmt.Sprintf("❌ %s is not in the notification list", username)
	}

	if err := app.streamerManager.removeStreamer(username); err != nil {
		log.Printf("Error removing streamer %s: %v", username, err)
		return fmt.Sprintf("❌ Error removing streamer: %v", err)
	}

	app.removeSubscription(removedStreamer.UserID)

	if err := app.streamerManager.assignNotificationModes(app.config.MaxWebSocketStreamers); err != nil {
		log.Printf("Error reassigning notification modes: %v", err)
	}

	go func() {
		if err := app.ensureWebSocketConnection(); err != nil {
			log.Printf("Error managing WebSocket connection: %v", err)
		}
	}()

	return fmt.Sprintf("✅ Removed %s from notifications", removedStreamer.DisplayName)
}

func (app *App) handleListCommand() string {
	streamers := app.streamerManager.getStreamers()
	if len(streamers) == 0 {
		return "📋 No streamers in the notification list.\n\nUse /add <username> to add streamers!"
	}

	responseText := "📋 Current streamers:\n\n"

	webSocketCount := 0
	pollingCount := 0

	for _, streamer := range streamers {
		status := "🔴"
		if !streamer.IsLive {
			status = "⚫"
		}

		priorityIcon := "⚡"
		if streamer.Priority != "high" {
			priorityIcon = "🔹"
		}

		modeIcon := "🌐"
		if streamer.NotificationMode == "polling" {
			modeIcon = "🔄"
			pollingCount++
		} else {
			webSocketCount++
		}

		responseText += fmt.Sprintf("%s %s %s %s (%s)\n", status, priorityIcon, modeIcon, streamer.DisplayName, streamer.Username)
	}

	responseText += fmt.Sprintf("\n📊 Total: %d streamers", len(streamers))
	responseText += fmt.Sprintf("\n🌐 Real-time: %d/%d", webSocketCount, app.config.MaxWebSocketStreamers)
	responseText += fmt.Sprintf("\n🔄 Polling: %d", pollingCount)
	responseText += "\n\n⚡ = High priority | 🔹 = Normal priority"
	responseText += "\n🌐 = Real-time | 🔄 = Polling"

	return responseText
}

func (app *App) handleCheckCommand() string {
	streamers := app.streamerManager.getStreamers()
	if len(streamers) == 0 {
		return "📋 No streamers to check.\n\nUse /add <username> to add streamers!"
	}

	responseText := "🔍 Live Status Check:\n\n"

	for _, streamer := range streamers {
		streamInfo, err := app.getStreamInfo(streamer.UserID)
		if err != nil {
			log.Printf("Error checking stream for %s: %v", streamer.Username, err)
			responseText += fmt.Sprintf("❌ %s - Error checking status\n", streamer.DisplayName)
			continue
		}

		isCurrentlyLive := streamInfo != nil && len(streamInfo.Data) > 0

		if isCurrentlyLive {
			stream := streamInfo.Data[0]
			responseText += fmt.Sprintf("🔴 %s is LIVE!\n", streamer.DisplayName)
			responseText += fmt.Sprintf("   📺 %s\n", stream.Title)
			responseText += fmt.Sprintf("   🎮 %s\n", stream.GameName)
			responseText += fmt.Sprintf("   👥 %d viewers\n\n", stream.ViewerCount)

			if !streamer.IsLive {
				app.streamerManager.updateStreamerStatus(streamer.UserID, true)
			}
		} else {
			responseText += fmt.Sprintf("⚫ %s is offline\n", streamer.DisplayName)

			if streamer.IsLive {
				app.streamerManager.updateStreamerStatus(streamer.UserID, false)
			}
		}
	}

	responseText += "\n💡 This command manually checks current status and updates the bot's internal state."
	return responseText
}

func (app *App) handlePriorityCommand(args string) string {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return "Usage: /priority <username> <high|normal>\n\nExample: /priority ninja high"
	}

	username := strings.ToLower(strings.TrimSpace(parts[0]))
	priority := strings.ToLower(strings.TrimSpace(parts[1]))

	if priority != "high" && priority != "normal" {
		return "❌ Priority must be 'high' or 'normal'"
	}

	streamers := app.streamerManager.getStreamers()
	var targetStreamer *Streamer
	for _, s := range streamers {
		if s.Username == username {
			targetStreamer = s
			break
		}
	}

	if targetStreamer == nil {
		return fmt.Sprintf("❌ %s is not in the notification list. Use /add to add them first.", username)
	}

	if err := app.streamerManager.setStreamerPriority(username, priority); err != nil {
		log.Printf("Error setting priority for %s: %v", username, err)
		return fmt.Sprintf("❌ Error setting priority: %v", err)
	}

	if err := app.streamerManager.assignNotificationModes(app.config.MaxWebSocketStreamers); err != nil {
		log.Printf("Error reassigning notification modes: %v", err)
	}

	go func() {
		if err := app.ensureWebSocketConnection(); err != nil {
			log.Printf("Error managing WebSocket connection: %v", err)
		}
	}()

	updatedStreamer := app.streamerManager.getStreamerByUserID(targetStreamer.UserID)
	mode := "polling"
	if updatedStreamer != nil {
		mode = updatedStreamer.NotificationMode
	}

	return fmt.Sprintf("✅ Set %s priority to %s (Mode: %s)", targetStreamer.DisplayName, priority, mode)
}

func (app *App) handleStatusCommand() string {
	webSocketStreamers := app.streamerManager.getWebSocketStreamers()
	pollingStreamers := app.streamerManager.getPollingStreamers()

	responseText := "📊 Notification System Status:\n\n"

	responseText += fmt.Sprintf("🌐 WebSocket (Real-time): %d/%d streamers\n", len(webSocketStreamers), app.config.MaxWebSocketStreamers)
	if len(webSocketStreamers) > 0 {
		responseText += "   High-priority streamers:\n"
		for _, s := range webSocketStreamers {
			status := "🔴"
			if !s.IsLive {
				status = "⚫"
			}
			responseText += fmt.Sprintf("   %s %s\n", status, s.DisplayName)
		}
	}

	responseText += fmt.Sprintf("\n🔄 Polling (~%ds delay): %d streamers\n", int(app.config.PollingInterval.Seconds()), len(pollingStreamers))
	if len(pollingStreamers) > 0 {
		responseText += "   Normal-priority streamers:\n"
		for _, s := range pollingStreamers {
			status := "🔴"
			if !s.IsLive {
				status = "⚫"
			}
			responseText += fmt.Sprintf("   %s %s\n", status, s.DisplayName)
		}
	}

	wsStatus := "❌ Disconnected"
	if app.wsConn != nil && app.sessionID != "" {
		wsStatus = "✅ Connected"
	}
	responseText += fmt.Sprintf("\n🔗 WebSocket Connection: %s\n", wsStatus)

	if app.wsConn == nil {
		responseText += fmt.Sprintf("\n💡 To enable real-time notifications, visit: %s", app.config.OAuthCallbackURL)
	}

	return responseText
}

func (app *App) getHelpText() string {
	return fmt.Sprintf(`🤖 Twitch Notification Bot Commands:

/add <username> - Add a Twitch streamer to notifications (starts as normal priority)
/remove <username> - Remove a streamer from notifications  
/list - Show all tracked streamers with live status and notification modes
/priority <username> <high|normal> - Set streamer priority level
/status - Show notification system status and configuration
/check - Check current live status and update internal state
/help - Show this help message

🚀 Hybrid Notification System:
• High-priority streamers: Real-time WebSocket notifications (~2-3s delay)
• Normal-priority streamers: Polling notifications (~%ds delay)
• Maximum %d high-priority streamers supported

📊 Priority Levels:
• High: Uses real-time WebSocket (limited slots)
• Normal: Uses polling system (unlimited)

⚠️ Note: Real-time notifications require user authentication. 
Visit %s to authorize the bot with your Twitch account.

Examples:
/add ninja              # Add ninja (normal priority)
/priority ninja high    # Upgrade to high priority  
/add shroud            # Add shroud (normal priority)
/status                # View system status
/list                  # View all streamers
/remove ninja          # Remove ninja`,
		int(app.config.PollingInterval.Seconds()),
		app.config.MaxWebSocketStreamers,
		app.config.OAuthCallbackURL)
}

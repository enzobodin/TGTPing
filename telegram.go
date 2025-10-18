package main

import (
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (app *App) sendNotification(streamer *Streamer, streamData *TwitchStreamResponse) error {
	message := fmt.Sprintf("🔴 %s is now live!\n\n", streamer.DisplayName)

	if streamData != nil && len(streamData.Data) > 0 {
		stream := streamData.Data[0]
		message += fmt.Sprintf("📺 %s\n🎮 %s\n👥 %d viewers\n\n", stream.Title, stream.GameName, stream.ViewerCount)
	}

	message += fmt.Sprintf("🔗 https://twitch.tv/%s", streamer.Username)

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
			if _, err := app.bot.Send(msg); err != nil {
				log.Printf("Error sending panic recovery message: %v", err)
			}
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
	username, err := validateUsernameArg(args)
	if err != nil {
		return err.Error()
	}

	if existingStreamer := app.findStreamerByUsername(username); existingStreamer != nil {
		return fmt.Sprintf("⚠️ %s is already in the notification list", existingStreamer.DisplayName)
	}

	streamer, err := app.getTwitchUser(username)
	if err != nil {
		log.Printf("Error getting Twitch user %s: %v", username, err)
		return fmt.Sprintf("❌ Error: Could not find Twitch user '%s'. Please check the username and try again.", username)
	}

	streamers := app.streamerManager.getStreamers()
	for _, s := range streamers {
		if s.UserID == streamer.UserID {
			return fmt.Sprintf("⚠️ %s is already in the notification list (same as %s)", streamer.DisplayName, s.DisplayName)
		}
	}

	streamInfo, err := app.getStreamInfo(streamer.UserID)
	if err != nil {
		log.Printf("Error checking stream status for %s: %v", streamer.Username, err)
	}
	streamer.IsLive = streamInfo != nil && len(streamInfo.Data) > 0

	if err := app.streamerManager.addStreamer(streamer); err != nil {
		log.Printf("Error adding streamer %s: %v", username, err)
		return fmt.Sprintf("❌ Error adding streamer: %v", err)
	}

	return fmt.Sprintf("✅ Added %s (%s) to notifications", streamer.DisplayName, streamer.Username)
}

func (app *App) handleRemoveCommand(args string) string {
	username, err := validateUsernameArg(args)
	if err != nil {
		return strings.ReplaceAll(err.Error(), "/add", "/remove")
	}

	removedStreamer := app.findStreamerByUsername(username)
	if removedStreamer == nil {
		return fmt.Sprintf("❌ %s is not in the notification list", username)
	}

	if err := app.streamerManager.removeStreamer(username); err != nil {
		log.Printf("Error removing streamer %s: %v", username, err)
		return fmt.Sprintf("❌ Error removing streamer: %v", err)
	}

	return fmt.Sprintf("✅ Removed %s from notifications", removedStreamer.DisplayName)
}

func (app *App) handleListCommand() string {
	streamers := app.streamerManager.getStreamers()
	if len(streamers) == 0 {
		return "📋 No streamers in the notification list.\n\nUse /add <username> to add streamers!"
	}

	responseText := "📋 Current streamers:\n\n"

	for _, streamer := range streamers {
		status := map[bool]string{true: "🔴", false: "⚫"}[streamer.IsLive]
		responseText += fmt.Sprintf("%s %s (%s)\n", status, streamer.DisplayName, streamer.Username)
	}

	responseText += fmt.Sprintf("\n📊 Total: %d streamers", len(streamers))

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

		var streamData *TwitchStreamData
		isLive := streamInfo != nil && len(streamInfo.Data) > 0
		if isLive {
			streamData = &streamInfo.Data[0]
		}

		if streamData != nil {
			responseText += fmt.Sprintf("🔴 %s is LIVE!\n", streamer.DisplayName)
			responseText += fmt.Sprintf("   📺 %s\n", streamData.Title)
			responseText += fmt.Sprintf("   🎮 %s\n", streamData.GameName)
			responseText += fmt.Sprintf("   👥 %d viewers\n\n", streamData.ViewerCount)
		} else {
			responseText += fmt.Sprintf("⚫ %s is offline\n", streamer.DisplayName)
		}

		if err := app.checkAndUpdateStreamerStatus(streamer, streamData, false); err != nil {
			log.Printf("Error updating streamer status for %s: %v", streamer.Username, err)
		}
	}

	responseText += "\n💡 This command manually checks current status and updates the bot's internal state."
	return responseText
}

func (app *App) getHelpText() string {
	return fmt.Sprintf(`🤖 Twitch Notification Bot Commands:

/add <username> - Add a Twitch streamer to notifications
/remove <username> - Remove a streamer from notifications  
/list - Show all tracked streamers with live status
/check - Check current live status and update internal state
/help - Show this help message

🔄 Polling System:
• All streamers monitored via polling (~%ds delay)
• Reliable notification delivery
• No setup required

Examples:
/add ninja              # Add ninja to notifications
/add shroud            # Add shroud to notifications  
/list                  # View all streamers
/remove ninja          # Remove ninja`,
		int(app.config.PollingInterval.Seconds()))
}

func (app *App) findStreamerByUsername(username string) *Streamer {
	streamers := app.streamerManager.getStreamers()
	for _, s := range streamers {
		if s.Username == username {
			return s
		}
	}
	return nil
}

func validateUsernameArg(args string) (string, error) {
	if args == "" {
		return "", fmt.Errorf("usage: /add <twitch_username>")
	}
	return strings.ToLower(strings.TrimSpace(args)), nil
}

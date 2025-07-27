package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

func (app *App) startPollingManager() {
	log.Printf("Starting polling manager with interval: %v", app.config.PollingInterval)

	app.pollingTicker = time.NewTicker(app.config.PollingInterval)

	go func() {
		defer app.pollingTicker.Stop()

		for {
			select {
			case <-app.ctx.Done():
				log.Println("Polling manager stopping...")
				return
			case <-app.pollingTicker.C:
				if err := app.pollStreamStatus(); err != nil {
					log.Printf("Error during polling: %v", err)
				}
			}
		}
	}()
}

func (app *App) stopPollingManager() {
	app.pollingMutex.Lock()
	defer app.pollingMutex.Unlock()

	if app.pollingTicker != nil {
		app.pollingTicker.Stop()
		app.pollingTicker = nil
	}
}

func (app *App) pollStreamStatus() error {
	pollingStreamers := app.streamerManager.getPollingStreamers()
	if len(pollingStreamers) == 0 {
		return nil
	}

	//log.Printf("Polling %d streamers for status updates", len(pollingStreamers))

	batchSize := 100
	for i := 0; i < len(pollingStreamers); i += batchSize {
		end := i + batchSize
		if end > len(pollingStreamers) {
			end = len(pollingStreamers)
		}

		batch := pollingStreamers[i:end]
		if err := app.pollStreamerBatch(batch); err != nil {
			log.Printf("Error polling batch %d-%d: %v", i, end-1, err)
		}

		if end < len(pollingStreamers) {
			time.Sleep(1 * time.Second)
		}
	}

	return nil
}

func (app *App) pollStreamerBatch(streamers []*Streamer) error {
	if len(streamers) == 0 {
		return nil
	}

	var userLogins []string
	streamerMap := make(map[string]*Streamer)

	for _, streamer := range streamers {
		userLogins = append(userLogins, streamer.Username)
		streamerMap[streamer.Username] = streamer
	}

	liveStreams, err := app.getStreamsInfo(userLogins)
	if err != nil {
		return fmt.Errorf("failed to get streams info: %v", err)
	}

	liveStreamers := make(map[string]bool)
	for _, stream := range liveStreams {
		liveStreamers[stream.UserLogin] = true

		streamer := streamerMap[stream.UserLogin]
		if streamer != nil && !streamer.IsLive {
			log.Printf("Polling detected stream online: %s (%s)", streamer.DisplayName, streamer.Username)

			streamData := &TwitchStreamResponse{
				Data: []struct {
					UserID      string `json:"user_id"`
					UserLogin   string `json:"user_login"`
					UserName    string `json:"user_name"`
					GameName    string `json:"game_name"`
					Title       string `json:"title"`
					ViewerCount int    `json:"viewer_count"`
					StartedAt   string `json:"started_at"`
				}{stream},
			}

			if err := app.sendNotification(streamer, streamData); err != nil {
				log.Printf("Error sending notification for %s: %v", streamer.Username, err)
			}

			app.streamerManager.updateStreamerStatus(streamer.UserID, true)
		}
	}

	for _, streamer := range streamers {
		if streamer.IsLive && !liveStreamers[streamer.Username] {
			log.Printf("Polling detected stream offline: %s (%s)", streamer.DisplayName, streamer.Username)
			app.streamerManager.updateStreamerStatus(streamer.UserID, false)
		}
	}

	return nil
}

func (app *App) getStreamsInfo(userLogins []string) ([]struct {
	UserID      string `json:"user_id"`
	UserLogin   string `json:"user_login"`
	UserName    string `json:"user_name"`
	GameName    string `json:"game_name"`
	Title       string `json:"title"`
	ViewerCount int    `json:"viewer_count"`
	StartedAt   string `json:"started_at"`
}, error) {
	if err := app.getTwitchAppToken(); err != nil {
		return nil, err
	}

	url := "https://api.twitch.tv/helix/streams?user_login=" + strings.Join(userLogins, "&user_login=")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := app.makeTwitchAPIRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var streamResp TwitchStreamResponse
	if err := app.decodeJSONResponse(resp, &streamResp); err != nil {
		return nil, err
	}

	return streamResp.Data, nil
}

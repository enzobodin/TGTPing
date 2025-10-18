package main

import (
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
	streamers := app.streamerManager.getStreamers()
	if len(streamers) == 0 {
		return nil
	}

	batchSize := 100
	for i := 0; i < len(streamers); i += batchSize {
		end := i + batchSize
		if end > len(streamers) {
			end = len(streamers)
		}

		batch := streamers[i:end]
		if err := app.pollStreamerBatch(batch); err != nil {
			log.Printf("Error polling batch %d-%d: %v", i, end-1, err)
		}

		if end < len(streamers) {
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

	liveStreamMap := make(map[string]*TwitchStreamData)
	for i := range liveStreams {
		stream := &liveStreams[i]
		liveStreamMap[stream.UserLogin] = stream
	}

	for _, streamer := range streamers {
		streamData := liveStreamMap[streamer.Username]
		if err := app.checkAndUpdateStreamerStatus(streamer, streamData, true); err != nil {
			log.Printf("Error updating streamer status for %s: %v", streamer.Username, err)
		}
	}

	return nil
}

func (app *App) getStreamsInfo(userLogins []string) ([]TwitchStreamData, error) {
	url := "https://api.twitch.tv/helix/streams?user_login=" + strings.Join(userLogins, "&user_login=")

	var streamResp TwitchStreamResponse
	if err := app.callTwitchAPI(url, &streamResp); err != nil {
		return nil, err
	}

	return streamResp.Data, nil
}

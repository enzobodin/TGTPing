package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (app *App) getTwitchToken() error {
	if time.Now().Before(app.tokenExpiry) {
		return nil
	}

	data := url.Values{}
	data.Set("client_id", app.config.TwitchClientID)
	data.Set("client_secret", app.config.TwitchClientSecret)
	data.Set("grant_type", "client_credentials")

	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://id.twitch.tv/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := app.makeHTTPRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var tokenResp TwitchTokenResponse
	if err := app.decodeJSONResponse(resp, &tokenResp); err != nil {
		return err
	}

	app.twitchToken = tokenResp.AccessToken
	app.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return nil
}

func (app *App) makeHTTPRequest(req *http.Request) (*http.Response, error) {
	return app.httpClient.Do(req)
}

func (app *App) callTwitchAPI(url string, target interface{}) error {
	if err := app.getTwitchToken(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), DefaultHTTPTimeout)
	defer cancel()

	resp, err := app.makeTwitchAPIRequest(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return app.decodeJSONResponse(resp, target)
}

func (app *App) getTwitchUser(username string) (*Streamer, error) {
	var userResp TwitchUserResponse
	if err := app.callTwitchAPI(fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", username), &userResp); err != nil {
		return nil, err
	}

	if len(userResp.Data) == 0 {
		return nil, fmt.Errorf("user %s not found", username)
	}

	user := userResp.Data[0]
	return &Streamer{
		Username:    user.Login,
		DisplayName: user.DisplayName,
		UserID:      user.ID,
		IsLive:      false,
		LastChecked: time.Now(),
	}, nil
}

func (app *App) getStreamInfo(userID string) (*TwitchStreamResponse, error) {
	var streamResp TwitchStreamResponse
	if err := app.callTwitchAPI(fmt.Sprintf("https://api.twitch.tv/helix/streams?user_id=%s", userID), &streamResp); err != nil {
		return nil, err
	}

	if len(streamResp.Data) == 0 {
		return nil, nil
	}
	return &streamResp, nil
}

func (app *App) makeTwitchAPIRequest(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", app.config.TwitchClientID)
	req.Header.Set("Authorization", "Bearer "+app.twitchToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return app.makeHTTPRequest(req)
}

func (app *App) decodeJSONResponse(resp *http.Response, target interface{}) error {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, target)
}

func (app *App) checkAndUpdateStreamerStatus(streamer *Streamer, streamData *TwitchStreamData, sendNotification bool) error {
	isCurrentlyLive := streamData != nil

	if isCurrentlyLive && !streamer.IsLive {
		log.Printf("Stream detected online: %s (%s)", streamer.DisplayName, streamer.Username)

		if sendNotification {
			streamResp := &TwitchStreamResponse{
				Data: []TwitchStreamData{*streamData},
			}
			if err := app.sendNotification(streamer, streamResp); err != nil {
				log.Printf("Error sending notification for %s: %v", streamer.Username, err)
			}
		}

		return app.streamerManager.updateStreamerStatus(streamer.UserID, true)
	}

	if !isCurrentlyLive && streamer.IsLive {
		log.Printf("Stream detected offline: %s (%s)", streamer.DisplayName, streamer.Username)
		return app.streamerManager.updateStreamerStatus(streamer.UserID, false)
	}

	return nil
}

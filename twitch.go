package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func (app *App) saveUserToken() error {
	if app.twitchUserToken == "" {
		return nil
	}

	tokenData := TokenData{
		UserAccessToken: app.twitchUserToken,
		ExpiresAt:       app.userTokenExpiry,
	}

	data, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile("/data/token.json", data, 0600)
}

func (app *App) loadUserToken() error {
	data, err := os.ReadFile("/data/token.json")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var tokenData TokenData
	if err := json.Unmarshal(data, &tokenData); err != nil {
		return err
	}

	if time.Now().After(tokenData.ExpiresAt) {
		return nil
	}

	app.twitchUserToken = tokenData.UserAccessToken
	app.userTokenExpiry = tokenData.ExpiresAt
	return nil
}

func (app *App) getTwitchToken() error {
	if time.Now().Before(app.tokenExpiry) {
		return nil
	}

	data := url.Values{}
	data.Set("client_id", app.config.TwitchClientID)
	data.Set("client_secret", app.config.TwitchClientSecret)
	data.Set("grant_type", "client_credentials")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://id.twitch.tv/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tokenResp TwitchTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}

	app.twitchToken = tokenResp.AccessToken
	app.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return nil
}

func (app *App) getTwitchUserToken() error {
	if app.twitchUserToken == "" {
		if err := app.loadUserToken(); err != nil {
			return err
		}
	}

	if time.Now().Before(app.userTokenExpiry) && app.twitchUserToken != "" {
		return nil
	}

	if app.twitchUserToken == "" {
		return fmt.Errorf("no user access token available - please visit %s to authenticate with Twitch and enable EventSub WebSocket subscriptions", app.config.OAuthCallbackURL)
	}

	return nil
}

func (app *App) getTwitchUser(username string) (*Streamer, error) {
	if err := app.getTwitchToken(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.twitch.tv/helix/users?login=%s", username)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", app.config.TwitchClientID)
	req.Header.Set("Authorization", "Bearer "+app.twitchToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userResp TwitchUserResponse
	if err := json.Unmarshal(body, &userResp); err != nil {
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
	if err := app.getTwitchToken(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.twitch.tv/helix/streams?user_id=%s", userID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Client-ID", app.config.TwitchClientID)
	req.Header.Set("Authorization", "Bearer "+app.twitchToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var streamResp TwitchStreamResponse
	if err := json.Unmarshal(body, &streamResp); err != nil {
		return nil, err
	}

	if len(streamResp.Data) > 0 {
		return &streamResp, nil
	}

	return nil, nil
}

func (app *App) getTwitchAppToken() error {
	return app.getTwitchToken()
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

	client := &http.Client{}
	return client.Do(req)
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

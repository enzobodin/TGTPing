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

func (app *App) oauthHandler(w http.ResponseWriter, r *http.Request) {
	streamers := app.streamerManager.getStreamers()
	streamerCount := len(streamers)

	wsStatus := "Disconnected"
	if app.wsConn != nil && app.sessionID != "" {
		wsStatus = "Connected"
	}

	code := r.URL.Query().Get("code")
	if code != "" {
		if err := app.handleOAuthCallback(code); err != nil {
			log.Printf("OAuth callback error: %v", err)
			http.Error(w, "OAuth callback failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	needsAuth := app.twitchUserToken == "" || time.Now().After(app.userTokenExpiry)
	authURL := ""
	if needsAuth {
		authURL = fmt.Sprintf("https://id.twitch.tv/oauth2/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=user:read:email",
			app.config.TwitchClientID,
			url.QueryEscape(app.config.OAuthCallbackURL+"/oauth/callback"))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	authSection := ""
	if needsAuth {
		authSection = fmt.Sprintf(`
		<div class="status">
			<h2>🔐 Authentication Required</h2>
			<p class="disconnected">❌ No user access token available</p>
			<p>EventSub WebSocket subscriptions require a user access token.</p>
			<a href="%s" style="display: inline-block; padding: 10px 20px; background: #9146ff; color: white; text-decoration: none; border-radius: 5px; margin: 10px 0;">Authorize with Twitch</a>
			<p><small>This will redirect you to Twitch to authorize the application.</small></p>
		</div>`, authURL)
	} else {
		authSection = `
		<div class="status">
			<h2>🔐 Authentication Status</h2>
			<p class="connected">✅ User access token available</p>
			<p>EventSub WebSocket subscriptions are enabled.</p>
		</div>`
	}

	w.Write([]byte(fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Twitch Notification Bot</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: #0f0f23; color: #fff; }
        .container { max-width: 600px; margin: 0 auto; }
        h1 { color: #9146ff; }
        .status { padding: 20px; background: #1f1f35; border-radius: 10px; margin: 20px 0; }
        .live { color: #00f5ff; }
        .connected { color: #00ff00; }
        .disconnected { color: #ff0000; }
        a { color: #9146ff; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🤖 Twitch Notification Bot</h1>
        %s
        <div class="status">
            <h2>📡 EventSub WebSocket Status</h2>
            <p class="%s">%s %s</p>
            <p>Session ID: <code>%s</code></p>
        </div>
        <div class="status">
            <h2>📊 Statistics</h2>
            <p>Tracked Streamers: <strong>%d</strong></p>
            <p>Subscriptions: <strong>%d</strong></p>
        </div>
        <p><strong>Features:</strong></p>
        <ul style="text-align: left;">
            <li>🔴 Instant stream online notifications via EventSub</li>
            <li>📊 Rich stream information (title, game, viewers)</li>
            <li>💬 Telegram bot commands (/add, /remove, /list)</li>
            <li>📁 Persistent Docker storage</li>
            <li>🔄 Auto-reconnection and error recovery</li>
            <li>🔐 OAuth user authentication for WebSocket subscriptions</li>
        </ul>
    </div>
</body>
</html>`,
		authSection,
		func() string {
			if wsStatus == "Connected" {
				return "connected"
			} else {
				return "disconnected"
			}
		}(),
		func() string {
			if wsStatus == "Connected" {
				return "✅"
			} else {
				return "❌"
			}
		}(),
		wsStatus,
		app.sessionID,
		streamerCount,
		len(app.subscriptions))))
}

func (app *App) handleOAuthCallback(code string) error {
	data := url.Values{}
	data.Set("client_id", app.config.TwitchClientID)
	data.Set("client_secret", app.config.TwitchClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", app.config.OAuthCallbackURL+"/oauth/callback")

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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OAuth token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TwitchTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return err
	}

	app.twitchUserToken = tokenResp.AccessToken
	app.userTokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	if err := app.saveUserToken(); err != nil {
		log.Printf("Warning: Failed to save user token: %v", err)
	}

	log.Println("Successfully obtained user access token via OAuth")

	if app.wsConn == nil {
		if err := app.connectWebSocket(); err != nil {
			log.Printf("Failed to connect to EventSub WebSocket after OAuth: %v", err)
		} else {
			go app.handleWebSocketMessages()
		}
	} else if app.sessionID != "" {
		go app.subscribeToExistingStreamers()
	}

	return nil
}

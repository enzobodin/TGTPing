# TGTping

A Golang application that sends Telegram notifications when Twitch streamers go live, using a hybrid notification system combining real-time EventSub WebSocket and API polling.

> **⚠️ Disclaimer:** This project was created with the assistance of AI tools.

## Features

- 🚀 **Hybrid notification system** - Real-time for priority streamers, polling for others
- ⚡ **Priority management** - Set high-priority streamers for instant notifications
- 🔴 **Real-time notifications** via Twitch EventSub WebSocket (~2-3s delay)
- 🔄 **Polling notifications** for normal-priority streamers (~90s delay)
- 📊 **Rich stream information** (title, game, viewer count)
- 💬 **Enhanced Telegram bot commands** (/add, /remove, /list, /priority, /status, /help)
- 🔐 **OAuth authentication** for user access tokens
- 🔄 **Auto-reconnection** and error recovery
- 📦 **Docker containerization** for easy deployment

## File Structure

The codebase is organized into logical modules for better maintainability:

- **`main.go`** - Application initialization and startup
- **`types.go`** - All struct definitions and type declarations
- **`config.go`** - Configuration loading and environment setup
- **`streamer.go`** - StreamerManager operations and file persistence
- **`twitch.go`** - Twitch API interactions and token management
- **`websocket.go`** - EventSub WebSocket handling and subscriptions
- **`telegram.go`** - Telegram bot commands and message handling
- **`oauth.go`** - OAuth flow and web interface
- **`polling.go`** - API polling system for normal-priority streamers

## Hybrid Notification System

The bot uses a smart hybrid approach to handle Twitch's EventSub limits:

### 🌐 Real-time WebSocket (High Priority)

- **Limit**: 5 streamers maximum (cost limit of 10)
- **Delay**: ~2-3 seconds
- **Usage**: Priority streamers set with `/priority <username> high`
- **Each streamer costs 2 (stream.online + stream.offline subscriptions)**

### 🔄 API Polling (Normal Priority)

- **Limit**: Unlimited streamers
- **Delay**: ~90 seconds (configurable)
- **Usage**: All other streamers
- **Rate limited**: Respects Twitch API limits (800 requests/minute)
- **Batched**: Up to 100 streamers per API call

## Important: OAuth Authentication Required

EventSub WebSocket subscriptions require **user access tokens**, not app access tokens. The bot includes a built-in OAuth flow:

1. **Visit the web interface** at your configured callback URL
2. **Click "Authorize with Twitch"** when prompted
3. **Complete the Twitch OAuth flow**
4. **Bot automatically connects to EventSub and starts real-time subscriptions**

**Connection Behavior:**

- **Without OAuth**: Bot runs without WebSocket connection (no real-time notifications)
- **With OAuth**: Bot automatically connects to EventSub WebSocket and creates subscriptions
- **After OAuth**: Existing running bot will immediately connect and enable real-time notifications

**Without OAuth authentication:**

- Bot commands will work (/add, /remove, /list, /check)
- Streamer lookup via Twitch API will work
- Real-time EventSub notifications will be **disabled**

## Quick Start

1. **Clone and setup:**

   ```bash
   git clone <repository>
   cd TGTping
   cp .env.example .env
   ```

2. **Configure environment variables in `.env`:**

   ```env
   TWITCH_CLIENT_ID=your_twitch_client_id
   TWITCH_CLIENT_SECRET=your_twitch_client_secret
   TELEGRAM_BOT_TOKEN=your_telegram_bot_token
   TELEGRAM_CHAT_ID=your_telegram_chat_id
   OAUTH_CALLBACK_URL=https://your-domain.com
   ```

3. **Run with Docker:**

   ```bash
   docker compose up -d
   ```

4. **Authenticate for EventSub:**
   - Visit your configured OAuth callback URL
   - Click "Authorize with Twitch" to enable real-time notifications

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `TWITCH_CLIENT_ID` | Your Twitch application client ID | Yes | - |
| `TWITCH_CLIENT_SECRET` | Your Twitch application client secret | Yes | - |
| `TELEGRAM_BOT_TOKEN` | Your Telegram bot token from @BotFather | Yes | - |
| `TELEGRAM_CHAT_ID` | The chat ID where notifications will be sent | Yes | - |
| `OAUTH_CALLBACK_URL` | Base URL for OAuth callbacks | No | - |
| `MAX_WEBSOCKET_STREAMERS` | Maximum high-priority streamers | No | 5 |
| `POLLING_INTERVAL_SECONDS` | Polling interval for normal streamers | No | 90 |

## How to Get Credentials

### Twitch Credentials

1. Go to [Twitch Developers Console](https://dev.twitch.tv/console)
2. Create a new application
3. Set OAuth Redirect URL to: `{YOUR_OAUTH_CALLBACK_URL}/oauth/callback`
4. Copy the Client ID and Client Secret

### Telegram Bot

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Use `/newbot` command to create a new bot
3. Copy the bot token
4. Add the bot to your chat and get the chat ID

### Get Chat ID

Send a message to your bot, then visit:

```text
https://api.telegram.org/bot<YourBotToken>/getUpdates
```

## Telegram Commands

- `/add <username>` - Add a Twitch streamer to notifications (starts as normal priority)
- `/remove <username>` - Remove a streamer from notifications
- `/list` - Show all tracked streamers with live status and notification modes
- `/priority <username> <high|normal>` - Set streamer priority (high = real-time, normal = polling)
- `/status` - Show notification system status and configuration
- `/check` - Check current live status and update internal state
- `/help` - Show help message with system information

### Priority Examples

```text
/add ninja              # Add ninja with normal priority (polling)
/priority ninja high    # Upgrade ninja to high priority (real-time)
/add shroud            # Add shroud with normal priority
/status                # View current system status
/list                  # View all streamers with their modes
```

## Technical Details

### Hybrid Notification Flow

#### WebSocket Mode (High Priority)

1. Connect to `wss://eventsub.wss.twitch.tv/ws`
2. Receive welcome message with session ID
3. Create subscriptions for `stream.online` and `stream.offline` events (only for high-priority streamers)
4. Receive real-time notifications when priority streamers go live/offline

#### Polling Mode (Normal Priority)

1. Periodic API calls to Twitch Streams endpoint
2. Batch up to 100 streamers per request
3. Compare current status with stored status
4. Send notifications for status changes
5. Rate limiting to respect Twitch API limits

### System Behavior

- **Automatic Mode Assignment**: High-priority streamers use WebSocket until the limit (5) is reached
- **Graceful Fallback**: If WebSocket fails, all streamers temporarily use polling
- **Dynamic Switching**: Priority changes immediately reassign notification modes
- **Efficient Batching**: Polling system batches requests to minimize API usage

### OAuth Flow

1. User visits web interface at configured callback URL
2. Redirected to Twitch OAuth with required scopes
3. Twitch redirects back with authorization code
4. Bot exchanges code for user access token
5. EventSub subscriptions are created with user token

### Data Persistence

- Streamer data is stored in `/data/streamers.json`
- OAuth tokens are stored in `/data/token.json`
- Docker volume ensures data persists across container restarts

## Docker Usage

### Build and Run

```bash
docker build -t tgtping .
docker run -d \
  --name tgtping \
  -p 8085:8080 \
  -v tgtping-data:/data \
  --env-file .env \
  tgtping
```

### Docker Compose

```bash
docker compose up -d
```

### Logs

```bash
docker logs -f tgtping
```

## Architecture

The application follows a clean modular architecture:

1. **Configuration Layer** (`config.go`) - Environment and settings
2. **Data Layer** (`types.go`, `streamer.go`) - Data structures and persistence
3. **External APIs** (`twitch.go`, `oauth.go`) - Third-party integrations
4. **Real-time Layer** (`websocket.go`) - EventSub WebSocket handling
5. **Interface Layer** (`telegram.go`) - User interaction
6. **Application Layer** (`main.go`) - Initialization and coordination

## Contributing

1. Fork the repository
2. Create a feature branch
3. Follow the code style guidelines in `AGENTS.md`
4. Test thoroughly with Docker
5. Submit a pull request

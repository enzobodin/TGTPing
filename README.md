# TGTping

A Golang application that sends Telegram notifications when Twitch streamers go live using reliable API polling.

> **⚠️ Disclaimer:** This project was created with the assistance of AI tools.

## Features

- 🔄 **Reliable polling system** - Consistent notifications via Twitch API
- 📊 **Rich stream information** (title, game, viewer count)
- 💬 **Telegram bot commands** (/add, /remove, /list, /check, /help)
- 🔄 **Auto-recovery** and error handling
- 📦 **Docker containerization** for easy deployment
- ⚡ **Efficient batching** - Up to 100 streamers per API call

## File Structure

The codebase is organized into logical modules for better maintainability:

- **`config.go`** - Configuration loading and environment setup
- **`types.go`** - All struct definitions and type declarations
- **`streamer.go`** - StreamerManager operations and file persistence
- **`twitch.go`** - Twitch API interactions and app token management
- **`polling.go`** - Polling-based stream monitoring and notifications
- **`telegram.go`** - Telegram bot commands and message handling
- **`main.go`** - Application initialization and startup

## Notification System

The bot uses a simple, reliable polling approach:

### 🔄 API Polling

- **Limit**: Unlimited streamers
- **Delay**: ~90 seconds (configurable)
- **Rate limited**: Respects Twitch API limits (800 requests/minute)
- **Batched**: Up to 100 streamers per API call
- **Reliable**: No complex WebSocket connections to maintain

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
   ```

3. **Run with Docker:**

   ```bash
   docker compose up -d
   ```

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `TWITCH_CLIENT_ID` | Your Twitch application client ID | Yes | - |
| `TWITCH_CLIENT_SECRET` | Your Twitch application client secret | Yes | - |
| `TELEGRAM_BOT_TOKEN` | Your Telegram bot token from @BotFather | Yes | - |
| `TELEGRAM_CHAT_ID` | The chat ID where notifications will be sent | Yes | - |
| `POLLING_INTERVAL_SECONDS` | Polling interval for checking streams | No | 90 |

## How to Get Credentials

### Twitch Credentials

1. Go to [Twitch Developers Console](https://dev.twitch.tv/console)
2. Create a new application
3. Copy the Client ID and Client Secret

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

- `/add <username>` - Add a Twitch streamer to notifications
- `/remove <username>` - Remove a streamer from notifications
- `/list` - Show all tracked streamers with live status
- `/check` - Check current live status and update internal state
- `/help` - Show help message

### Usage Examples

```text
/add ninja              # Add ninja to notifications
/add shroud            # Add shroud to notifications  
/list                  # View all streamers
/remove ninja          # Remove ninja
```

## Technical Details

### Polling Flow

1. Periodic API calls to Twitch Streams endpoint every 90 seconds (configurable)
2. Batch up to 100 streamers per request
3. Compare current status with stored status
4. Send notifications for status changes
5. Rate limiting to respect Twitch API limits

### System Behavior

- **Efficient Batching**: Polling system batches requests to minimize API usage
- **Status Tracking**: Maintains accurate live/offline status for each streamer
- **Error Recovery**: Graceful handling of API failures and network issues

### Data Persistence

- Streamer data is stored in `/data/streamers.json`
- Docker volume ensures data persists across container restarts

## Docker Usage

### Build and Run

```bash
docker build -t tgtping .
docker run -d \
  --name tgtping \
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
3. **External APIs** (`twitch.go`) - Twitch API integration
4. **Polling Layer** (`polling.go`) - Stream monitoring
5. **Interface Layer** (`telegram.go`) - User interaction
6. **Application Layer** (`main.go`) - Initialization and coordination

## Contributing

1. Fork the repository
2. Create a feature branch
3. Follow the code style guidelines in `AGENTS.md`
4. Test thoroughly with Docker
5. Submit a pull request

# AGENTS.md - Development Guidelines

## Build/Test Commands
- **Build**: `go build -o main .`
- **Run locally**: `go run main.go`
- **Test**: No test framework configured - manual testing via Docker
- **Docker build**: `docker build -t tgtping .`
- **Docker run**: `docker compose up -d`
- **Format**: `go fmt ./...`
- **Modules**: `go mod tidy`

## Environment Variables
- **TWITCH_CLIENT_ID**: Twitch application client ID
- **TWITCH_CLIENT_SECRET**: Twitch application client secret
- **TELEGRAM_BOT_TOKEN**: Telegram bot token from @BotFather
- **TELEGRAM_CHAT_ID**: Telegram chat ID for notifications
- **POLLING_INTERVAL_SECONDS**: Optional polling interval (default: 90s, minimum: 30s)

## File Structure
- **config.go**: Configuration loading and environment setup
- **types.go**: All struct definitions and type declarations
- **streamer.go**: StreamerManager operations and file persistence
- **twitch.go**: Twitch API interactions and app token management
- **polling.go**: Polling-based stream monitoring and notifications
- **telegram.go**: Telegram bot commands and message handling
- **main.go**: Application initialization and startup

## Architecture
TGTPing uses a **polling-only architecture** for reliable stream monitoring:
- **No WebSocket connections** - eliminates connection management complexity
- **App token authentication** - uses client credentials flow only
- **Batch API requests** - polls multiple streamers efficiently
- **Simple state management** - streamers stored in JSON file

## Code Style Guidelines

### Imports
- Standard library imports first, external packages second, separated by blank line
- Use explicit package names, avoid dot imports

### Types & Naming
- Struct fields use PascalCase with JSON tags in snake_case
- Variables use camelCase
- Constants use PascalCase
- Prefer descriptive names: `TwitchTokenResponse` over `TokenResp`

### Error Handling
- Always check and handle errors explicitly
- Use fmt.Errorf for error wrapping
- Log errors before returning: `log.Printf("Error: %v", err)`
- Return early on errors to reduce nesting

### Concurrency
- Use mutexes for shared data (RWMutex for read-heavy operations)
- Defer mutex unlocks immediately after lock
- Use context.Context for cancellation and timeouts
- Separate goroutines for independent operations

### HTTP & JSON
- Use context.WithTimeout for HTTP requests (10s standard)
- Defer response.Body.Close() after error checks
- Use json.Marshal/Unmarshal with proper struct tags
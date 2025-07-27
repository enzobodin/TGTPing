package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

func NewStreamerManager(filename string) *StreamerManager {
	sm := &StreamerManager{
		streamers: make(map[string]*Streamer),
		filename:  filename,
	}
	sm.loadFromFile()
	return sm
}

func (sm *StreamerManager) loadFromFile() {
	data, err := os.ReadFile(sm.filename)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("Streamers file does not exist, starting with empty list")
			return
		}
		log.Printf("Error reading streamers file: %v", err)
		return
	}

	var streamers []Streamer
	if err := json.Unmarshal(data, &streamers); err != nil {
		log.Printf("Error unmarshalling streamers: %v", err)
		return
	}

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	seenUserIDs := make(map[string]bool)
	for _, streamer := range streamers {
		if seenUserIDs[streamer.UserID] {
			continue
		}
		seenUserIDs[streamer.UserID] = true

		if streamer.Priority == "" {
			streamer.Priority = "normal"
		}
		if streamer.NotificationMode == "" {
			streamer.NotificationMode = "polling"
		}

		streamerCopy := streamer
		sm.streamers[streamerCopy.Username] = &streamerCopy
	}

	if len(streamers) != len(sm.streamers) {
		sm.saveToFile()
	}
}

func (sm *StreamerManager) saveToFile() error {
	streamers := make([]Streamer, 0, len(sm.streamers))
	for _, streamer := range sm.streamers {
		streamers = append(streamers, *streamer)
	}

	data, err := json.MarshalIndent(streamers, "", "  ")
	if err != nil {
		log.Printf("Error marshaling JSON: %v", err)
		return err
	}

	err = os.WriteFile(sm.filename, data, 0644)
	if err != nil {
		log.Printf("Error writing file %s: %v", sm.filename, err)
		return err
	}

	return nil
}

func (sm *StreamerManager) addStreamer(streamer *Streamer) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if streamer.Priority == "" {
		streamer.Priority = "normal"
	}
	if streamer.NotificationMode == "" {
		streamer.NotificationMode = "polling"
	}

	sm.streamers[streamer.Username] = streamer
	err := sm.saveToFile()
	if err != nil {
		log.Printf("Error saving file for streamer %s: %v", streamer.Username, err)
		return err
	}

	return nil
}

func (sm *StreamerManager) removeStreamer(username string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.streamers, username)
	err := sm.saveToFile()
	if err != nil {
		log.Printf("Error saving file after removing %s: %v", username, err)
		return err
	}

	return nil
}

func (sm *StreamerManager) getStreamers() []*Streamer {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	streamers := make([]*Streamer, 0, len(sm.streamers))
	for _, streamer := range sm.streamers {
		streamers = append(streamers, streamer)
	}
	return streamers
}

func (sm *StreamerManager) getStreamerByUserID(userID string) *Streamer {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	for _, streamer := range sm.streamers {
		if streamer.UserID == userID {
			return streamer
		}
	}
	return nil
}

func (sm *StreamerManager) updateStreamerStatus(userID string, isLive bool) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	for _, streamer := range sm.streamers {
		if streamer.UserID == userID {
			streamer.IsLive = isLive
			streamer.LastChecked = time.Now()
			return sm.saveToFile()
		}
	}
	return fmt.Errorf("streamer with userID %s not found", userID)
}

func (sm *StreamerManager) setStreamerPriority(username string, priority string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	streamer, exists := sm.streamers[username]
	if !exists {
		return fmt.Errorf("streamer %s not found", username)
	}

	streamer.Priority = priority
	return sm.saveToFile()
}

func (sm *StreamerManager) assignNotificationModes(maxWebSocketStreamers int) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	highPriorityCount := 0
	currentWebSocketCount := 0
	webSocketWithNormalPriority := 0
	highPriorityInPolling := 0

	for _, streamer := range sm.streamers {
		if streamer.Priority == "high" {
			highPriorityCount++
			if streamer.NotificationMode == "polling" {
				highPriorityInPolling++
			}
		}
		if streamer.NotificationMode == "websocket" {
			currentWebSocketCount++
			if streamer.Priority != "high" {
				webSocketWithNormalPriority++
			}
		}
	}

	needsReassignment := currentWebSocketCount > maxWebSocketStreamers ||
		webSocketWithNormalPriority > 0 ||
		(highPriorityInPolling > 0 && currentWebSocketCount < maxWebSocketStreamers)
	if !needsReassignment {
		return nil
	}

	for _, streamer := range sm.streamers {
		streamer.NotificationMode = "polling"
	}

	webSocketSlots := maxWebSocketStreamers
	for _, streamer := range sm.streamers {
		if streamer.Priority == "high" && webSocketSlots > 0 {
			streamer.NotificationMode = "websocket"
			webSocketSlots--
		}
	}

	return sm.saveToFile()
}

func (sm *StreamerManager) getWebSocketStreamers() []*Streamer {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	var webSocketStreamers []*Streamer
	for _, streamer := range sm.streamers {
		if streamer.NotificationMode == "websocket" {
			webSocketStreamers = append(webSocketStreamers, streamer)
		}
	}
	return webSocketStreamers
}

func (sm *StreamerManager) getPollingStreamers() []*Streamer {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	var pollingStreamers []*Streamer
	for _, streamer := range sm.streamers {
		if streamer.NotificationMode == "polling" {
			pollingStreamers = append(pollingStreamers, streamer)
		}
	}
	return pollingStreamers
}

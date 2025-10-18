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

		streamerCopy := streamer
		sm.streamers[streamerCopy.Username] = &streamerCopy
	}

	if len(streamers) != len(sm.streamers) {
		if err := sm.saveToFile(); err != nil {
			log.Printf("Error saving streamers to file: %v", err)
		}
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

	sm.streamers[streamer.Username] = streamer
	return sm.saveToFileWithLog(streamer.Username, "saving file for streamer")
}

func (sm *StreamerManager) removeStreamer(username string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	delete(sm.streamers, username)
	return sm.saveToFileWithLog(username, "saving file after removing")
}

func (sm *StreamerManager) saveToFileWithLog(context, action string) error {
	if err := sm.saveToFile(); err != nil {
		log.Printf("Error %s %s: %v", action, context, err)
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

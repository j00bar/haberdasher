package logging

import (
	"encoding/json"
	"log"
	"time"
	"os"
)

var defaultTags []string
var defaultLabels map[string]string
const defaultEcsVersion = "1.5.0"

// If the wrapped application emits plain text messages, we should wrap them
// ourselves in an ECS compatible envelope. The environment variables
// HABERDASHER_TAGS and HABERDASHER_LABELS contain serialized JSON values for
// the tags and labels to go in such messages. They are optional.
func init() {
	tagsFromEnv, exists := os.LookupEnv("HABERDASHER_TAGS")
	if !exists {
		tagsFromEnv = "[]"
	}
	labelsFromEnv, exists := os.LookupEnv("HABERDASHER_LABELS")
	if !exists {
		labelsFromEnv = "{}"
	}
	err := json.Unmarshal([]byte(tagsFromEnv), &defaultTags)
	if err != nil {
		log.Fatal("HABERDASHER_TAGS must be a JSON array of strings")
	}
	err = json.Unmarshal([]byte(labelsFromEnv), &defaultLabels)
	if err != nil {
		log.Fatal("HABERDASHER_LABELS must be a JSON object of strings")
	}
}

// An Emitter defines how to ship a log message to a log service.
type Emitter interface {
	Setup()
	HandleLogMessage(jsonBytes []byte) (error)
	Cleanup() (error)
}

// A Message is a structured log message - only used if the log message we
// consume from the subprocess is not already structured
type Message struct {
	ECSVersion string `json:"ecs.version"`
	Timestamp time.Time `json:"@timestamp"`
	Labels map[string]string `json:"labels"`
	Tags []string `json:"tags"`
	Message string `json:"message"`
}

// Emitters is the registry of Emitter implementers
var Emitters = make(map[string]Emitter)

// Register will make note of new types of Emitters
func Register(emitterType string, emitter Emitter) {
	Emitters[emitterType] = emitter
}

// Emit is launched as a goroutine for individual log lines to be sent
// concurrently. When it receives a line, it tries to decode it from JSON.
// If that succeeds, meaning it's already a structured object, we pass it along
// unmodified. If not, we wrap it in a basic ECS structure.
func Emit(emitter Emitter, logMessage string) {
	// If the emitted message is JSON, pass it along unmodified
	var decodedJSON map[string]interface{}
	messageToEmit := []byte(logMessage)
	if err := json.Unmarshal(messageToEmit, &decodedJSON); err != nil {
		m := Message{defaultEcsVersion, time.Now(), defaultLabels, defaultTags, logMessage}
		messageToEmit, _ = json.Marshal(m)
	}
	if err := emitter.HandleLogMessage(messageToEmit); err != nil {
		log.Println("Error emitting message:", messageToEmit, err)
	}
}
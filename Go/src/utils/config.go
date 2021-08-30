package utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Config struct {
	Token           	string  // Bot API token
	API                 API     // See API struct
	Owner           	int     // Owner of the bot: skips logging
	ConversionRate      int     // Rate-limit for conversions per hour
	StatConverted   	int     // Keep track of converted images
	StatUniqueChats 	int     // Keep track of count of unique chats
	StatStarted     	int64   // Unix timestamp of startup time
	UniqueUsers     	[]int   // List of all unique chats
}

type API struct {
	LocalAPIEnabled     bool    // Is the local API in use?
	CloudAPILoggedOut   bool    // Logged out from the cloud API?
	LocalWorkingDir     string  // Local working directory
	URL                 string  // API endpoint URL
}

func DumpConfig(config *Config) {
	jsonbytes, err := json.MarshalIndent(*config, "", "\t")
	if err != nil {
		log.Printf("⚠️ Error marshaling json! Err: %s\n", err)
	}

	wd, _ := os.Getwd()
	configf := filepath.Join(wd, "config", "botConfig.json")

	file, err := os.Create(configf)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Write, close
	file.Write(jsonbytes)
	file.Close()
}

func LoadConfig() Config {
	// Get log file's path relative to working dir
	wd, _ := os.Getwd()
	configPath := filepath.Join(wd, "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = os.Mkdir(configPath, os.ModePerm)
	}

	configf := filepath.Join(configPath, "botConfig.json")
	if _, err := os.Stat(configf); os.IsNotExist(err) {
		// Config doesn't exist: create
		fmt.Print("\nEnter bot token: ")

		reader := bufio.NewReader(os.Stdin)
		inp, _ := reader.ReadString('\n')
		botToken := strings.TrimSuffix(inp, "\n")

		// Create, marshal
		config := Config{
			Token:           botToken,
			API:             API{
				LocalAPIEnabled:   false,
				CloudAPILoggedOut: false,
				LocalWorkingDir:   "working/dir/on/server",
				URL:               "https://api.telegram.org",
			},
			Owner:           0,
			ConversionRate:  100,
			StatConverted:   0,
			StatUniqueChats: 0,
			StatStarted:     time.Now().Unix(),
			UniqueUsers:     []int{},
		}

		go DumpConfig(&config)
		return config
	}

	// Config exists: load
	fbytes, err := ioutil.ReadFile(configf)
	if err != nil {
		log.Println("⚠️ Error reading config file:", err)
		os.Exit(1)
	}

	// New config struct
	var config Config

	// Unmarshal into our config struct
	err = json.Unmarshal(fbytes, &config)
	if err != nil {
		log.Println("⚠️ Error unmarshaling config json: ", err)
		os.Exit(1)
	}

	// Set startup time
	config.StatStarted = time.Now().Unix()
	config.StatUniqueChats = len(config.UniqueUsers)

	// Set rate-limit if it has defaulted to 0
	if config.ConversionRate == 0 {
		config.ConversionRate = 100
	}

	// Sort UniqueChats, as they may be unsorted
	sort.Ints(config.UniqueUsers)

	return config
}
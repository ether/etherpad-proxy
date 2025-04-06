package main

import (
	"encoding/json"
	"github.com/ether/etherpad-proxy/models"
	"go.uber.org/zap"
	"os"
)

const SettingsFile = "SETTINGS_FILE"
const SettingsFileDefault = "settings.json"

func main() {
	var fileNameFromEnv, ok = os.LookupEnv(SettingsFile)
	logger, _ := zap.NewProduction()
	defer logger.Sync() // flushes buffer, if any
	sugar := logger.Sugar()
	if !ok {
		fileNameFromEnv = SettingsFileDefault
	}
	content, err := os.ReadFile(fileNameFromEnv)
	if err != nil {
		sugar.Fatalf("Error reading file: %v", err)
	}

	var settingsData models.Settings
	if err = json.Unmarshal(content, &settingsData); err != nil {
		sugar.Fatalf("Error reading file: %v", err)
	}

	StartServer(settingsData, sugar)
}

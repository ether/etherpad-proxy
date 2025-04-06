package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/ether/etherpad-proxy/models"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	_ "golang.org/x/oauth2/clientcredentials"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var AvailableBackends = models.AvailableBackends{
	Available: []string{},
	Up:        []string{},
	Mutex:     sync.Mutex{},
}

func checkAvailabilityLoop(settings models.Settings, _ *zap.SugaredLogger) {
	var timerP = time.Duration(settings.CheckInterval) * time.Millisecond
	go func() {
		for {
			var response = checkAvailability(settings)
			AvailableBackends.Mutex.Lock()
			AvailableBackends.Available = response.Available
			AvailableBackends.Up = response.Up
			AvailableBackends.Mutex.Unlock()
			time.Sleep(timerP)
		}
	}()
}

func cleanUpEtherpadsLoop(settings models.Settings, logger *zap.SugaredLogger, db DB) {
	var timerP = time.Duration(settings.CheckInterval) * time.Second
	var timerBefore = time.Duration(5) * time.Second
	go func() {
		for {
			time.Sleep(timerBefore)
			cleanUpEtherpads(settings, logger, db)
			time.Sleep(timerP)
		}
	}()
}

func cleanUpEtherpads(settings models.Settings, logger *zap.SugaredLogger, db DB) {
	AvailableBackends.Mutex.Lock()
	defer AvailableBackends.Mutex.Unlock()
	var mapOfPadsToBackends = make(map[string]string)
	for _, backend := range AvailableBackends.Up {
		foundBackend := settings.Backends[backend]
		var authorizationHeader string
		if foundBackend.Username != nil && foundBackend.Password != nil {
			authorizationHeader = "Basic " + base64.StdEncoding.EncodeToString([]byte(*foundBackend.Username+":"+*foundBackend.Password))
		} else if foundBackend.ClientId != nil && foundBackend.ClientSecret != nil {
			conf := &clientcredentials.Config{
				ClientID:     *foundBackend.ClientId,
				ClientSecret: *foundBackend.ClientSecret,
				Scopes:       foundBackend.Scopes,
				TokenURL:     *foundBackend.TokenURL,
				AuthStyle:    oauth2.AuthStyleInHeader,
			}
			token, err := conf.Token(context.Background())
			if err != nil {
				logger.Warnf("Error getting token: %v", err)
				continue
			}
			authorizationHeader = "Bearer " + token.AccessToken
		} else {
			logger.Info("No authentication method found for backend: ", backend)
			continue
		}

		client := &http.Client{}
		req, _ := http.NewRequest("GET", "http://"+foundBackend.Host+":"+strconv.Itoa(foundBackend.Port)+"/api/1.3.0/listAllPads", nil)
		req.Header.Set("Authorization", authorizationHeader)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			logger.Warnf("Error retrieving etherpads: %v from %s", err, backend)
			continue
		}
		bytes, err := io.ReadAll(res.Body)

		if err != nil {
			logger.Warnf("Error reading response body: %v", err)
			continue
		}
		var response models.ListAllPadsModel
		if err = json.Unmarshal(bytes, &response); err != nil {
			logger.Warnf("Error unmarshalling response: %v", err)
			continue
		}

		if response.Code != 0 {
			logger.Warnf("Error retrieving etherpads: %v", response.Message)
			continue
		}

		for _, pad := range response.Data.PadIds {
			if entry, ok := mapOfPadsToBackends[pad]; ok {
				logger.Warnf("Pad %s already exists in the map", entry)
				if err = db.RecordClash(pad, backend); err != nil {
					logger.Warnf("Error recording clash: %v", err)
				}
				continue
			}
			mapOfPadsToBackends[pad] = backend
		}

		if err = res.Body.Close(); err != nil {
			logger.Warnf("Error closing response body: %v", err)
			continue
		}
	}
	var backendToPads = make(map[string][]string)

	for pad, backend := range mapOfPadsToBackends {
		if val, ok := backendToPads[backend]; ok {
			backendToPads[backend] = append(val, pad)
		} else {
			backendToPads[backend] = []string{pad}
		}
	}

	for backend, pads := range backendToPads {
		if err := db.CleanUpPads(pads, backend); err != nil {
			logger.Warnf("Error cleaning up etherpads: %v", err)
		}
	}

	for pad, backend := range mapOfPadsToBackends {
		if err := db.Set(pad, models.DBBackend{Backend: backend}); err != nil {
			logger.Warnf("Error setting etherpad: %v", err)
		}
	}

}

func StartServer(settings models.Settings, logger *zap.SugaredLogger) {
	var backendIds []string
	for key := range settings.Backends {
		backendIds = append(backendIds, key)
	}
	db, err := NewDB(settings.DBSettings.Filename)
	if err != nil {
		logger.Fatalf("Error opening database: %v", err)
	}

	proxies := make(map[string]httputil.ReverseProxy)
	checkAvailabilityLoop(settings, logger)
	cleanUpEtherpadsLoop(settings, logger, *db)
	ScrapeJSFiles(settings)

	for key, backend := range settings.Backends {
		proxyUrl, err := url.Parse("http://" + backend.Host + ":" + strconv.Itoa(backend.Port))
		if err != nil {
			panic(err.Error())
		}
		proxy := httputil.NewSingleHostReverseProxy(proxyUrl)

		proxies[key] = *proxy
	}

	handler := ProxyHandler{
		p:      proxies,
		logger: logger,
		db:     *db,
	}

	http.HandleFunc("/", handler.ServeHTTP)

	go func() {
		const managementPort = 8081
		adminMux := http.NewServeMux()
		adminPanel := AdminPanel{
			DB:     db,
			logger: logger,
		}
		adminMux.Handle("/pads", &adminPanel)

		logger.Info("Starting management server on port ", managementPort)
		if err = http.ListenAndServe(":"+strconv.Itoa(managementPort), adminMux); err != nil {
			logger.Fatalf("Error starting management server: %v", err)
		}
	}()

	logger.Info("Starting server on port ", settings.Port)
	if err = http.ListenAndServe(":"+strconv.Itoa(settings.Port), nil); err != nil {
		logger.Fatalf("Error starting server: %v", err)
	}
}

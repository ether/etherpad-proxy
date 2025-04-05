package main

import (
	"github.com/ether/etherpad-proxy/models"
	"go.uber.org/zap"
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

func StartServer(settings models.Settings, logger *zap.SugaredLogger) {
	var backendIds []string
	for key, _ := range settings.Backends {
		backendIds = append(backendIds, key)
	}

	proxies := make(map[string]httputil.ReverseProxy)
	checkAvailabilityLoop(settings, logger)

	for key, backend := range settings.Backends {
		proxyUrl, err := url.Parse("http://" + backend.Host + ":" + strconv.Itoa(backend.Port))
		if err != nil {
			panic(err.Error())
		}
		proxy := httputil.NewSingleHostReverseProxy(proxyUrl)

		proxies[key] = *proxy
	}
	db, err := NewDB(settings.DBSettings.Filename)
	if err != nil {
		logger.Fatalf("Error opening database: %v", err)
	}

	handler := ProxyHandler{
		p:      proxies,
		logger: logger,
		db:     *db,
	}

	http.HandleFunc("/", handler.ServeHTTP)
	logger.Info("Starting server on port ", settings.Port)
	if err = http.ListenAndServe(":"+strconv.Itoa(settings.Port), nil); err != nil {
		logger.Fatalf("Error starting server: %v", err)
	}
}

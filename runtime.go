package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ether/etherpad-proxy/databases"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/metrics"
	"github.com/ether/etherpad-proxy/models"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const defaultManagementPort = 8081

func checkAvailabilityLoop(settings models.Settings, state *models.BackendState, _ *zap.SugaredLogger) {
	timerP := time.Duration(settings.CheckInterval) * time.Millisecond
	go func() {
		for {
			response := checkAvailability(settings)
			state.SetState(response.Available, response.Up)
			metrics.BackendsAvailable.Set(float64(len(response.Available)))
			metrics.BackendsUp.Set(float64(len(response.Up)))
			time.Sleep(timerP)
		}
	}()
}

func cleanUpEtherpadsLoop(settings models.Settings, logger *zap.SugaredLogger, db interfaces.IDB, state *models.BackendState) {
	timerP := time.Duration(settings.CheckInterval) * time.Second
	timerBefore := 5 * time.Second
	go func() {
		for {
			time.Sleep(timerBefore)
			cleanUpEtherpads(settings, logger, db, state)
			time.Sleep(timerP)
		}
	}()
}

func cleanUpEtherpads(settings models.Settings, logger *zap.SugaredLogger, db interfaces.IDB, state *models.BackendState) {
	upBackends := state.SnapshotUp()
	mapOfPadsToBackends := make(map[string]string)
	for _, backend := range upBackends {
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
			_ = res.Body.Close()
			continue
		}
		var response models.ListAllPadsModel
		if err = json.Unmarshal(bytes, &response); err != nil {
			logger.Warnf("Error unmarshalling response: %v", err)
			_ = res.Body.Close()
			continue
		}
		if response.Code != 0 {
			logger.Warnf("Error retrieving etherpads: %v", response.Message)
			_ = res.Body.Close()
			continue
		}
		for _, pad := range response.Data.PadIds {
			if entry, ok := mapOfPadsToBackends[pad]; ok {
				logger.Warnf("Pad %s already exists on backend %s", pad, entry)
				if err = db.RecordClash(pad, backend); err != nil {
					metrics.DBErrorsTotal.Inc()
					logger.Warnf("Error recording clash: %v", err)
				} else {
					metrics.ClashesTotal.Inc()
				}
				continue
			}
			mapOfPadsToBackends[pad] = backend
		}
		if err = res.Body.Close(); err != nil {
			logger.Warnf("Error closing response body: %v", err)
		}
	}

	backendToPads := make(map[string][]string)
	for pad, backend := range mapOfPadsToBackends {
		backendToPads[backend] = append(backendToPads[backend], pad)
	}
	for backend, pads := range backendToPads {
		if err := db.CleanUpPads(pads, backend); err != nil {
			metrics.DBErrorsTotal.Inc()
			logger.Warnf("Error cleaning up etherpads: %v", err)
		}
	}
	for pad, backend := range mapOfPadsToBackends {
		if err := db.Set(pad, models.DBBackend{Backend: backend}); err != nil {
			metrics.DBErrorsTotal.Inc()
			logger.Warnf("Error setting etherpad: %v", err)
		}
	}
}

func StartServer(settings models.Settings, logger *zap.SugaredLogger) {
	db, err := databases.CreateNewDatabase(settings)
	if err != nil {
		logger.Fatalf("Error opening database: %v", err)
	}

	state := &models.BackendState{}
	static := newStaticResources()

	checkAvailabilityLoop(settings, state, logger)
	cleanUpEtherpadsLoop(settings, logger, db, state)
	ScrapeJSFiles(settings, static, logger)

	proxies := make(map[string]httputil.ReverseProxy)
	for key, backend := range settings.Backends {
		proxyURL, perr := url.Parse("http://" + backend.Host + ":" + strconv.Itoa(backend.Port))
		if perr != nil {
			logger.Fatalf("Error parsing backend URL for %s: %v", key, perr)
		}
		proxies[key] = *httputil.NewSingleHostReverseProxy(proxyURL)
	}

	handler := &ProxyHandler{
		p:      proxies,
		logger: logger,
		db:     db,
		state:  state,
		static: static,
	}

	proxyMux := http.NewServeMux()
	proxyMux.HandleFunc("/", handler.ServeHTTP)
	proxySrv := &http.Server{Addr: ":" + strconv.Itoa(settings.Port), Handler: proxyMux}

	managementPort := settings.ManagementPort
	if managementPort == 0 {
		managementPort = defaultManagementPort
	}
	adminMux := http.NewServeMux()
	adminMux.Handle("/pads", &AdminPanel{DB: db, logger: logger})
	adminMux.Handle("/metrics", promhttp.Handler())
	adminMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	adminMux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if len(state.SnapshotUp()) > 0 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mgmtSrv := &http.Server{Addr: ":" + strconv.Itoa(managementPort), Handler: adminMux}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("Starting management server on port ", managementPort)
		if serr := mgmtSrv.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			logger.Fatalf("Error starting management server: %v", serr)
		}
	}()
	go func() {
		logger.Info("Starting server on port ", settings.Port)
		if serr := proxySrv.ListenAndServe(); serr != nil && !errors.Is(serr, http.ErrServerClosed) {
			logger.Fatalf("Error starting server: %v", serr)
		}
	}()

	<-ctx.Done()
	logger.Info("Shutdown signal received, draining...")
	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if serr := proxySrv.Shutdown(shutdownCtx); serr != nil {
		logger.Warnf("Proxy server shutdown error: %v", serr)
	}
	if serr := mgmtSrv.Shutdown(shutdownCtx); serr != nil {
		logger.Warnf("Management server shutdown error: %v", serr)
	}
	if cerr := db.Close(); cerr != nil {
		logger.Warnf("Database close error: %v", cerr)
	}
	logger.Info("Shutdown complete")
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/ether/etherpad-proxy/models"
	"github.com/ether/etherpad-proxy/ui"
	"go.uber.org/zap"
	"log"
	"net/http"
	"net/http/httputil"
	"slices"
	"strings"
)
import "math/rand/v2"
import _ "github.com/ether/etherpad-proxy/ui"

const PadPrefix = "padId:"

type ProxyHandler struct {
	p      map[string]httputil.ReverseProxy
	logger *zap.SugaredLogger
	db     DB
}

func (ph *ProxyHandler) createRoute(padId *string) (httputil.ReverseProxy, error) {
	if padId == nil {
		AvailableBackends.Mutex.Lock()
		if len(AvailableBackends.Available) == 0 {
			return httputil.ReverseProxy{}, errors.New("no backends available")
		}
		var newBackend = AvailableBackends.Available[rand.IntN(len(AvailableBackends.Available))]
		AvailableBackends.Mutex.Unlock()
		return ph.p[newBackend], nil
	}

	var padRead, err = ph.db.Get(PadPrefix + *padId)
	if len(AvailableBackends.Available) == 0 {
		return httputil.ReverseProxy{}, errors.New("no backends available")
	}
	if errors.Is(err, sql.ErrNoRows) {
		// if no backend is stored for this pad, create a new connection
		AvailableBackends.Mutex.Lock()
		var newBackend = AvailableBackends.Available[rand.IntN(len(AvailableBackends.Available))]
		AvailableBackends.Mutex.Unlock()

		if err = ph.db.Set(PadPrefix+*padId, models.DBBackend{
			Backend: newBackend,
		}); err != nil {
			ph.logger.Info("Error while setting padId in DB: ", err)
		}
		return ph.p[newBackend], nil
	}

	if len(AvailableBackends.Available) == 0 {
		ph.logger.Info("request made during startup")
	}

	if slices.Index(AvailableBackends.Up, padRead.Backend) != -1 {
		return ph.p[padRead.Backend], nil
	} else {
		AvailableBackends.Mutex.Lock()
		newBackend := AvailableBackends.Up[rand.IntN(len(AvailableBackends.Up))]
		AvailableBackends.Mutex.Unlock()
		if err = ph.db.Set(PadPrefix+*padId, models.DBBackend{
			Backend: newBackend,
		}); err != nil {
			ph.logger.Info("Error while setting padId in DB: ", err)
		}
		return ph.p[newBackend], nil
	}
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	var padId *string

	if strings.Index(r.URL.Path, "/p/") != -1 {
		var separatedString = strings.Split(r.URL.Path, "/p/")[1]
		var separatedStringByQuestion = strings.Split(separatedString, "?")[0]
		padId = &strings.Split(separatedStringByQuestion, "/")[0]
		ph.logger.Info("Initial request to /p/" + *padId)
	}

	if padId == nil {
		padIdToWrite := r.URL.Query().Get("padId")
		if padIdToWrite != "" {
			padId = &padIdToWrite
		}
	}

	var padProxy, err = ph.createRoute(padId)

	if err != nil {
		ph.logger.Error("Error while creating route: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		template := ui.Error()
		if err := template.Render(context.Background(), w); err != nil {
			ph.logger.Error("Error while rendering template: ", err)
		}
		return
	}

	padProxy.ServeHTTP(w, r)
}

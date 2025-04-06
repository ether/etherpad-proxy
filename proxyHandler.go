package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/ether/etherpad-proxy/models"
	"github.com/ether/etherpad-proxy/ui"
	"go.uber.org/zap"
	"log"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"
	"time"
)
import "math/rand/v2"
import _ "github.com/ether/etherpad-proxy/ui"

type ProxyHandler struct {
	p      map[string]httputil.ReverseProxy
	logger *zap.SugaredLogger
	db     DB
}

type StaticResource struct {
	Backend  string
	FullPath string
}

var StaticResourceMap = map[string]StaticResource{}

type ResourceNotFound struct {
	newPath string
}

func (m *ResourceNotFound) Error() string {
	return "Resource not found"
}

type ClashInPadId struct {
	padId string
}

func (m *ClashInPadId) Error() string {
	return "Resource not found"
}

func ScrapeJSFiles(backends models.Settings) {
	go func() {
		for {
			for key, backend := range backends.Backends {
				response, err := http.Get("http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/p/test")
				if err != nil {
					log.Println("Error while scraping JS files: ", err)
					continue
				}

				doc, err := goquery.NewDocumentFromReader(response.Body)
				doc.Find("script").Each(func(i int, s *goquery.Selection) {
					log.Println(s.Attr("src"))
					var src, ok = s.Attr("src")
					if ok {
						if strings.Index(src, "padbootstrap") != -1 {
							var splittedPath = strings.Split(src, "/")
							StaticResourceMap[splittedPath[len(splittedPath)-1]] = StaticResource{
								Backend:  key,
								FullPath: "http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/" + splittedPath[len(splittedPath)-1],
							}
						}
					}
				})
				if err = response.Body.Close(); err != nil {
					log.Println("Error while closing response body: ", err)
					continue
				}
			}
			time.Sleep(10 * time.Minute)
		}

	}()
}

func (ph *ProxyHandler) createRoute(padId *string, r *http.Request) (*httputil.ReverseProxy, error) {
	if padId == nil {
		var newBackend *string
		// It's a static resource
		AvailableBackends.Mutex.Lock()
		if len(AvailableBackends.Available) == 0 {
			return nil, errors.New("no backends available")
		}
		if strings.Contains(r.URL.Path, "padbootstrap") {
			// This is a static resource
			// We need to find the backend that serves this resource
			var splittedPath = strings.Split(r.URL.Path, "/")
			var resourceName = splittedPath[len(splittedPath)-1]
			if key, okay := StaticResourceMap[resourceName]; okay {
				for i := 0; i < len(AvailableBackends.Up); i++ {
					if AvailableBackends.Up[i] == key.Backend {
						newBackend = &key.Backend
						break
					}
				}
			} else {
				var firstNewPadRef string
				for _, resource := range StaticResourceMap {
					firstNewPadRef = resource.FullPath
				}
				return nil, &ResourceNotFound{
					firstNewPadRef,
				}
			}
		} else {
			newBackend = &AvailableBackends.Available[rand.IntN(len(AvailableBackends.Available))]
		}
		AvailableBackends.Mutex.Unlock()
		var chosenBackend = ph.p[*newBackend]
		return &chosenBackend, nil
	}

	var padRead, err = ph.db.Get(*padId)
	if len(AvailableBackends.Available) == 0 {
		return nil, errors.New("no backends available")
	}
	if errors.Is(err, sql.ErrNoRows) {
		// if no backend is stored for this pad, create a new connection
		result, err := ph.db.getClashByPadID(*padId)

		if err != nil && errors.Is(err, sql.ErrNoRows) || len(result) == 0 {
			AvailableBackends.Mutex.Lock()
			var newBackend = AvailableBackends.Available[rand.IntN(len(AvailableBackends.Available))]
			AvailableBackends.Mutex.Unlock()

			if err = ph.db.Set(*padId, models.DBBackend{
				Backend: newBackend,
			}); err != nil {
				ph.logger.Info("Error while setting padId in DB: ", err)
			}
			var chosenBackend = ph.p[newBackend]

			return &chosenBackend, nil
		} else if err != nil {
			return nil, err
		}

		// There is an active clash for this pad
		ph.logger.Warnf("Pad %s is in a clash with backends: %v", *padId, result)
		return nil, &ClashInPadId{
			padId: *padId,
		}
	}

	if len(AvailableBackends.Available) == 0 {
		ph.logger.Info("request made during startup")
	}

	if slices.Index(AvailableBackends.Up, padRead.Backend) != -1 {
		var chosenBackend = ph.p[padRead.Backend]
		return &chosenBackend, nil
	} else {
		AvailableBackends.Mutex.Lock()
		newBackend := AvailableBackends.Up[rand.IntN(len(AvailableBackends.Up))]
		AvailableBackends.Mutex.Unlock()
		if err = ph.db.Set(*padId, models.DBBackend{
			Backend: newBackend,
		}); err != nil {
			ph.logger.Info("Error while setting padId in DB: ", err)
		}
		var chosenBackend = ph.p[newBackend]
		return &chosenBackend, nil
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

	var padProxy, err = ph.createRoute(padId, r)

	if err != nil {
		if errors.Is(err, &ResourceNotFound{}) {
			var resourceNotFound *ResourceNotFound
			errors.As(err, &resourceNotFound)
			var newPath = resourceNotFound.newPath
			http.Redirect(w, r, newPath, http.StatusTemporaryRedirect)
			return
		}

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

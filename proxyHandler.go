package main

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/metrics"
	"github.com/ether/etherpad-proxy/models"
	"github.com/ether/etherpad-proxy/ui"
	"go.uber.org/zap"
)

type StaticResource struct {
	Backend  string
	FullPath string
}

// staticResources is a concurrency-safe map of scraped static resource names.
type staticResources struct {
	mu sync.RWMutex
	m  map[string]StaticResource
}

func newStaticResources() *staticResources {
	return &staticResources{m: make(map[string]StaticResource)}
}

func (s *staticResources) set(name string, r StaticResource) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[name] = r
}

func (s *staticResources) get(name string) (StaticResource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.m[name]
	return r, ok
}

func (s *staticResources) anyPath() (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.m {
		return r.FullPath, true
	}
	return "", false
}

type ProxyHandler struct {
	p      map[string]httputil.ReverseProxy
	logger *zap.SugaredLogger
	db     interfaces.IDB
	state  *models.BackendState
	static *staticResources
}

type ResourceNotFound struct {
	newPath string
}

func (m *ResourceNotFound) Error() string { return "Resource not found" }

type ClashInPadId struct {
	padId string
}

func (m *ClashInPadId) Error() string { return "Pad clash" }

func ScrapeJSFiles(settings models.Settings, static *staticResources, logger *zap.SugaredLogger) {
	go func() {
		for {
			for key, backend := range settings.Backends {
				response, err := http.Get("http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/p/test")
				if err != nil {
					logger.Warnf("Error while scraping JS files: %v", err)
					continue
				}
				doc, err := goquery.NewDocumentFromReader(response.Body)
				if err != nil {
					logger.Warnf("Error parsing scraped document: %v", err)
					_ = response.Body.Close()
					continue
				}
				doc.Find("script").Each(func(_ int, s *goquery.Selection) {
					src, ok := s.Attr("src")
					if ok && strings.Contains(src, "padbootstrap") {
						parts := strings.Split(src, "/")
						name := parts[len(parts)-1]
						static.set(name, StaticResource{
							Backend:  key,
							FullPath: "http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/" + name,
						})
					}
				})
				if err = response.Body.Close(); err != nil {
					logger.Warnf("Error while closing response body: %v", err)
				}
			}
			time.Sleep(10 * time.Minute)
		}
	}()
}

// chooseBackend returns the backend key a request should be routed to, or an
// error (ResourceNotFound carries a redirect path; ClashInPadId signals an
// unresolved pad clash).
func (ph *ProxyHandler) chooseBackend(padId *string, r *http.Request) (string, error) {
	available := ph.state.SnapshotAvailable()
	up := ph.state.SnapshotUp()

	if padId == nil {
		if len(available) == 0 {
			return "", errors.New("no backends available")
		}
		if strings.Contains(r.URL.Path, "padbootstrap") {
			parts := strings.Split(r.URL.Path, "/")
			name := parts[len(parts)-1]
			if res, ok := ph.static.get(name); ok && slices.Contains(up, res.Backend) {
				return res.Backend, nil
			}
			if path, ok := ph.static.anyPath(); ok {
				return "", &ResourceNotFound{newPath: path}
			}
			return "", &ResourceNotFound{}
		}
		return available[rand.IntN(len(available))], nil
	}

	if len(available) == 0 {
		return "", errors.New("no backends available")
	}

	stored, err := ph.db.Get(*padId)
	if errors.Is(err, sql.ErrNoRows) {
		clashes, cerr := ph.db.GetClashByPadID(*padId)
		if cerr != nil && !errors.Is(cerr, sql.ErrNoRows) {
			metrics.DBErrorsTotal.Inc()
			return "", cerr
		}
		if len(clashes) == 0 {
			candidate := available[rand.IntN(len(available))]
			backend, aerr := ph.db.Assign(*padId, candidate)
			if aerr != nil {
				metrics.DBErrorsTotal.Inc()
				return "", aerr
			}
			metrics.PadAssignmentsTotal.Inc()
			return backend, nil
		}
		ph.logger.Warnf("Pad %s is in a clash with backends: %v", *padId, clashes)
		return "", &ClashInPadId{padId: *padId}
	} else if err != nil {
		metrics.DBErrorsTotal.Inc()
		return "", err
	}

	if slices.Contains(up, stored.Backend) {
		return stored.Backend, nil
	}
	if len(up) == 0 {
		return "", errors.New("no backends available")
	}
	newBackend := up[rand.IntN(len(up))]
	if serr := ph.db.Set(*padId, models.DBBackend{Backend: newBackend}); serr != nil {
		metrics.DBErrorsTotal.Inc()
		ph.logger.Info("Error while setting padId in DB: ", serr)
	}
	return newBackend, nil
}

func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ph.logger.Debugf("%s %s", r.Method, r.URL)
	var padId *string

	if strings.Contains(r.URL.Path, "/p/") {
		afterP := strings.Split(r.URL.Path, "/p/")[1]
		beforeQuery := strings.Split(afterP, "?")[0]
		first := strings.Split(beforeQuery, "/")[0]
		padId = &first
		ph.logger.Infof("Initial request to /p/%s", first)
	}

	if padId == nil {
		if q := r.URL.Query().Get("padId"); q != "" {
			padId = &q
		}
	}

	backendKey, err := ph.chooseBackend(padId, r)
	if err != nil {
		var resourceNotFound *ResourceNotFound
		if errors.As(err, &resourceNotFound) && resourceNotFound.newPath != "" {
			metrics.RequestsTotal.WithLabelValues("resource_redirect").Inc()
			http.Redirect(w, r, resourceNotFound.newPath, http.StatusTemporaryRedirect)
			return
		}
		var clash *ClashInPadId
		if errors.As(err, &clash) {
			metrics.RequestsTotal.WithLabelValues("clash").Inc()
		} else {
			metrics.RequestsTotal.WithLabelValues("no_backend").Inc()
		}
		ph.logger.Error("Error while creating route: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		template := ui.Error()
		if rerr := template.Render(context.Background(), w); rerr != nil {
			ph.logger.Error("Error while rendering template: ", rerr)
		}
		return
	}

	proxy := ph.p[backendKey]
	metrics.RequestsTotal.WithLabelValues("proxied").Inc()
	proxy.ServeHTTP(w, r)
}

package main

import (
	"encoding/json"
	"github.com/ether/etherpad-proxy/models"
	"io"
	"maps"
	"net/http"
	"strconv"
)

func filterId(id string, arrayToFilter []string) []string {
	var filteredArray []string
	for _, item := range arrayToFilter {
		if item != id {
			filteredArray = append(filteredArray, item)
		}
	}
	return filteredArray
}

func checkAvailability(settings models.Settings) models.CheckAvailabilityModel {
	var available = make([]string, 0)
	for key := range maps.Keys(settings.Backends) {
		available = append(available, key)
	}

	up := available
	for key, backend := range settings.Backends {
		response, err := http.Get("http://" + backend.Host + ":" + strconv.Itoa(backend.Port) + "/stats")
		if err != nil || response == nil {
			available = filterId(key, available)
			up = filterId(key, up)
			continue
		}
		body, err := io.ReadAll(response.Body)
		if err = response.Body.Close(); err != nil {
			available = filterId(key, available)
			up = filterId(key, up)
			continue
		}
		var statsRequest models.StatsRequest
		if err := json.Unmarshal(body, &statsRequest); err != nil {
			available = filterId(key, available)
			up = filterId(key, up)
		}

		if statsRequest.ActivePads >= settings.MaxPadsPerInstance {
			available = filterId(key, available)
		}
	}
	return models.CheckAvailabilityModel{
		Available: available,
		Up:        up,
	}
}

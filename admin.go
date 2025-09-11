package main

import (
	"context"
	"net/http"

	"github.com/ether/etherpad-proxy/databases/interfaces"
	"github.com/ether/etherpad-proxy/ui"
	"go.uber.org/zap"
)

type AdminPanel struct {
	DB     interfaces.IDB
	logger *zap.SugaredLogger
}

func (a *AdminPanel) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	padIDMap, err := a.DB.GetAllPads()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var component = ui.Management(padIDMap)
	if err := component.Render(context.Background(), w); err != nil {
		a.logger.Error("Error while rendering template: ", err)
	}
}

package models

import "sync"

type AvailableBackends struct {
	Up        []string
	Available []string
	Mutex     sync.Mutex
}

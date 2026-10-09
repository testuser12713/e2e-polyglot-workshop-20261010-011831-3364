package handlers

import (
	"workshop/internal/config"
	"workshop/internal/queue"
	"workshop/internal/store"
)

// Store, Cfg and Queue are the dependencies every handler shares. Init wires
// them once at startup.
var (
	Store *store.Store
	Cfg   *config.Config
	Queue *queue.Publisher
)

// Init wires the shared handler dependencies.
func Init(s *store.Store, c *config.Config, q *queue.Publisher) {
	Store = s
	Cfg = c
	Queue = q
}

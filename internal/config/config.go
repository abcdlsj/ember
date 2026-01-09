package config

import "time"

const (
	HTTPClientTimeout = 15 * time.Second
	ImageFetchTimeout = 10 * time.Second
	DefaultPageSize   = 20
	ImageCacheWidth   = 120
	ImageCacheHeight  = 60
	PingInterval      = 10 * time.Second
	TicksPerSecond    = 10_000_000
	SidebarWidth      = 16
	StatusWidth       = 24
)

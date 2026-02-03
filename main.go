package main

import (
	"flag"
	"fmt"
	"os"

	"ember/internal/api"
	"ember/internal/player"
	"ember/internal/service"
	"ember/internal/storage"
	"ember/internal/ui"
	"ember/internal/web"
)

func main() {
	var webMode bool
	var webAddr string

	flag.BoolVar(&webMode, "web", false, "Run web UI server")
	flag.StringVar(&webAddr, "addr", "localhost:8080", "Web server address")
	flag.Parse()

	if !player.Available() {
		fmt.Println("Warning: mpv not found")
		fmt.Println("Install with: brew install mpv")
	}

	// Initialize storage
	store, err := storage.New()
	if err != nil {
		fmt.Printf("Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	// Initialize API client
	client := initClient(store)

	// Create Media Service (unified business logic layer)
	svc := service.NewMediaService(client, store)

	if webMode {
		// Run web UI server
		server := web.New(svc)
		if err := server.Run(webAddr); err != nil {
			fmt.Printf("Web server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Run TUI
		if err := ui.Run(svc); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func initClient(store *storage.Store) *api.Client {
	srv := store.GetActiveServer()
	if srv == nil {
		return api.New("")
	}

	client := api.New(srv.URL)
	client.UserID = srv.UserID
	client.Token = srv.Token

	if client.VerifyToken() {
		return client
	}

	if err := client.Login(srv.Username, srv.Password); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		return client
	}

	store.SaveServerToken(store.GetActiveServerIndex(), client.UserID, client.Token)
	return client
}

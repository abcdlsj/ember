package main

import (
	"fmt"
	"os"

	"ember/internal/api"
	"ember/internal/player"
	"ember/internal/storage"
	"ember/internal/ui"
)

func main() {
	if !player.Available() {
		fmt.Println("Warning: mpv not found")
		fmt.Println("Install with: brew install mpv")
	}

	store, err := storage.New()
	if err != nil {
		fmt.Printf("Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	client := initClient(store)

	if err := ui.Run(client, store); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
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

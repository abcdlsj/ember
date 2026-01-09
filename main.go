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
	server := os.Getenv("EMBY_SERVER")
	username := os.Getenv("EMBY_USERNAME")
	password := os.Getenv("EMBY_PASSWORD")

	if server == "" || username == "" || password == "" {
		fmt.Println("Error: Missing environment variables")
		fmt.Println("Please set: EMBY_SERVER, EMBY_USERNAME, EMBY_PASSWORD")
		os.Exit(1)
	}

	if !player.Available() {
		fmt.Println("Warning: mpv not found")
		fmt.Println("Install with: brew install mpv")
	}

	store, err := storage.New()
	if err != nil {
		fmt.Printf("Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	client := api.New()

	userID, token := store.GetToken(server)
	if userID != "" && token != "" {
		client.UserID = userID
		client.Token = token
		if client.VerifyToken() {
			fmt.Println("Using cached token...")
		} else {
			login(client, store, username, password)
		}
	} else {
		login(client, store, username, password)
	}

	if err := ui.Run(client, store); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func login(client *api.Client, store *storage.Store, username, password string) {
	fmt.Println("Logging in...")
	if err := client.Login(username, password); err != nil {
		fmt.Printf("Login failed: %v\n", err)
		os.Exit(1)
	}
	store.SetToken(client.Server, client.UserID, client.Token)
	fmt.Println("Login successful!")
}

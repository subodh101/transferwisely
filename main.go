package main

import (
	"fmt"
	"github.com/go-co-op/gocron"
	"net/http"
	"strconv"
	"time"
)

func main() {
	// intervalVar is defined and initialized in transferwise.go
	// It uses the "INTERVAL" environment variable or defaults to "1"
	interval, err := strconv.ParseUint(intervalVar, 10, 64)
	if err != nil {
		fmt.Printf("Invalid value for INTERVAL: %v\n", err)
		return
	}

	// Initialize the scheduler using UTC time
	s1 := gocron.NewScheduler(time.UTC)

	// Schedule the checkAndProcess function to run every X minutes
	// checkAndProcess is defined in transferwise.go and contains the API logic
	_, err = s1.Every(int(interval)).Minutes().Do(checkAndProcess)
	if err != nil {
		fmt.Printf("Error scheduling task: %v\n", err)
		return
	}

	// Start the scheduler in the background
	s1.StartAsync()

	// Keep the application running by starting a basic HTTP server
	fmt.Println("Starting batch server on port 3000")
	_ = http.ListenAndServe(":3000", nil)
}

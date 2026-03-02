package main

import (
	"fmt"
	"github.com/go-co-op/gocron"
	"net/http"
	"strconv"
	"time"
)

func main() {
	interval, err := strconv.ParseUint(intervalVar, 10, 64)
	if err != nil {
		fmt.Printf("Invalid value for INTERVAL: %v\n", err)
		return
	}

	// Use NewScheduler for v1 or the new v2 instantiation
	s1 := gocron.NewScheduler(time.UTC)

	_, err = s1.Every(interval).Minutes().Do(checkAndProcess)
	if err != nil {
		fmt.Printf("Error scheduling task: %v\n", err)
		return
	}

	// In newer versions, this is often StartAsync()
	// or simply Start() depending on the minor version.
	// If Start() is unexported, use StartAsync()
	s1.StartAsync()

	fmt.Println("Starting batch server on port 3000")
	_ = http.ListenAndServe(":3000", nil)
}

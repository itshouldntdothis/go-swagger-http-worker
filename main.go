package main

import (
	"log"
	"runtime"
	"strconv"

	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	version = "0.0.1"
	via     = "go-swagger-http-worker/" + version
)

func init() {
	envSwaggerMaxProcs := os.Getenv("SW_MAX_PROCS")
	var max = 1
	if len(envSwaggerMaxProcs) != 0 {
		maxProcs, err := strconv.Atoi(envSwaggerMaxProcs)
		if err != nil {
			log.Fatalf("Unable to covert env var 'SW_MAX_PROCS' of %v to int", envSwaggerMaxProcs)
		}
		max = maxProcs
	}
	runtime.GOMAXPROCS(max)
	log.Printf("GOMAXPROCS set to %v", max)
}

func main() {
	// close channel sin
	var close = make(chan int)

	// finishUP channel signals the application to finish up
	var finishUP = make(chan struct{})

	// done channel signals the signal handler that the application has completed
	var done = make(chan struct{})

	// gracefulStop is a channel of os.Signals that we will watch for -SIGTERM
	var gracefulStop = make(chan os.Signal)

	// watch for SIGTERM and SIGINT from the operating system, and notify the app on
	// the gracefulStop channel
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	// launch a worker whose job it is to always watch for gracefulStop signals
	go func() {
		// wait for our os signal to stop the app
		// on the graceful stop channel
		// this goroutine will block until we get an OS signal
		sig := <-gracefulStop
		log.Printf("Caught sig: %+v", sig)

		// send message on "finish up" channel to tell the app to
		// gracefully shutdown
		finishUP <- struct{}{}

		// wait for word back if we finished or not
		select {
		case <-time.After(9 * time.Second):
			// timeout after 9 seconds waiting for app to finish,
			// our application should Exit(1)
			log.Println("Error shutting down, killing process.")
			os.Exit(1)
		case <-done:
			// if we got a message on done, we finished, so end app
			// our application should Exit(0)
			log.Println("Successfully shut down.")
			close <- 0
		}
	}()

	grcpOptions := NewGrcpOptions()
	go grcpOptions.start()

	healthServer := NewHealthServer()
	go healthServer.start()

	<-finishUP

	grcpOptions.close()

	healthServer.close()

	done <- struct{}{}

	os.Exit(<-close)
}

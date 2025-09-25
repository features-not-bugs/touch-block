package main

import (
	"log"
	"os"
	"time"

	"github.com/jezek/xgb"
)

func main() {
	cfg := ParseFlags()

	xgb.Logger = log.New(os.Stdout, "X11: ", log.LstdFlags|log.Lmsgprefix)

	// Create X11 connection
	X, err := xgb.NewConn()
	if err != nil {
		log.Fatalf("Failed to connect to X server: %v", err)
	}
	defer X.Close()

	log.Println("Connected to X server")

	overlay, err := NewBlockOverlay(X)
	if err != nil {
		log.Fatalf("Failed to create overlay: %v", err)
	}
	defer overlay.Destroy()

	var eventChannel <-chan bool
	if cfg.UseDPMS {
		log.Println("Using DPMS")
		eventChannel, err = ListenToDPMSEvents(X, time.Duration(cfg.PollingInterval)*time.Millisecond)
		if err != nil {
			log.Println("Failure listening for DPMS events", err)
		}
	} else {
		log.Println("Using Xrandr")
		eventChannel, err = ListenToXrandrEvents(X)
		if err != nil {
			log.Println("Failure listening for Xrandr events", err)
		}
	}

	log.Printf("Initialized, delay: %dms, dpms-poll: %dms\n", cfg.Delay, cfg.PollingInterval)

	for {
		displayOn, ok := <-eventChannel
		if !ok {
			log.Println("Events channel closed")
			break
		}

		overlayActive := overlay.IsActive()

		if displayOn && overlayActive {
			if cfg.Delay > 0 {
				log.Printf("Waiting additional delay(%dms) before hiding overlay\n", cfg.Delay)
				time.Sleep(time.Duration(cfg.Delay) * time.Millisecond)
			}
			err = overlay.Hide()
			if err != nil {
				log.Printf("Failed to hide overlay: %v\n", err)
			}
		} else if !displayOn && !overlayActive {
			err = overlay.Show()
			if err != nil {
				log.Printf("failed to show overlay: %v\n", err)
			}
		}
	}

	log.Println("Exiting...")
}

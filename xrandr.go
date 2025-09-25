package main

import (
	"fmt"
	"log"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

func ListenToXrandrEvents(X *xgb.Conn) (<-chan bool, error) {
	if err := randr.Init(X); err != nil {
		return nil, fmt.Errorf("xrandr not available: %v", err)
	}

	setup := xproto.Setup(X)
	root := setup.DefaultScreen(X).Root

	err := randr.SelectInputChecked(X, root,
		randr.NotifyMaskScreenChange|
			randr.NotifyMaskCrtcChange|
			randr.NotifyMaskOutputChange|
			randr.NotifyMaskOutputProperty).Check()

	if err != nil {
		return nil, fmt.Errorf("failed to register for xrandr events: %v", err)
	}

	stateChan := make(chan bool, 1)
	lastState := isXrandrDisplayOn(X, root)

	go func() {
		defer close(stateChan)

		// Emit the initial state
		stateChan <- lastState

		for {
			ev, err := X.WaitForEvent()
			if err != nil {
				log.Printf("Xrandr event error: %v", err)
				return
			}

			// Check if it's a RandR event
			switch ev.(type) {
			case randr.ScreenChangeNotifyEvent, randr.NotifyEvent:
				// Check current state
				currentState := isXrandrDisplayOn(X, root)

				// Only emit if state changed
				if currentState != lastState {
					lastState = currentState
					stateChan <- currentState
				}
			}
		}
	}()

	return stateChan, nil
}

func isXrandrDisplayOn(X *xgb.Conn, root xproto.Window) bool {
	resources, err := randr.GetScreenResources(X, root).Reply()
	if err != nil {
		return false
	}

	// Check each output
	for _, output := range resources.Outputs {
		info, err := randr.GetOutputInfo(X, output, 0).Reply()
		if err != nil {
			continue
		}

		// Check if output is connected and has an active CRTC
		if info.Connection == randr.ConnectionConnected && info.Crtc != 0 {
			// Check if CRTC has a mode (is active)
			crtcInfo, err := randr.GetCrtcInfo(X, info.Crtc, 0).Reply()
			if err == nil && crtcInfo.Mode != 0 {
				// Found at least one active output - display is on
				return true
			}
		}
	}

	// No active outputs found - display is off
	return false
}

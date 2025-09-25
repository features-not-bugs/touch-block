package main

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/dpms"
)

func ListenToDPMSEvents(X *xgb.Conn, pollInterval time.Duration) (<-chan bool, error) {
	if err := dpms.Init(X); err != nil {
		return nil, fmt.Errorf("DPMS not available: %v", err)
	}

	capReply, err := dpms.Capable(X).Reply()
	if err != nil || capReply == nil || !capReply.Capable {
		return nil, fmt.Errorf("DPMS not supported")
	}

	stateChannel := make(chan bool, 1)
	lastState, err := isDPMSDisplayOn(X)
	if err != nil {
		return nil, errors.Join(errors.New("failure fetching initial dpms state"), err)
	}

	// start the polling loop
	go func() {
		defer close(stateChannel)

		// Emit the initial state
		stateChannel <- lastState

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			_, ok := <-ticker.C
			if !ok {
				return
			}

			currentState, err := isDPMSDisplayOn(X)
			if err != nil {
				log.Println("failed to get display status", err)
				return
			}

			if currentState != lastState {
				lastState = currentState
				stateChannel <- currentState
			}
		}
	}()

	return stateChannel, nil
}

func isDPMSDisplayOn(X *xgb.Conn) (bool, error) {
	infoReply, err := dpms.Info(X).Reply()
	if err != nil {
		return false, errors.Join(errors.New("failed to query DPMS state"), err)
	}

	if infoReply == nil {
		return false, errors.Join(errors.New("failed to query DPMS state"), errors.New("reply was nil"))
	}

	if !infoReply.State {
		return false, errors.New("DPMS state is false")
	}

	switch infoReply.PowerLevel {
	case dpms.DPMSModeStandby:
		fallthrough
	case dpms.DPMSModeSuspend:
		fallthrough
	case dpms.DPMSModeOff:
		return false, nil

	default:
		log.Printf("Unknown DPMS power level (%d)\n", infoReply.PowerLevel)
		fallthrough

	case dpms.DPMSModeOn:
		return true, nil
	}
}

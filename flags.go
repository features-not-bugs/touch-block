package main

import (
	"flag"
)

type FlagCfg struct {
	UseDPMS         bool
	Delay           uint64
	PollingInterval uint64
}

func ParseFlags() *FlagCfg {
	cfg := FlagCfg{}
	flag.BoolVar(&cfg.UseDPMS, "dpms", true, "use dpms instead of xrandr as the source of truth for testing if the display is on or off, (default true)")
	flag.Uint64Var(&cfg.Delay, "delay", 500, "additional input blocking delay in milliseconds (default 500)")
	flag.Uint64Var(&cfg.PollingInterval, "dpms-poll", 50, "interval in milliseconds to poll for DPMS state changes (default 50)")
	flag.Parse()
	return &cfg
}

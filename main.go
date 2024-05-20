package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"

	"seaspy/aisstream"
)

type Config struct {
	Dock      Dock             `json:"dock"`
	Portal    Portal           `json:"portal"`
	Swabby    Swabby           `json:"swabby"`
	Aisstream aisstream.Config `json:"aisstream"`
	Google    Google           `json:"google"`
}

func main() {
	configFile := flag.String("c", "", "config file")
	flag.Parse()

	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("could not load config file: %s\n", err.Error())
	}

	ais := aisstream.NewAIS(config.Aisstream.Url, config.Aisstream.Api)
	ais.Sub.AddBox(config.Aisstream.DefaultSub.Boxes)
	ais.Sub.AddMMSI(config.Aisstream.DefaultSub.FilterMMSI)
	ais.Sub.AddMsgType(config.Aisstream.DefaultSub.FilterMsgType)
	go ais.ConnectAndStream()

	dock := NewDock(config.Dock)
	go dock.Run(ais.Msg)

	swabby := NewSwabby(config.Swabby)
	go swabby.Cleanup(dock)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	ListenAndServe(ctx, dock, config.Portal, config.Google)

	ais.Quit <- struct{}{}
	<-ais.Done

	dock.Quit <- struct{}{}
	<-dock.Done

	swabby.Quit <- struct{}{}
	<-swabby.Done
}

func loadConfig(f string) (Config, error) {
	var config Config

	configFile, err := os.Open(f)
	if err != nil {
		return config, fmt.Errorf("could not open file: %w", err)
	}
	defer configFile.Close()

	configBytes, err := io.ReadAll(configFile)
	if err != nil {
		return config, fmt.Errorf("could not read file: %w", err)
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return config, fmt.Errorf("could not unmarshal file: %w", err)
	}

	return config, nil
}

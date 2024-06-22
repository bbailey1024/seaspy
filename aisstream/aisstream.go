package aisstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

const (
	DIAL_TIMEOUT       = 5
	SUBSCRIBE_TIMEOUT  = 5
	HEARTBEAT_TIMEOUT  = 10
	HEARTBEAT_INTERVAL = 30
	BACKOFF_MULTIPLIER = 5
	BACKOFF_MAX        = 30
)

type AIS struct {
	Url  string
	Conn *websocket.Conn
	Sub  SubMsg
	Msg  chan []byte
	Quit chan struct{}
	Done chan struct{}
}
type SubMsg struct {
	APIKey             string        `json:"APIKey"`
	BoundingBoxes      [][][]float64 `json:"BoundingBoxes"`
	FiltersShipMMSI    []string      `json:"FiltersShipMMSI,omitempty"`
	FilterMessageTypes []string      `json:"FilterMessageTypes,omitempty"`
}

type Config struct {
	Url        string `json:"url"`
	Api        string `json:"api"`
	DefaultSub struct {
		Boxes         [][][]float64 `json:"boxes"`
		FilterMMSI    []string      `json:"filterMMSI"`
		FilterMsgType []string      `json:"filterMsgType"`
	} `json:"defaultSub"`
}

type Packet struct {
	Msg      Message  `json:"Message"`
	MsgType  string   `json:"MessageType"`
	Metadata Metadata `json:"Metadata"`
}

type Metadata struct {
	MMSI       int     `json:"MMSI"`
	MMSIString int     `json:"MMSI_String"`
	ShipName   string  `json:"ShipName"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	TimeUtc    string  `json:"time_utc"`
}

type Message struct {
	PositionReport PositionReport `json:"PositionReport,omitempty"`
	ShipStaticData ShipStaticData `json:"ShipStaticData,omitempty"`
}

func NewAIS(url string, api string) *AIS {
	return &AIS{
		Url:  url,
		Sub:  SubMsg{APIKey: api},
		Msg:  make(chan []byte),
		Quit: make(chan struct{}),
		Done: make(chan struct{}),
	}
}

func (ais *AIS) Connect() error {

	hc := &http.Client{Timeout: time.Duration(DIAL_TIMEOUT) * time.Second}

	c, _, err := websocket.Dial(context.Background(), ais.Url, &websocket.DialOptions{HTTPClient: hc})
	if err != nil {
		return fmt.Errorf("could not connect to websocket: %w", err)
	}
	ais.Conn = c
	return nil
}

func (sub *SubMsg) AddBox(newBox [][][]float64) {
	sub.BoundingBoxes = append(sub.BoundingBoxes, newBox...)
}

func (sub *SubMsg) AddMMSI(newMmsi []string) {
	sub.FiltersShipMMSI = append(sub.FiltersShipMMSI, newMmsi...)
}

func (sub *SubMsg) AddMsgType(newMsgType []string) {
	sub.FilterMessageTypes = append(sub.FilterMessageTypes, newMsgType...)
}

func (sub *SubMsg) ClearBoxes() {
	sub.BoundingBoxes = nil
}

func (sub *SubMsg) ClearMMSIs() {
	sub.FiltersShipMMSI = nil
}

func (sub *SubMsg) ClearMsgTypes() {
	sub.FilterMessageTypes = nil
}

func (sub *SubMsg) ClearAll() {
	sub.BoundingBoxes = nil
	sub.FiltersShipMMSI = nil
	sub.FilterMessageTypes = nil
}

func (ais *AIS) Subscribe() error {
	b, err := json.Marshal(ais.Sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(SUBSCRIBE_TIMEOUT)*time.Second)
	defer cancel()

	err = ais.Conn.Write(ctx, websocket.MessageText, b)
	if err != nil {
		return fmt.Errorf("failed to write subscription message to websocket: %w", err)
	}

	return nil
}

func (ais *AIS) ConnectAndStream() {

	backoffCount := 0

connect:
	for {
		select {
		case <-ais.Quit:
			ais.Done <- struct{}{}
			return
		default:
			err := ais.Connect()
			if err != nil {
				fmt.Printf("ais connect failed: %s\n", err.Error())
				backoffCount = backoff(backoffCount)
				continue connect
			}

			err = ais.Subscribe()
			if err != nil {
				fmt.Printf("ais subscribe failed: %s\n", err.Error())
				ais.Conn.Close(websocket.StatusNormalClosure, "")
				backoffCount = backoff(backoffCount)
				continue connect
			}

			go ais.heartbeat()

			for {
				select {
				case <-ais.Quit:
					ais.Conn.Close(websocket.StatusNormalClosure, "")
					ais.Done <- struct{}{}
					return
				default:
					_, b, err := ais.Conn.Read(context.Background())
					if err != nil {
						fmt.Printf("ais read failed: %s\n", err.Error())
						ais.Conn.Close(websocket.StatusNormalClosure, "")
						backoffCount = backoff(backoffCount)
						continue connect
					}
					backoffCount = 0
					ais.Msg <- b
				}
			}
		}
	}
}

// heartbeat sends websocket pings to server at HEARTBEAT_INTERVAL intervals.
// If Conn.ping fails to write to the connection within HEARTBEAT_TIMEOUT, Conn.ping will close the connection.
// This will cause Conn.Read to fail in the event the connection is broken.
// This allows Reads without context timeouts for low volume subscriptions.
// https://github.com/nhooyr/websocket/blob/e3a2d32f704fb06c439e56d2a85334de04b50d32/conn.go#L224
func (ais *AIS) heartbeat() {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(HEARTBEAT_TIMEOUT)*time.Second)
		err := ais.Conn.Ping(ctx)
		if err != nil {
			cancel()
			return
		}
		cancel()
		time.Sleep(time.Duration(HEARTBEAT_INTERVAL) * time.Second)
	}
}

func backoff(backoffCount int) int {

	backoffSleep := BACKOFF_MULTIPLIER * backoffCount
	if backoffSleep > BACKOFF_MAX {
		backoffSleep = BACKOFF_MAX
	}

	time.Sleep(time.Duration(backoffSleep) * time.Second)

	return backoffCount + 1
}

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"seaspy/aisstream"
)

const (
	MOVING_SPEED_THRESHOLD = 0.1
	HEADING_RESET          = 511
)

type Dock struct {
	Workers    int
	WorkerList []*DockWorker
	Quit       chan struct{}
	Done       chan struct{}
	Ships      *Ships
}

type Ships struct {
	StateLock   sync.RWMutex
	State       map[int]*State
	InfoLock    sync.RWMutex
	Info        map[int]*Info
	HistoryLock sync.RWMutex
	History     map[int][]History
}

type State struct {
	Name       string    `json:"name"`
	LatLon     []float64 `json:"latlon"`
	Heading    int       `json:"heading"`
	SOG        float64   `json:"sog"`
	NavStat    int       `json:"navStat"`
	ShipType   int       `json:"shipType"`
	Marker     int       `json:"marker"`
	Rotation   int       `json:"rotation"`
	LastUpdate int64     `json:"lastUpdate"`
}

type Info struct {
	Destination string `json:"destination"`
	IMONumber   int    `json:"imoNumber"`
}

type History struct {
	LatLon    []float64 `json:"latlon"`
	Timestamp int64     `json:"timestamp"`
}

type ShipDump struct {
	State   State     `json:"state"`
	Info    Info      `json:"info"`
	History []History `json:"history"`
}

func NewDock(w int) *Dock {
	return &Dock{
		Workers:    w,
		WorkerList: []*DockWorker{},
		Quit:       make(chan struct{}),
		Done:       make(chan struct{}),
		Ships:      NewShips(),
	}
}

func NewShips() *Ships {
	return &Ships{
		State:   map[int]*State{},
		Info:    map[int]*Info{},
		History: map[int][]History{},
	}
}

func (d *Dock) Run(msg <-chan []byte) {
	for i := 0; i < d.Workers; i++ {
		dw := NewDockWorker()
		d.WorkerList = append(d.WorkerList, dw)
		go dw.Work(d, msg)
	}

	<-d.Quit

	for _, dw := range d.WorkerList {
		dw.Quit <- struct{}{}
		<-dw.Done
	}

	d.Done <- struct{}{}
}

type DockWorker struct {
	Quit chan struct{}
	Done chan struct{}
}

func NewDockWorker() *DockWorker {
	return &DockWorker{
		Quit: make(chan struct{}),
		Done: make(chan struct{}),
	}
}

func (dw *DockWorker) Work(d *Dock, msg <-chan []byte) {
	for {
		select {
		case <-dw.Quit:
			dw.Done <- struct{}{}
			return
		case b := <-msg:
			var p aisstream.Packet
			err := json.Unmarshal(b, &p)
			if err != nil {
				fmt.Printf("dock worker failed to unmarshal packet: %s\n", err.Error())
				continue
			}

			d.Ships.NewShip(p.Metadata.MMSI)

			d.Ships.UpdateMetadata(p.Metadata)
			d.Ships.UpdateHistory(p.Metadata.MMSI, []float64{p.Metadata.Latitude, p.Metadata.Longitude})

			switch p.MsgType {
			case "PositionReport":
				d.Ships.UpdatePositionReport(p.Metadata.MMSI, p.Msg.PositionReport)
			case "ShipStaticData":
				d.Ships.UpdateShipStaticData(p.Metadata.MMSI, p.Msg.ShipStaticData)
			}

			d.Ships.UpdateMarker(p.Metadata.MMSI)
		}
	}
}

func (s *Ships) NewShip(mmsi int) {
	s.StateLock.Lock()
	if _, ok := s.State[mmsi]; !ok {
		s.State[mmsi] = &State{}
	}
	s.StateLock.Unlock()

	s.InfoLock.Lock()
	if _, ok := s.Info[mmsi]; !ok {
		s.Info[mmsi] = &Info{}
	}
	s.InfoLock.Unlock()

	s.HistoryLock.Lock()
	if _, ok := s.History[mmsi]; !ok {
		s.History[mmsi] = []History{}
	}
	s.HistoryLock.Unlock()
}

func (s *Ships) UpdateMetadata(m aisstream.Metadata) {
	s.StateLock.Lock()
	defer s.StateLock.Unlock()

	s.State[m.MMSI].Name = m.ShipName
	s.State[m.MMSI].LatLon = []float64{m.Latitude, m.Longitude}
	s.State[m.MMSI].LastUpdate = time.Now().UTC().Unix()
}

func (s *Ships) UpdatePositionReport(mmsi int, m aisstream.PositionReport) {
	s.StateLock.Lock()
	defer s.StateLock.Unlock()
	s.State[mmsi].Heading = m.TrueHeading
	s.State[mmsi].SOG = m.Sog
	s.State[mmsi].NavStat = m.NavigationalStatus
}

func (s *Ships) UpdateShipStaticData(mmsi int, m aisstream.ShipStaticData) {
	s.StateLock.Lock()
	s.State[mmsi].ShipType = m.Type
	s.StateLock.Unlock()

	s.InfoLock.Lock()
	s.Info[mmsi].Destination = m.Destination
	s.Info[mmsi].IMONumber = m.ImoNumber
	s.InfoLock.Unlock()
}

func NewHistory(latLon []float64) History {
	return History{
		LatLon:    latLon,
		Timestamp: time.Now().UTC().Unix(),
	}
}

func (s *Ships) UpdateHistory(mmsi int, latLon []float64) {
	s.HistoryLock.Lock()
	defer s.HistoryLock.Unlock()

	if len(s.History[mmsi]) == 0 || shipMoved(latLon, s.History[mmsi][0].LatLon) {
		s.History[mmsi] = append([]History{NewHistory(latLon)}, s.History[mmsi]...) // Prepend new latLon to existing history
	}
}

// shipMoved attempts to determine whether a ship has moved since last update.
// The intention is to stop history additions for ships that haven't moved.
// The gps coordinates are rounded to nearest four decimal places as ships with higher precision gps may wobble when stationary.
// e.g., 36.87983666666666 -> 36.8798
func shipMoved(current []float64, previous []float64) bool {
	for i := 0; i < 2; i++ {
		if math.Round(current[i]*10000)/10000 != math.Round(previous[i]*10000)/10000 {
			return true
		}
	}
	return false
}

func (s *Ships) UpdateMarker(mmsi int) {
	s.StateLock.Lock()
	defer s.StateLock.Unlock()

	ship := s.State[mmsi]
	if ship.NavStat == 1 || ship.NavStat == 5 || ship.NavStat == 6 {
		ship.Marker = 0
	} else if ship.SOG > MOVING_SPEED_THRESHOLD {
		ship.Marker = 1
	} else {
		ship.Marker = 2
	}

	if ship.Heading == HEADING_RESET {
		ship.Rotation = 0
	} else {
		ship.Rotation = ship.Heading
	}
}

func (s *Ships) GetShips() (string, error) {
	s.StateLock.RLock()
	b, err := json.Marshal(s.State)
	s.StateLock.RUnlock()
	if err != nil {
		return "", fmt.Errorf("failed to marshal ships: %w", err)
	}

	return string(b), nil
}

func (s *Ships) GetShipInfo(mmsi int) (string, error) {
	s.InfoLock.RLock()
	defer s.InfoLock.RUnlock()

	if _, ok := s.Info[mmsi]; !ok {
		return "", fmt.Errorf("mmsi does not exist in ship info")
	}

	b, err := json.Marshal(s.Info[mmsi])
	if err != nil {
		return "", fmt.Errorf("could not marshal ship info: %w", err)
	}
	return string(b), nil
}

func (s *Ships) GetShipHistory(mmsi int) (string, error) {
	s.HistoryLock.RLock()
	defer s.HistoryLock.RUnlock()

	if _, ok := s.History[mmsi]; !ok {
		return "", fmt.Errorf("mmsi does not exist in ship history")
	}

	b, err := json.Marshal(s.History[mmsi])
	if err != nil {
		return "", fmt.Errorf("could not marshal ship history: %w", err)
	}
	return string(b), nil
}

func (s *Ships) GetShipDump() (string, error) {

	ships := make(map[int]*ShipDump)

	s.StateLock.RLock()
	for k, v := range s.State {
		ships[k] = &ShipDump{}
		ships[k].State = *v
	}
	s.StateLock.RUnlock()

	s.InfoLock.RLock()
	for k, v := range s.Info {
		ships[k].Info = *v

	}
	s.InfoLock.RUnlock()

	s.HistoryLock.RLock()
	for k, v := range s.History {
		ships[k].History = v

	}
	s.HistoryLock.RUnlock()

	b, err := json.Marshal(ships)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ship dump: %w", err)
	}

	return string(b), nil
}

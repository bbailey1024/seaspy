package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"slices"
	"sync"
	"time"

	"seaspy/aisstream"

	"github.com/bbailey1024/geohash"
)

const (
	MOVING_SPEED_THRESHOLD = 0.1
	HEADING_RESET          = 511
)

type Dock struct {
	ShipHistory bool `json:"shipHistory"`
	CacheTimer  int  `json:"cacheTimer"`
	Workers     int  `json:"workerCount"`
	WorkerList  []*DockWorker
	Quit        chan struct{}
	Done        chan struct{}
	Ships       *Ships
	Cache       *Cache
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
	MMSI       int       `json:"mmsi"`
	Name       string    `json:"name"`
	LatLon     []float64 `json:"latlon"`
	Geohash    uint64    `json:"geohash"`
	Heading    int       `json:"heading"`
	SOG        float64   `json:"sog"`
	NavStatus  int       `json:"navStatus"`
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

type InfoWindow struct {
	Name        string    `json:"name"`
	MMSI        int       `json:"mmsi"`
	LatLon      []float64 `json:"latlon"`
	Heading     int       `json:"heading"`
	SOG         float64   `json:"sog"`
	NavStatus   int       `json:"navStatus"`
	ShipType    int       `json:"shipType"`
	LastUpdate  int64     `json:"lastUpdate"`
	Destination string    `json:"destination"`
	IMONumber   int       `json:"imoNumber"`
}

type ShipDump struct {
	State   State     `json:"state"`
	Info    Info      `json:"info"`
	History []History `json:"history"`
}

type DockWorker struct {
	Quit chan struct{}
	Done chan struct{}
}

func NewDock(d Dock) *Dock {
	d.WorkerList = []*DockWorker{}
	d.Quit = make(chan struct{})
	d.Done = make(chan struct{})
	d.Ships = NewShips()
	return &d
}

func NewDockDefaults() *Dock {
	return &Dock{
		Workers:     10,
		WorkerList:  []*DockWorker{},
		Quit:        make(chan struct{}),
		Done:        make(chan struct{}),
		Ships:       NewShips(),
		ShipHistory: true,
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

	if d.Workers < 1 {
		log.Fatalf("must have at least 1 dock worker, config specifies %d\n", d.Workers)
	}

	for i := 0; i < d.Workers; i++ {
		dw := NewDockWorker()
		d.WorkerList = append(d.WorkerList, dw)
		go dw.Work(d, msg)
	}

	d.Cache = NewCache(d.CacheTimer)
	go d.Cache.Run(d.Ships)

	<-d.Quit

	d.Cache.Quit <- struct{}{}
	<-d.Cache.Done

	for _, dw := range d.WorkerList {
		dw.Quit <- struct{}{}
		<-dw.Done
	}

	d.Done <- struct{}{}
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

			if p.Metadata.MMSI == 0 {
				continue
			}

			d.Ships.StateLock.Lock()
			d.Ships.NewShip(p.Metadata.MMSI)
			d.Ships.UpdateMetadata(p.Metadata)
			d.Ships.StateLock.Unlock()

			if d.ShipHistory {
				d.Ships.UpdateHistory(p.Metadata.MMSI, []float64{p.Metadata.Latitude, p.Metadata.Longitude})
			}

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
	if _, ok := s.State[mmsi]; !ok {
		s.State[mmsi] = &State{}
	}

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
	s.State[m.MMSI].MMSI = m.MMSI
	s.State[m.MMSI].Name = m.ShipName
	s.State[m.MMSI].LatLon = []float64{m.Latitude, m.Longitude}
	s.State[m.MMSI].Geohash = geohash.EncodeInt(s.State[m.MMSI].LatLon[0], s.State[m.MMSI].LatLon[1])
	s.State[m.MMSI].LastUpdate = time.Now().Unix()
}

func (s *Ships) UpdatePositionReport(mmsi int, m aisstream.PositionReport) {
	s.StateLock.Lock()
	defer s.StateLock.Unlock()
	s.State[mmsi].Heading = m.TrueHeading
	s.State[mmsi].SOG = m.Sog
	s.State[mmsi].NavStatus = m.NavigationalStatus
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
	if ship.NavStatus == 1 || ship.NavStatus == 5 || ship.NavStatus == 6 {
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

func (s *Ships) GetInfoWindow(mmsi int) (InfoWindow, error) {
	s.StateLock.RLock()
	defer s.StateLock.RUnlock()

	s.InfoLock.RLock()
	defer s.InfoLock.RUnlock()

	var infoWindow InfoWindow

	if _, ok := s.State[mmsi]; !ok {
		return infoWindow, fmt.Errorf("mmsi does not exist in ship state")
	}

	if _, ok := s.Info[mmsi]; !ok {
		return infoWindow, fmt.Errorf("mmsi does not exist in ship info")
	}

	infoWindow.Name = s.State[mmsi].Name
	infoWindow.MMSI = s.State[mmsi].MMSI
	infoWindow.LatLon = s.State[mmsi].LatLon
	infoWindow.Heading = s.State[mmsi].Heading
	infoWindow.SOG = s.State[mmsi].SOG
	infoWindow.NavStatus = s.State[mmsi].NavStatus
	infoWindow.ShipType = s.State[mmsi].ShipType
	infoWindow.LastUpdate = s.State[mmsi].LastUpdate
	infoWindow.Destination = s.Info[mmsi].Destination
	infoWindow.IMONumber = s.Info[mmsi].IMONumber

	return infoWindow, nil
}

func (s *Ships) GetShipHistory(mmsi int) ([]History, error) {
	s.HistoryLock.RLock()
	defer s.HistoryLock.RUnlock()

	if history, ok := s.History[mmsi]; !ok {
		return nil, fmt.Errorf("mmsi does not exist in ship history")
	} else {
		return history, nil
	}
}

func (s *Ships) GetShipDump() (map[int]*ShipDump, error) {

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

	return ships, nil
}

func (s *Ships) GetShipsInBox(bbox [2][2]float64, geocache *Geocache) ([]*State, error) {

	if !validBbox(bbox) {
		return nil, fmt.Errorf("bounding box out of range")
	}

	begin, end, err := geocache.BinarySearch(bbox)
	if err != nil {
		return nil, err
	}

	// If begin is greater than end, ship results should loop around geocache list.
	var binaryShipResults []int
	if begin > end {
		for i := begin; i < len(geocache.List); i++ {
			binaryShipResults = append(binaryShipResults, geocache.List[i].MMSI)
		}
		for i := 0; i <= end; i++ {
			binaryShipResults = append(binaryShipResults, geocache.List[i].MMSI)
		}
	} else {
		for i := begin; i <= end; i++ {
			binaryShipResults = append(binaryShipResults, geocache.List[i].MMSI)
		}
	}

	shipsInCoords := make([]*State, 0)

	s.StateLock.RLock()
	for _, mmsi := range binaryShipResults {
		ship := s.State[mmsi]
		if ship.LatLon[0] >= bbox[0][0] && ship.LatLon[0] < bbox[1][0] && ship.LatLon[1] >= bbox[0][1] && ship.LatLon[1] < bbox[1][1] {
			shipsInCoords = append(shipsInCoords, ship)
		}
	}
	s.StateLock.RUnlock()

	return shipsInCoords, nil
}

func (s *Ships) GetShipsInBoxDebug(bbox [2][2]float64, geocache *Geocache) ([]*State, error) {

	if !validBbox(bbox) {
		return nil, fmt.Errorf("bounding box out of range")
	}

	totalTime := time.Now()

	binaryTime := time.Now()

	begin, end, err := geocache.BinarySearch(bbox)
	if err != nil {
		return nil, err
	}

	// If begin is greater than end, ship results should loop around geocache list.
	var binaryShipResults []int
	if begin > end {
		for i := begin; i < len(geocache.List); i++ {
			binaryShipResults = append(binaryShipResults, geocache.List[i].MMSI)
		}
		for i := 0; i <= end; i++ {
			binaryShipResults = append(binaryShipResults, geocache.List[i].MMSI)
		}
	} else {
		for i := begin; i <= end; i++ {
			binaryShipResults = append(binaryShipResults, geocache.List[i].MMSI)
		}
	}
	binaryElapsed := time.Since(binaryTime).Microseconds()

	shipsInCoords := make([]*State, 0)

	s.StateLock.RLock()

	fineTime := time.Now()
	for _, mmsi := range binaryShipResults {
		ship := s.State[mmsi]
		if ship.LatLon[0] >= bbox[0][0] && ship.LatLon[0] < bbox[1][0] && ship.LatLon[1] >= bbox[0][1] && ship.LatLon[1] < bbox[1][1] {
			shipsInCoords = append(shipsInCoords, ship)
		}
	}
	fineElapsed := time.Since(fineTime).Microseconds()

	// This list contains all ships within the bbox by iterating over every one of them.
	// Used to validate results from binary search.
	// This should go into a debug function when moved into the seaspy program.
	controlTime := time.Now()
	var controlList []int
	for mmsi, ship := range s.State {
		if ship.LatLon[0] >= bbox[0][0] && ship.LatLon[0] < bbox[1][0] && ship.LatLon[1] >= bbox[0][1] && ship.LatLon[1] < bbox[1][1] {
			if ship.LastUpdate < geocache.LastUpdate {
				controlList = append(controlList, mmsi)
			}
		}
	}
	controlElapsed := time.Since(controlTime).Microseconds()

	s.StateLock.RUnlock()

	totalElapsed := time.Since(totalTime).Microseconds()

	missing := 0
	for _, mmsi := range controlList {
		if !slices.Contains(binaryShipResults, mmsi) {
			missing++
		}
	}
	if missing > 0 {
		fmt.Printf("of the %d binary results, %d from the control group are missing\n", len(binaryShipResults), missing)
	}

	fmt.Printf("ships: %d, binary: %d, fine: %d, ctrl: %d, total: %d\n",
		len(shipsInCoords),
		binaryElapsed,
		fineElapsed,
		controlElapsed,
		totalElapsed,
	)

	return shipsInCoords, nil
}

func validBbox(bbox [2][2]float64) bool {

	if bbox[0][0] > LATMAX || bbox[0][0] < LATMIN || bbox[1][0] > LATMAX || bbox[1][0] < LATMIN {
		return false
	}

	if bbox[0][1] > LNGMAX || bbox[0][1] < LNGMIN || bbox[1][1] > LNGMAX || bbox[1][1] < LNGMIN {
		return false
	}

	return true
}

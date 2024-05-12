package main

import (
	"slices"
	"time"
)

const SECONDS_IN_DAY = 86400

type Swabby struct {
	Enable        bool       `json:"enable"`
	ScheduleHours int        `json:"scheduleHours"`
	ExpiryDays    ExpiryDays `json:"expiryDays"`
	Quit          chan struct{}
	Done          chan struct{}
}

type ExpiryDays struct {
	DerelictShip int `json:"derelictShip"`
	RouteHistory int `json:"routeHistory"`
}

func NewSwabby(s Swabby) *Swabby {
	s.Quit = make(chan struct{})
	s.Done = make(chan struct{})
	return &s
}

func NewSwabbyDefaults() *Swabby {
	return &Swabby{
		Enable: true,
		ExpiryDays: ExpiryDays{
			DerelictShip: 7,
			RouteHistory: 7,
		},
		Quit: make(chan struct{}),
		Done: make(chan struct{}),
	}
}

func (s *Swabby) Cleanup(d *Dock) {
	if !s.Enable || s.ExpiryDays.DerelictShip == 0 && s.ExpiryDays.RouteHistory == 0 {
		<-s.Quit
		s.Done <- struct{}{}
		return
	}

	ticker := time.NewTicker(time.Hour * time.Duration(s.ScheduleHours))

	for {
		select {
		case <-s.Quit:
			s.Done <- struct{}{}
			return
		case <-ticker.C:
			if s.ExpiryDays.DerelictShip > 0 {
				s.derelictShips(d)
			}

			if s.ExpiryDays.RouteHistory > 0 {
				s.routeHistory(d)
			}
		}
	}
}

func (s *Swabby) routeHistory(d *Dock) {
	now := time.Now().UTC().Unix()

	d.Ships.HistoryLock.Lock()
	for mmsi, history := range d.Ships.History {
		for i, h := range history {
			if now-h.Timestamp > int64(s.ExpiryDays.RouteHistory*SECONDS_IN_DAY) {
				history = slices.Delete(history, i, len(history))
				break
			}
		}
		d.Ships.History[mmsi] = history
	}
	d.Ships.HistoryLock.Unlock()
}

func (s *Swabby) derelictShips(d *Dock) {
	now := time.Now().UTC().Unix()
	derelictShips := []int{}

	d.Ships.StateLock.Lock()
	for mmsi, ship := range d.Ships.State {
		if now-ship.LastUpdate > int64(s.ExpiryDays.DerelictShip*SECONDS_IN_DAY) {
			derelictShips = append(derelictShips, mmsi)
			delete(d.Ships.State, mmsi)
		}
	}
	d.Ships.StateLock.Unlock()

	d.Ships.InfoLock.Lock()
	d.Ships.HistoryLock.Lock()
	for _, mmsi := range derelictShips {
		delete(d.Ships.History, mmsi)
		delete(d.Ships.Info, mmsi)
	}
	d.Ships.InfoLock.Unlock()
	d.Ships.HistoryLock.Unlock()
}

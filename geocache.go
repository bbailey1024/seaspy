package main

import (
	"fmt"
	"time"

	"github.com/bbailey1024/geohash"
)

const (
	LATMAX = 90.0
	LATMIN = -90.0
	LNGMAX = 180.0
	LNGMIN = -180.0
)

type Geocache struct {
	Timer      int
	List       []GeoMMSI
	LastUpdate int64
	Quit       chan struct{}
	Done       chan struct{}
}

type GeoMMSI struct {
	MMSI    int
	Geohash uint64
}

func NewGeocache(t int) *Geocache {
	return &Geocache{
		Timer:      t,
		List:       []GeoMMSI{},
		LastUpdate: 0,
		Quit:       make(chan struct{}),
		Done:       make(chan struct{}),
	}
}

func (gc *Geocache) Run(s *Ships) {

	// Generate every second for the first five seconds.
	for i := 0; i < 5; i++ {
		gc.Generate(s)
		time.Sleep(time.Second * 1)
	}

	ticker := time.NewTicker(time.Duration(gc.Timer) * time.Second)

	for {
		select {
		case <-ticker.C:
			// n := time.Now()
			gc.Generate(s)
			// fmt.Printf("ships: %d, geocache generation: %dμs\n", len(s.State), time.Since(n).Microseconds())
		case <-gc.Quit:
			gc.Done <- struct{}{}
			return
		}
	}
}

func (gc *Geocache) Generate(s *Ships) {
	s.StateLock.RLock()
	defer s.StateLock.RUnlock()

	geoSortedMMSI := make([]GeoMMSI, 0, len(s.State))
	for mmsi, state := range s.State {
		geoSortedMMSI = append(geoSortedMMSI, GeoMMSI{MMSI: mmsi, Geohash: state.Geohash})
	}

	gc.LastUpdate = time.Now().Unix()

	quickSortGeohash(geoSortedMMSI, 0, len(geoSortedMMSI))

	gc.List = geoSortedMMSI
}

func (gc *Geocache) BinarySearch(bbox [2][2]float64) (int, int, error) {

	if len(gc.List) == 0 {
		return 0, 0, fmt.Errorf("geocache list is empty, binary search cannot be performed")
	}

	bboxHashSW := geohash.EncodeInt(bbox[0][0], bbox[0][1])
	bboxHashNE := geohash.EncodeInt(bbox[1][0], bbox[1][1])

	begin := gc.binarySearchSW(bboxHashSW)
	end := gc.binarySearchNE(bboxHashNE)

	return begin, end, nil
}

func (gc *Geocache) binarySearchSW(bboxSW uint64) int {
	mid := len(gc.List) / 2
	top := len(gc.List)

	begin := 0

	for {
		if gc.List[mid].Geohash >= bboxSW {
			if mid-1 < 0 || gc.List[mid-1].Geohash < bboxSW {
				begin = mid
				break
			}

			top = mid
			mid = mid / 2

		} else {
			if mid+1 >= len(gc.List) || gc.List[mid+1].Geohash > bboxSW {
				begin = mid + 1 // Does this break things? Should it just be mid?
				break
			}
			mid = mid + ((top - mid) / 2)
		}
	}

	return begin
}

func (gc *Geocache) binarySearchNE(bboxNE uint64) int {
	mid := len(gc.List) / 2
	top := len(gc.List)

	end := 0

	for {
		if gc.List[mid].Geohash < bboxNE {
			if mid+1 >= len(gc.List) || gc.List[mid+1].Geohash > bboxNE {
				end = mid
				break
			}
			mid = mid + ((top - mid) / 2)

		} else {
			if mid-1 < 0 || gc.List[mid-1].Geohash < bboxNE {
				end = mid - 1
				break
			}

			top = mid
			mid = mid / 2
		}
	}

	return end
}

func quickSortGeohash(list []GeoMMSI, begin int, end int) {

	// End sorting partition branch when finished.
	if begin-end == 0 {
		return
	}

	pivot := end - 1
	swap := begin

	for i := begin; i < pivot; i++ {
		if list[i].Geohash < list[pivot].Geohash {
			list[i], list[swap] = list[swap], list[i]
			swap++
		}
	}

	list[pivot], list[swap] = list[swap], list[pivot]

	quickSortGeohash(list, begin, swap)
	quickSortGeohash(list, swap+1, end)
}

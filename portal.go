package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Portal struct {
	ListenAddr string `json:"listenAddr"`
	HtmlDir    string `json:"htmlDir"`
}

type Google struct {
	Api string `json:"api"`
}

func ListenAndServe(ctx context.Context, dock *Dock, p Portal, g Google) {

	mux := http.NewServeMux()

	staticDir := filepath.Join(p.HtmlDir, "static")
	staticFs := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticFs))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		index(w, r, p.HtmlDir, g)
	})
	mux.HandleFunc("GET /shipCount", func(w http.ResponseWriter, r *http.Request) {
		shipCount(w, r, dock)
	})
	mux.HandleFunc("GET /shipDump", func(w http.ResponseWriter, r *http.Request) {
		shipDump(w, r, dock)
	})
	mux.HandleFunc("GET /shipInfoWindow/{mmsi}", func(w http.ResponseWriter, r *http.Request) {
		shipInfoWindow(w, r, dock)
	})
	mux.HandleFunc("GET /shipHistory/{mmsi}", func(w http.ResponseWriter, r *http.Request) {
		shipHistory(w, r, dock)
	})
	mux.HandleFunc("GET /ships/{sw}/{ne}", func(w http.ResponseWriter, r *http.Request) {
		shipsBbox(w, r, dock)
	})
	mux.HandleFunc("GET /searchFields", func(w http.ResponseWriter, r *http.Request) {
		searchFields(w, r, dock)
	})
	mux.HandleFunc("GET /shipMeta", shipMeta)

	server := &http.Server{Addr: p.ListenAddr, Handler: mux}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v\n", err.Error())
		}
	}()

	<-ctx.Done()

	serverCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := server.Shutdown(serverCtx)
	if err != nil {
		fmt.Printf("http server failed to shutdown: %s\n", err.Error())
	}
}

func index(w http.ResponseWriter, _ *http.Request, htmlDir string, g Google) {
	indexTemplate := filepath.Join(htmlDir, "templates", "index.html")
	tmpl, err := template.New("index.html").ParseFiles(indexTemplate)
	if err != nil {
		log.Fatalf("could not read index file: %s\n", err.Error())
	}

	err = tmpl.Execute(w, g)
	if err != nil {
		log.Fatalf("could not execute template: %s\n", err.Error())
	}
}

func shipCount(w http.ResponseWriter, _ *http.Request, d *Dock) {
	d.Ships.StateLock.RLock()
	l := len(d.Ships.State)
	d.Ships.StateLock.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(l)
	if err != nil {
		fmt.Printf("shipCount handler failed: %s\n", err.Error())
	}
}

func shipInfoWindow(w http.ResponseWriter, r *http.Request, d *Dock) {
	mmsiStr := r.PathValue("mmsi")
	if mmsiStr == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	mmsi, err := strconv.Atoi(mmsiStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	res, err := d.Ships.GetInfoWindow(mmsi)
	if err != nil {
		fmt.Printf("shipInfo handler failed: %s\n", err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		fmt.Printf("shipInfo handler failed: %s\n", err.Error())
	}
}

func shipDump(w http.ResponseWriter, _ *http.Request, d *Dock) {
	res, err := d.Ships.GetShipDump()
	if err != nil {
		fmt.Printf("shipDump handler failed: %s\n", err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		fmt.Printf("shipDump handler failed: %s\n", err.Error())
	}
}

func shipHistory(w http.ResponseWriter, r *http.Request, d *Dock) {
	mmsiStr := r.PathValue("mmsi")
	if mmsiStr == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	mmsi, err := strconv.Atoi(mmsiStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	res, err := d.Ships.GetShipHistory(mmsi)
	if err != nil {
		fmt.Printf("shipHistory handler failed: %s\n", err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		fmt.Printf("shipHistory handler failed: %s\n", err.Error())
	}
}

func shipsBbox(w http.ResponseWriter, r *http.Request, d *Dock) {
	sw := strings.Split(r.PathValue("sw"), ",")
	ne := strings.Split(r.PathValue("ne"), ",")

	if len(sw) != 2 || len(ne) != 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	bbox, err := generateBbox(sw, ne)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Printf("shipsBbox handler failed: %s\n", err.Error())
		return
	}

	res, err := d.Ships.GetShipsInBox(bbox, d.Cache.Geo)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Printf("shipsBbox handler failed: %s\n", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(res)
	if err != nil {
		fmt.Printf("shipsBbox handler failed: %s\n", err.Error())
	}
}

func searchFields(w http.ResponseWriter, _ *http.Request, d *Dock) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(d.Cache.Search.List)
	if err != nil {
		fmt.Printf("searchFields handler failed: %s\n", err.Error())
	}
}

func shipMeta(w http.ResponseWriter, _ *http.Request) {
	shipmeta := ShipMetadata{
		ShipType:  ShipTypes,
		ShipGroup: ShipTypeGroups,
		NavStatus: NavStatus,
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(shipmeta)
	if err != nil {
		fmt.Printf("shipMeta handler failed: %s\n", err.Error())
	}
}

func generateBbox(sw []string, ne []string) ([2][2]float64, error) {
	bbox := [2][2]float64{}
	var err error

	bbox[0][0], err = strconv.ParseFloat(sw[0], 64)
	if err != nil {
		return bbox, err
	}

	bbox[0][1], err = strconv.ParseFloat(sw[1], 64)
	if err != nil {
		return bbox, err
	}

	bbox[1][0], err = strconv.ParseFloat(ne[0], 64)
	if err != nil {
		return bbox, err
	}

	bbox[1][1], err = strconv.ParseFloat(ne[1], 64)
	if err != nil {
		return bbox, err
	}

	return bbox, nil
}

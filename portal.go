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
	mux.HandleFunc("GET /ships", func(w http.ResponseWriter, r *http.Request) {
		ships(w, r, dock)
	})
	mux.HandleFunc("GET /shipDump", func(w http.ResponseWriter, r *http.Request) {
		shipDump(w, r, dock)
	})
	mux.HandleFunc("GET /shipInfo/{mmsi}", func(w http.ResponseWriter, r *http.Request) {
		shipInfo(w, r, dock)
	})
	mux.HandleFunc("GET /shipHistory/{mmsi}", func(w http.ResponseWriter, r *http.Request) {
		shipHistory(w, r, dock)
	})
	mux.HandleFunc("GET /geoList", func(w http.ResponseWriter, r *http.Request) {
		geoList(w, r, dock)
	})
	mux.HandleFunc("GET /ships/{sw}/{ne}", func(w http.ResponseWriter, r *http.Request) {
		shipsBbox(w, r, dock)
	})

	mux.HandleFunc("GET /shipTypes", shipTypes)
	mux.HandleFunc("GET /shipGroups", shipGroups)

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
	fmt.Fprint(w, len(d.Ships.State))
	d.Ships.StateLock.RUnlock()
}

func ships(w http.ResponseWriter, _ *http.Request, d *Dock) {
	res, err := d.Ships.GetShips()
	if err != nil {
		log.Fatalf("ships handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, res)
}

func shipInfo(w http.ResponseWriter, r *http.Request, d *Dock) {
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

	res, err := d.Ships.GetShipInfo(mmsi)
	if err != nil {
		log.Fatalf("shipInfo handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, res)
}

func shipDump(w http.ResponseWriter, _ *http.Request, d *Dock) {
	res, err := d.Ships.GetShipDump()
	if err != nil {
		log.Fatalf("shipDump handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, res)
}

func shipHistory(w http.ResponseWriter, r *http.Request, d *Dock) {

	if !d.ShipHistory {
		fmt.Fprint(w, "[]")
		return
	}

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
		log.Fatalf("shipHistory handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, res)
}

func geoList(w http.ResponseWriter, _ *http.Request, d *Dock) {
	b, err := json.Marshal(d.Geocache.List)
	if err != nil {
		log.Fatalf("geoList handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, string(b))
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
		return
	}

	res, err := d.Ships.GetShipsInBoxDebug(bbox, d.Geocache)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	fmt.Fprint(w, res)
}

func shipTypes(w http.ResponseWriter, _ *http.Request) {
	b, err := json.Marshal(ShipTypes)
	if err != nil {
		log.Fatalf("shipTypes handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, string(b))
}

func shipGroups(w http.ResponseWriter, _ *http.Request) {
	b, err := json.Marshal(ShipTypeGroups)
	if err != nil {
		log.Fatalf("shipTypes handler failed: %s\n", err.Error())
	}

	fmt.Fprint(w, string(b))
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

package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
)

type Portal struct {
	ListenAddr string `json:"listenAddr"`
}

type Google struct {
	Api string `json:"api"`
}

func ListenAndServe(addr string, dock *Dock, g Google) *http.Server {

	mux := http.NewServeMux()

	staticFs := http.FileServer(http.Dir("./html/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticFs))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		index(w, r, g)
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

	mux.HandleFunc("GET /shipTypes", shipTypes)
	mux.HandleFunc("GET /shipGroups", shipGroups)

	server := &http.Server{Addr: "127.0.0.1:8080", Handler: mux}
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v\n", err.Error())
		}
	}()
	return server
}

func index(w http.ResponseWriter, _ *http.Request, g Google) {
	tmpl, err := template.New("index.html").ParseFiles("./html/templates/index.html")
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
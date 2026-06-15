package main

import (
	_ "embed"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/oschwald/geoip2-golang"
	"xip/xip"
)

//go:embed dashboard.html
var dashboardHTML []byte

// geoIPAdapter adapts a *geoip2.Reader (MaxMind GeoLite2) to the
// xip.CountryLookerUpper interface so the xip package needn't import geoip2.
type geoIPAdapter struct{ reader *geoip2.Reader }

func (g geoIPAdapter) Country(ip net.IP) (string, error) {
	rec, err := g.reader.Country(ip)
	if err != nil {
		return "", err
	}
	return rec.Country.IsoCode, nil
}

// openGeoIP opens the GeoLite2-Country database at path. It returns a closer to
// release the file on shutdown. On failure it logs a warning and returns a nil
// lookerUpper so country stats are simply disabled rather than fatal.
func openGeoIP(path string) (lookerUpper xip.CountryLookerUpper, closer func() error) {
	reader, err := geoip2.Open(path)
	if err != nil {
		log.Printf("geoip: warning: could not open %q: %v (country stats disabled)", path, err)
		return nil, nil
	}
	log.Printf("geoip: loaded country database %q", path)
	return geoIPAdapter{reader: reader}, reader.Close
}

// startDashboard serves the stats UI ("/") and the JSON stats API
// ("/stats.json") on addr. It blocks, so call it in a goroutine.
func startDashboard(addr string, x *xip.Xip) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(dashboardHTML)
	})
	mux.HandleFunc("/stats.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(x.Stats.Snapshot())
	})
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("dashboard: listening on http://%s (stats JSON at /stats.json)", addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("dashboard: server stopped: %v", err)
	}
}

// startStatsPersister periodically snapshots the stats to path so they survive
// a restart. It blocks, so call it in a goroutine.
func startStatsPersister(path string, x *xip.Xip, interval time.Duration) {
	for {
		time.Sleep(interval)
		if err := x.Stats.Save(path); err != nil {
			log.Printf("stats: could not save to %q: %v", path, err)
		}
	}
}

package xip

import (
	"errors"
	"net"
	"testing"
)

type fakeGeoIP struct {
	code string
	err  error
}

func (f fakeGeoIP) Country(net.IP) (string, error) { return f.code, f.err }

func TestLookupCountry(t *testing.T) {
	ip := net.ParseIP("1.2.3.4")

	// No GeoIP database configured -> unknown ("")
	x := &Xip{}
	if got := x.lookupCountry(ip); got != "" {
		t.Errorf("nil GeoIP: want %q, got %q", "", got)
	}

	// nil source IP -> unknown ("")
	x.GeoIP = fakeGeoIP{code: "US"}
	if got := x.lookupCountry(nil); got != "" {
		t.Errorf("nil IP: want %q, got %q", "", got)
	}

	// Successful lookup returns the ISO code
	if got := x.lookupCountry(ip); got != "US" {
		t.Errorf("lookup: want %q, got %q", "US", got)
	}

	// Lookup error degrades to unknown ("")
	x.GeoIP = fakeGeoIP{err: errors.New("boom")}
	if got := x.lookupCountry(ip); got != "" {
		t.Errorf("error: want %q, got %q", "", got)
	}
}

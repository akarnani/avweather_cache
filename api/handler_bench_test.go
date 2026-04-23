package api

import (
	"encoding/xml"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/andrew/avweather_cache/cache"
	"github.com/andrew/avweather_cache/models"
)

// loadRealStations reads testdata/metars.cache.xml and returns the parsed
// METARs. ObservationTime on every record is rewritten to time.Now() so that
// age filtering doesn't discard everything when the test data ages.
func loadRealStations(tb testing.TB) []models.METAR {
	tb.Helper()
	data, err := os.ReadFile("../testdata/metars.cache.xml")
	if err != nil {
		tb.Fatalf("failed to read testdata: %v", err)
	}
	var resp models.MetarResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		tb.Fatalf("failed to parse testdata: %v", err)
	}
	now := time.Now()
	for i := range resp.Data {
		resp.Data[i].ObservationTime = now
	}
	return resp.Data
}

// The following benchmarks characterize findNearest at increasing search
// radii over a realistic dataset (~5000 stations from testdata). Query point
// is KPHL (39.8722, -75.2408).
//
// Radius shapes the cost: a small radius rejects most stations at the
// bounding-box check (cheap compare), while a large radius forces haversine
// on everything inside the box.
//
// Run with:   go test -bench=. -benchmem ./api/

// sliceIter adapts a slice to the stationIter signature for benchmarking.
func sliceIter(stations []models.METAR) stationIter {
	return func(fn func(models.METAR) bool) {
		for _, m := range stations {
			if !fn(m) {
				return
			}
		}
	}
}

func benchmarkFindNearest(b *testing.B, radiusMi float64) {
	stations := loadRealStations(b)
	iter := sliceIter(stations)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = findNearest(iter, 39.8722, -75.2408, radiusMi, time.Hour)
	}
}

func BenchmarkFindNearest_10mi(b *testing.B)   { benchmarkFindNearest(b, 10) }
func BenchmarkFindNearest_50mi(b *testing.B)   { benchmarkFindNearest(b, 50) }
func BenchmarkFindNearest_500mi(b *testing.B)  { benchmarkFindNearest(b, 500) }
func BenchmarkFindNearest_5000mi(b *testing.B) { benchmarkFindNearest(b, 5000) }

// BenchmarkNearestHandler exercises the full HTTP path (parse + findNearest +
// JSON encode). Gives a ceiling for real request latency.
func BenchmarkNearestHandler(b *testing.B) {
	c := cache.New("http://example.com", time.Minute)
	stations := loadRealStations(b)
	data := make(map[string]models.METAR, len(stations))
	for _, m := range stations {
		data[m.StationID] = m
	}
	c.SetDataForTest(data)
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar/nearest?lat=39.8722&lon=-75.2408&max_range_mi=50&max_age=1h", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		h.NearestHandler(w, req)
	}
}

func BenchmarkHaversine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = haversine(39.8722, -75.2408, 40.6413, -73.7781)
	}
}

func BenchmarkBoundingBox(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _, _, _ = boundingBox(39.8722, -75.2408, 50)
	}
}

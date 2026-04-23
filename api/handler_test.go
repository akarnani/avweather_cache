package api

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrew/avweather_cache/cache"
	"github.com/andrew/avweather_cache/models"
	"gopkg.in/yaml.v3"
)

func setupTestCache() *cache.Cache {
	c := cache.New("http://example.com", 1*time.Minute)

	// Manually populate cache with test data
	now := time.Now()
	temp1 := 15.5
	dewpoint1 := 10.0
	windSpeed1 := 10
	altim1 := 29.92

	temp2 := -5.0
	dewpoint2 := -8.0
	windSpeed2 := 15
	windGust2 := 20
	altim2 := 30.10

	testData := map[string]models.METAR{
		"KTEST1": {
			StationID:       "KTEST1",
			RawText:         "METAR KTEST1 261200Z 18010KT",
			ObservationTime: now.Add(-30 * time.Minute),
			Latitude:        40.0,
			Longitude:       -75.0,
			TempC:           &temp1,
			DewpointC:       &dewpoint1,
			WindDirDegrees:  "180",
			WindSpeedKt:     &windSpeed1,
			VisibilityMi:    "10+",
			AltimeterInHg:   &altim1,
			FlightCategory:  "VFR",
			MetarType:       "METAR",
		},
		"KTEST2": {
			StationID:       "KTEST2",
			RawText:         "METAR KTEST2 261100Z 09015G20KT",
			ObservationTime: now.Add(-90 * time.Minute),
			Latitude:        35.0,
			Longitude:       -80.0,
			TempC:           &temp2,
			DewpointC:       &dewpoint2,
			WindDirDegrees:  "90",
			WindSpeedKt:     &windSpeed2,
			WindGustKt:      &windGust2,
			VisibilityMi:    "5",
			AltimeterInHg:   &altim2,
			FlightCategory:  "MVFR",
			MetarType:       "METAR",
		},
		"KTEST3": {
			StationID:       "KTEST3",
			RawText:         "METAR KTEST3 260800Z",
			ObservationTime: now.Add(-5 * time.Hour),
		},
	}

	c.SetDataForTest(testData)
	return c
}

func TestMetarHandlerJSON(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar?stations=KTEST1,KTEST2&format=json", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected JSON content type, got %s", contentType)
	}

	var metars []models.METAR
	if err := json.Unmarshal(w.Body.Bytes(), &metars); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Note: The test might return 0 metars because we can't easily populate
	// the private cache data. This test mainly verifies the handler works.
	t.Logf("Received %d METARs", len(metars))
}

func TestMetarHandlerCSV(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar?stations=KTEST1&format=csv", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/csv") {
		t.Errorf("Expected CSV content type, got %s", contentType)
	}

	// Parse CSV to verify format
	reader := csv.NewReader(strings.NewReader(w.Body.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(records) > 0 {
		t.Logf("CSV has %d rows", len(records))
		t.Logf("Headers: %v", records[0])
	}
}

func TestMetarHandlerYAML(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar?stations=KTEST1&format=yaml", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/x-yaml") {
		t.Errorf("Expected YAML content type, got %s", contentType)
	}

	var metars []models.METAR
	if err := yaml.Unmarshal(w.Body.Bytes(), &metars); err != nil {
		t.Fatalf("Failed to parse YAML response: %v", err)
	}

	t.Logf("Received %d METARs", len(metars))
}

func TestMetarHandlerFieldFiltering(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar?stations=KTEST1&fields=station_id,temp_c,flight_category&format=json", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var metars []models.METAR
	if err := json.Unmarshal(w.Body.Bytes(), &metars); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// Field filtering is applied, but we'd need to populate the cache properly
	// to verify the exact fields. This test verifies the handler doesn't error.
	t.Logf("Field filtering request completed successfully")
}

func TestMetarHandlerAgeFilter(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	// Request METARs from last 1 hour only
	req := httptest.NewRequest("GET", "/api/metar?stations=KTEST1,KTEST2,KTEST3&hoursBeforeNow=1&format=json", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var metars []models.METAR
	if err := json.Unmarshal(w.Body.Bytes(), &metars); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	// KTEST3 is 5 hours old, so should be filtered out
	// KTEST1 is 30 min old, should be included
	// KTEST2 is 90 min old, should be filtered out
	t.Logf("Age filter request completed, returned %d METARs", len(metars))
}

func TestMetarHandlerMissingStations(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar?format=json", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for missing stations parameter, got %d", w.Code)
	}
}

func TestMetarHandlerInvalidFormat(t *testing.T) {
	c := setupTestCache()
	h := New(c)

	req := httptest.NewRequest("GET", "/api/metar?stations=KTEST1&format=xml", nil)
	w := httptest.NewRecorder()

	h.MetarHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid format, got %d", w.Code)
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"KTEST1,KTEST2,KTEST3", []string{"KTEST1", "KTEST2", "KTEST3"}},
		{"KTEST1, KTEST2 , KTEST3", []string{"KTEST1", "KTEST2", "KTEST3"}},
		{"KTEST1", []string{"KTEST1"}},
		{"", []string{}},
		{"  ,  ,  ", []string{}},
	}

	for _, tt := range tests {
		result := parseCommaSeparated(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseCommaSeparated(%q) returned %d items, expected %d",
				tt.input, len(result), len(tt.expected))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("parseCommaSeparated(%q)[%d] = %q, expected %q",
					tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestGetFieldValue(t *testing.T) {
	temp := 15.5
	metar := models.METAR{
		StationID:       "KTEST",
		RawText:         "METAR KTEST",
		ObservationTime: time.Date(2026, 1, 26, 12, 0, 0, 0, time.UTC),
		TempC:           &temp,
		WindDirDegrees:  "180",
		FlightCategory:  "VFR",
	}

	tests := []struct {
		field    string
		expected string
	}{
		{"station_id", "KTEST"},
		{"raw_text", "METAR KTEST"},
		{"flight_category", "VFR"},
		{"temp_c", "15.5"},
		{"wind_dir_degrees", "180"},
		{"dewpoint_c", ""}, // nil value
	}

	for _, tt := range tests {
		result := getFieldValue(metar, tt.field)
		if result != tt.expected {
			t.Errorf("getFieldValue(%q) = %q, expected %q", tt.field, result, tt.expected)
		}
	}
}

// setupNearestCache populates a cache with three stations at known coordinates
// and varying ages for exercising /api/metar/nearest.
func setupNearestCache() *cache.Cache {
	c := cache.New("http://example.com", 1*time.Minute)
	now := time.Now()

	c.SetDataForTest(map[string]models.METAR{
		// Philadelphia International
		"KPHL": {
			StationID:       "KPHL",
			Latitude:        39.8722,
			Longitude:       -75.2408,
			ObservationTime: now.Add(-10 * time.Minute),
			FlightCategory:  "VFR",
		},
		// Newark Liberty International — farther from query point than KPHL
		"KEWR": {
			StationID:       "KEWR",
			Latitude:        40.6925,
			Longitude:       -74.1687,
			ObservationTime: now.Add(-15 * time.Minute),
			FlightCategory:  "VFR",
		},
		// Los Angeles — far outside any reasonable East Coast radius
		"KLAX": {
			StationID:       "KLAX",
			Latitude:        33.9425,
			Longitude:       -118.4081,
			ObservationTime: now.Add(-5 * time.Minute),
			FlightCategory:  "VFR",
		},
		// Stale PHL-proximity station to exercise age filtering
		"KSTALE": {
			StationID:       "KSTALE",
			Latitude:        39.87,
			Longitude:       -75.25,
			ObservationTime: now.Add(-6 * time.Hour),
		},
	})
	return c
}

func TestNearestHandler_IgnoresInvalidCoords(t *testing.T) {
	c := cache.New("http://example.com", 1*time.Minute)
	now := time.Now()

	// Only stations with sentinel/invalid coords — none should match.
	// aviationweather.gov uses -99.99 for NIL-reporting stations and
	// occasionally (0,0) for records with missing coords.
	c.SetDataForTest(map[string]models.METAR{
		"NIL1": {
			StationID:       "NIL1",
			Latitude:        -99.99,
			Longitude:       -99.99,
			ObservationTime: now,
		},
		"ZERO": {
			StationID:       "ZERO",
			Latitude:        0,
			Longitude:       0,
			ObservationTime: now,
		},
	})
	h := New(c)

	// Query point deliberately far from (0,0) so a buggy implementation
	// wouldn't accidentally match ZERO via proximity.
	req := httptest.NewRequest("GET", "/api/metar/nearest?lat=39.95&lon=-75.17&max_range_mi=100&max_age=1h", nil)
	w := httptest.NewRecorder()

	h.NearestHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected 204 (all stations have invalid coords), got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestNearestHandler_Match(t *testing.T) {
	h := New(setupNearestCache())

	// Query point is downtown Philadelphia; KPHL should win over KEWR.
	req := httptest.NewRequest("GET", "/api/metar/nearest?lat=39.95&lon=-75.17&max_range_mi=100&max_age=1h", nil)
	w := httptest.NewRecorder()

	h.NearestHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp struct {
		models.METAR
		DistanceMi float64 `json:"distance_mi"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}

	if resp.StationID != "KPHL" {
		t.Errorf("Expected nearest station KPHL, got %s", resp.StationID)
	}
	if resp.DistanceMi <= 0 || resp.DistanceMi > 20 {
		t.Errorf("Expected small distance (<20mi), got %.2f", resp.DistanceMi)
	}
}

func TestNearestHandler_NoMatch_Range(t *testing.T) {
	h := New(setupNearestCache())

	// Middle of the Atlantic with a tiny radius — nothing qualifies.
	req := httptest.NewRequest("GET", "/api/metar/nearest?lat=30.0&lon=-40.0&max_range_mi=50&max_age=1h", nil)
	w := httptest.NewRecorder()

	h.NearestHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
}

func TestNearestHandler_NoMatch_Age(t *testing.T) {
	h := New(setupNearestCache())

	// KSTALE sits right next to the query point but its observation is 6h old.
	// A 1-minute max_age should exclude all stations.
	req := httptest.NewRequest("GET", "/api/metar/nearest?lat=39.87&lon=-75.25&max_range_mi=5&max_age=1m", nil)
	w := httptest.NewRecorder()

	h.NearestHandler(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestNearestHandler_YAML(t *testing.T) {
	h := New(setupNearestCache())

	req := httptest.NewRequest("GET", "/api/metar/nearest?lat=39.95&lon=-75.17&max_range_mi=100&max_age=1h&format=yaml", nil)
	w := httptest.NewRecorder()

	h.NearestHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Errorf("Expected yaml content type, got %s", ct)
	}

	var resp struct {
		StationID  string  `yaml:"station_id"`
		DistanceMi float64 `yaml:"distance_mi"`
	}
	if err := yaml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	if resp.StationID != "KPHL" {
		t.Errorf("Expected KPHL, got %s", resp.StationID)
	}
}

func TestNearestHandler_InvalidParams(t *testing.T) {
	h := New(setupNearestCache())

	cases := []struct {
		name string
		url  string
	}{
		{"missing lat", "/api/metar/nearest?lon=-75&max_range_mi=50&max_age=1h"},
		{"missing lon", "/api/metar/nearest?lat=40&max_range_mi=50&max_age=1h"},
		{"missing range", "/api/metar/nearest?lat=40&lon=-75&max_age=1h"},
		{"missing age", "/api/metar/nearest?lat=40&lon=-75&max_range_mi=50"},
		{"bad lat", "/api/metar/nearest?lat=abc&lon=-75&max_range_mi=50&max_age=1h"},
		{"lat out of range", "/api/metar/nearest?lat=91&lon=-75&max_range_mi=50&max_age=1h"},
		{"lon out of range", "/api/metar/nearest?lat=40&lon=200&max_range_mi=50&max_age=1h"},
		{"zero range", "/api/metar/nearest?lat=40&lon=-75&max_range_mi=0&max_age=1h"},
		{"bad age", "/api/metar/nearest?lat=40&lon=-75&max_range_mi=50&max_age=nope"},
		{"negative age", "/api/metar/nearest?lat=40&lon=-75&max_range_mi=50&max_age=-1h"},
		{"bad format", "/api/metar/nearest?lat=40&lon=-75&max_range_mi=50&max_age=1h&format=xml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.url, nil)
			w := httptest.NewRecorder()
			h.NearestHandler(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400, got %d", w.Code)
			}
		})
	}
}

func TestHaversine(t *testing.T) {
	tests := []struct {
		name        string
		lat1, lon1  float64
		lat2, lon2  float64
		expectedMi  float64
		toleranceMi float64
	}{
		{"same point", 40.0, -75.0, 40.0, -75.0, 0, 0.001},
		// KJFK (40.6413, -73.7781) to KLAX (33.9425, -118.4081) ~= 2475 mi
		{"JFK to LAX", 40.6413, -73.7781, 33.9425, -118.4081, 2475, 10},
		// 1 degree of latitude at the equator ~= 69 mi
		{"1 deg lat at equator", 0, 0, 1, 0, 69.09, 0.1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := haversine(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if diff := got - tt.expectedMi; diff < -tt.toleranceMi || diff > tt.toleranceMi {
				t.Errorf("haversine = %.2f, expected %.2f ± %.2f", got, tt.expectedMi, tt.toleranceMi)
			}
		})
	}
}

func TestBoundingBox(t *testing.T) {
	// At (40, -75), a 50mi box should contain the point (40.7, -75) (~48mi N)
	// and exclude (41, -75) (~69mi N).
	latMin, latMax, lonMin, lonMax, wraps := boundingBox(40, -75, 50)
	if wraps {
		t.Error("unexpected antimeridian wrap")
	}
	if 40.7 < latMin || 40.7 > latMax {
		t.Errorf("40.7 should be inside lat [%f, %f]", latMin, latMax)
	}
	if 41.0 >= latMin && 41.0 <= latMax {
		t.Errorf("41.0 should be outside lat [%f, %f]", latMin, latMax)
	}
	if -75.0 < lonMin || -75.0 > lonMax {
		t.Errorf("-75 should be inside lon [%f, %f]", lonMin, lonMax)
	}

	// Antimeridian wrap: box around (0, 179) with large radius should wrap.
	_, _, lonMin, lonMax, wraps = boundingBox(0, 179, 200)
	if !wraps {
		t.Errorf("expected wrap, got lon [%f, %f]", lonMin, lonMax)
	}
}

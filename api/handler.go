package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/andrew/avweather_cache/cache"
	"github.com/andrew/avweather_cache/metrics"
	"github.com/andrew/avweather_cache/models"
	"gopkg.in/yaml.v3"
)

// Earth radius in statute miles, used for haversine and bounding-box math.
const earthRadiusMi = 3958.7613

// milesPerDegreeLat is approximate; Earth isn't a perfect sphere but this
// is well within the tolerance a bounding-box prefilter needs.
const milesPerDegreeLat = 69.0934

// Handler handles API requests
type Handler struct {
	cache *cache.Cache
}

// New creates a new API handler
func New(c *cache.Cache) *Handler {
	return &Handler{cache: c}
}

// MetarHandler handles GET /api/metar requests
func (h *Handler) MetarHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		metrics.QueryLatency.Observe(time.Since(start).Seconds())
		metrics.TotalQueries.Inc()
	}()

	// Parse query parameters
	stationsParam := r.URL.Query().Get("stations")
	if stationsParam == "" {
		http.Error(w, "stations parameter is required", http.StatusBadRequest)
		return
	}

	stations := parseCommaSeparated(stationsParam)
	fieldsParam := r.URL.Query().Get("fields")
	var fields []string
	if fieldsParam != "" {
		fields = parseCommaSeparated(fieldsParam)
	}

	hoursBeforeNow := 0
	if hoursParam := r.URL.Query().Get("hoursBeforeNow"); hoursParam != "" {
		h, err := strconv.Atoi(hoursParam)
		if err != nil {
			http.Error(w, "invalid hoursBeforeNow parameter", http.StatusBadRequest)
			return
		}
		hoursBeforeNow = h
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" && format != "yaml" {
		http.Error(w, "invalid format parameter (must be json, csv, or yaml)", http.StatusBadRequest)
		return
	}

	// Get data from cache
	allMetars := h.cache.Get(stations)

	// Track metrics for each requested station
	for _, station := range stations {
		metrics.QueriesByStation.WithLabelValues(station).Inc()
	}

	// Filter by age if specified
	var filteredMetars []models.METAR
	cutoffTime := time.Now().Add(-time.Duration(hoursBeforeNow) * time.Hour)

	foundStations := make(map[string]bool)
	for _, metar := range allMetars {
		foundStations[metar.StationID] = true
		if hoursBeforeNow == 0 || metar.ObservationTime.After(cutoffTime) {
			filteredMetars = append(filteredMetars, metar)
		} else {
			metrics.StationsFilteredByAge.WithLabelValues(metar.StationID).Inc()
		}
	}

	// Track stations not in cache
	for _, station := range stations {
		if !foundStations[station] {
			metrics.StationsNotCached.WithLabelValues(station).Inc()
		}
	}

	// Filter fields if specified
	if len(fields) > 0 {
		filteredMetars = filterFields(filteredMetars, fields)
	}

	// Return in requested format
	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(filteredMetars); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode JSON: %v", err), http.StatusInternalServerError)
			return
		}
	case "yaml":
		w.Header().Set("Content-Type", "application/x-yaml")
		if err := yaml.NewEncoder(w).Encode(filteredMetars); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode YAML: %v", err), http.StatusInternalServerError)
			return
		}
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		if err := writeCSV(w, filteredMetars, fields); err != nil {
			http.Error(w, fmt.Sprintf("failed to write CSV: %v", err), http.StatusInternalServerError)
		}
	}
}

// parseCommaSeparated splits a comma-separated string into a slice
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// filterFields returns METARs with only the specified fields populated
func filterFields(metars []models.METAR, fields []string) []models.METAR {
	// Build a map of requested fields for quick lookup
	fieldMap := make(map[string]bool)
	for _, f := range fields {
		fieldMap[strings.ToLower(f)] = true
	}

	// Always include station_id as it's the key
	fieldMap["station_id"] = true

	result := make([]models.METAR, len(metars))
	for i, metar := range metars {
		filtered := models.METAR{StationID: metar.StationID}

		// Use reflection to copy only requested fields
		v := reflect.ValueOf(metar)
		fv := reflect.ValueOf(&filtered).Elem()
		t := v.Type()

		for j := 0; j < v.NumField(); j++ {
			field := t.Field(j)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "" {
				continue
			}
			// Extract field name from json tag
			fieldName := strings.Split(jsonTag, ",")[0]
			if fieldMap[strings.ToLower(fieldName)] {
				fv.Field(j).Set(v.Field(j))
			}
		}

		result[i] = filtered
	}

	return result
}

// writeCSV writes METARs to CSV format
func writeCSV(w http.ResponseWriter, metars []models.METAR, fields []string) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if len(metars) == 0 {
		return nil
	}

	// Determine fields to write
	var headers []string
	if len(fields) > 0 {
		headers = fields
	} else {
		// Use all fields from the first METAR
		headers = getAllFieldNames()
	}

	// Write header
	if err := writer.Write(headers); err != nil {
		return err
	}

	// Write data rows
	for _, metar := range metars {
		row := make([]string, len(headers))
		for i, header := range headers {
			row[i] = getFieldValue(metar, header)
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// getAllFieldNames returns all field names from METAR struct
func getAllFieldNames() []string {
	return []string{
		"station_id", "raw_text", "observation_time", "latitude", "longitude",
		"temp_c", "dewpoint_c", "wind_dir_degrees", "wind_speed_kt", "wind_gust_kt",
		"visibility_statute_mi", "altim_in_hg", "wx_string", "flight_category",
		"metar_type", "elevation_m", "precip_in",
	}
}

// getFieldValue retrieves a field value from a METAR as a string
func getFieldValue(metar models.METAR, fieldName string) string {
	switch strings.ToLower(fieldName) {
	case "station_id":
		return metar.StationID
	case "raw_text":
		return metar.RawText
	case "observation_time":
		if !metar.ObservationTime.IsZero() {
			return metar.ObservationTime.Format(time.RFC3339)
		}
		return ""
	case "latitude":
		return fmt.Sprintf("%.4f", metar.Latitude)
	case "longitude":
		return fmt.Sprintf("%.4f", metar.Longitude)
	case "temp_c":
		if metar.TempC != nil {
			return fmt.Sprintf("%.1f", *metar.TempC)
		}
		return ""
	case "dewpoint_c":
		if metar.DewpointC != nil {
			return fmt.Sprintf("%.1f", *metar.DewpointC)
		}
		return ""
	case "wind_dir_degrees":
		return metar.WindDirDegrees
	case "wind_speed_kt":
		if metar.WindSpeedKt != nil {
			return fmt.Sprintf("%d", *metar.WindSpeedKt)
		}
		return ""
	case "wind_gust_kt":
		if metar.WindGustKt != nil {
			return fmt.Sprintf("%d", *metar.WindGustKt)
		}
		return ""
	case "visibility_statute_mi":
		return metar.VisibilityMi
	case "altim_in_hg":
		if metar.AltimeterInHg != nil {
			return fmt.Sprintf("%.2f", *metar.AltimeterInHg)
		}
		return ""
	case "wx_string":
		return metar.WxString
	case "flight_category":
		return metar.FlightCategory
	case "metar_type":
		return metar.MetarType
	case "elevation_m":
		if metar.ElevationM != nil {
			return fmt.Sprintf("%.0f", *metar.ElevationM)
		}
		return ""
	case "precip_in":
		if metar.PrecipIn != nil {
			return fmt.Sprintf("%.2f", *metar.PrecipIn)
		}
		return ""
	default:
		return ""
	}
}

// nearestResponse embeds METAR and adds the computed distance.
// The yaml:",inline" tag is required because yaml.v3 (unlike encoding/json)
// does not auto-flatten anonymous fields.
type nearestResponse struct {
	models.METAR `yaml:",inline"`
	DistanceMi   float64 `json:"distance_mi" yaml:"distance_mi"`
}

// NearestHandler handles GET /api/metar/nearest
func (h *Handler) NearestHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		metrics.QueryLatency.Observe(time.Since(start).Seconds())
		metrics.TotalQueries.Inc()
		metrics.NearestQueries.Inc()
	}()

	q := r.URL.Query()

	lat, err := parseFloatParam(q, "lat")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if lat < -90 || lat > 90 {
		http.Error(w, "lat must be between -90 and 90", http.StatusBadRequest)
		return
	}

	lon, err := parseFloatParam(q, "lon")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if lon < -180 || lon > 180 {
		http.Error(w, "lon must be between -180 and 180", http.StatusBadRequest)
		return
	}

	maxRangeMi, err := parseFloatParam(q, "max_range_mi")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if maxRangeMi <= 0 {
		http.Error(w, "max_range_mi must be > 0", http.StatusBadRequest)
		return
	}

	maxAgeStr := q.Get("max_age")
	if maxAgeStr == "" {
		http.Error(w, "max_age parameter is required", http.StatusBadRequest)
		return
	}
	maxAge, err := time.ParseDuration(maxAgeStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid max_age: %v", err), http.StatusBadRequest)
		return
	}
	if maxAge <= 0 {
		http.Error(w, "max_age must be > 0", http.StatusBadRequest)
		return
	}

	format := q.Get("format")
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "yaml" {
		http.Error(w, "invalid format parameter (must be json or yaml)", http.StatusBadRequest)
		return
	}

	bestMetar, bestDist, found := findNearest(h.cache.ForEach, lat, lon, maxRangeMi, maxAge)

	if !found {
		metrics.NearestNoMatch.Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	metrics.NearestDistanceMi.Observe(bestDist)

	resp := nearestResponse{METAR: bestMetar, DistanceMi: bestDist}

	switch format {
	case "yaml":
		w.Header().Set("Content-Type", "application/x-yaml")
		if err := yaml.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode YAML: %v", err), http.StatusInternalServerError)
		}
	default:
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, fmt.Sprintf("failed to encode JSON: %v", err), http.StatusInternalServerError)
		}
	}
}

// stationIter yields METARs to the supplied callback, stopping early if the
// callback returns false. Cache.ForEach satisfies this signature directly.
type stationIter func(func(models.METAR) bool)

// findNearest searches stations for the one closest to (lat, lon) within
// maxRangeMi statute miles and younger than maxAge. Returns (bestMetar,
// bestDistanceMi, found). Takes an iterator rather than a slice so the
// handler can search directly over the cache's map without a snapshot copy.
func findNearest(iter stationIter, lat, lon, maxRangeMi float64, maxAge time.Duration) (models.METAR, float64, bool) {
	latMin, latMax, lonMin, lonMax, wrapsAntimeridian := boundingBox(lat, lon, maxRangeMi)
	cutoff := time.Now().Add(-maxAge)

	var best models.METAR
	bestDist := math.MaxFloat64
	found := false

	iter(func(m models.METAR) bool {
		// Skip stations with invalid coordinates. aviationweather.gov uses
		// -99.99 as a "missing" sentinel for NIL-reporting stations and (0,0)
		// for at least one record with missing coords. The range check also
		// catches any future sentinel the upstream feed introduces.
		if m.Latitude < -90 || m.Latitude > 90 || m.Longitude < -180 || m.Longitude > 180 {
			return true
		}
		if m.Latitude == 0 && m.Longitude == 0 {
			return true
		}
		if m.ObservationTime.Before(cutoff) {
			return true
		}
		if m.Latitude < latMin || m.Latitude > latMax {
			return true
		}
		if wrapsAntimeridian {
			if m.Longitude < lonMin && m.Longitude > lonMax {
				return true
			}
		} else {
			if m.Longitude < lonMin || m.Longitude > lonMax {
				return true
			}
		}
		d := haversine(lat, lon, m.Latitude, m.Longitude)
		if d > maxRangeMi {
			return true
		}
		if d < bestDist {
			bestDist = d
			best = m
			found = true
		}
		return true
	})
	return best, bestDist, found
}

func parseFloatParam(q url.Values, name string) (float64, error) {
	raw := q.Get(name)
	if raw == "" {
		return 0, fmt.Errorf("%s parameter is required", name)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %v", name, err)
	}
	return v, nil
}

// haversine returns the great-circle distance in statute miles between two
// lat/lon points given in decimal degrees.
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMi * c
}

// boundingBox returns a lat/lon rectangle that fully contains the circle of
// radius rangeMi around (lat, lon). wrapsAntimeridian is true when the box
// straddles the ±180° longitude line, in which case callers must use an OR
// check (lon >= lonMin OR lon <= lonMax) rather than AND.
func boundingBox(lat, lon, rangeMi float64) (latMin, latMax, lonMin, lonMax float64, wrapsAntimeridian bool) {
	dLat := rangeMi / milesPerDegreeLat
	latMin = lat - dLat
	latMax = lat + dLat

	// If the circle reaches a pole, longitude filtering is meaningless —
	// widen to the full range.
	if latMin <= -90 || latMax >= 90 {
		latMin = math.Max(latMin, -90)
		latMax = math.Min(latMax, 90)
		return latMin, latMax, -180, 180, false
	}

	cosLat := math.Cos(lat * math.Pi / 180)
	// Near-pole safety; cosLat ~ 0 would blow up dLon.
	if cosLat < 1e-6 {
		return latMin, latMax, -180, 180, false
	}

	dLon := rangeMi / (milesPerDegreeLat * cosLat)
	if dLon >= 180 {
		return latMin, latMax, -180, 180, false
	}

	lonMin = lon - dLon
	lonMax = lon + dLon
	if lonMin < -180 {
		lonMin += 360
		wrapsAntimeridian = true
	}
	if lonMax > 180 {
		lonMax -= 360
		wrapsAntimeridian = true
	}
	return latMin, latMax, lonMin, lonMax, wrapsAntimeridian
}

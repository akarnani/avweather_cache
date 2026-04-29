package webapp

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrew/avweather_cache/cache"
	"github.com/andrew/avweather_cache/models"
)

// Real station IDs are 3-4 chars; cap with headroom for substring searches.
const maxSearchLen = 32

//go:embed templates/dashboard.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

var dashboardTmpl = template.Must(
	template.New("dashboard.html").
		Funcs(templateFuncs).
		ParseFS(templateFS, "templates/dashboard.html"),
)

var templateFuncs = template.FuncMap{
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return "Never"
		}
		return t.Format("2006-01-02 15:04:05 MST")
	},
	"formatAge": func(t time.Time) string {
		if t.IsZero() {
			return "N/A"
		}
		age := time.Since(t)
		if age < time.Minute {
			return fmt.Sprintf("%.0fs ago", age.Seconds())
		} else if age < time.Hour {
			return fmt.Sprintf("%.0fm ago", age.Minutes())
		} else if age < 24*time.Hour {
			return fmt.Sprintf("%.1fh ago", age.Hours())
		}
		return fmt.Sprintf("%.1fd ago", age.Hours()/24)
	},
	"formatTemp": func(temp *float64) string {
		if temp == nil {
			return "N/A"
		}
		return fmt.Sprintf("%.1f°C", *temp)
	},
	"formatWind": func(dir string, speed *int, gust *int) string {
		if dir == "" || speed == nil {
			return "N/A"
		}
		result := fmt.Sprintf("%s° @ %dkt", dir, *speed)
		if gust != nil && *gust > 0 {
			result += fmt.Sprintf(" G%dkt", *gust)
		}
		return result
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
}

// StaticFS returns the embedded /static filesystem rooted at the static dir.
// Callers mount this under /static/ via http.StripPrefix.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		// Embed paths are validated at compile time; this is unreachable.
		panic(err)
	}
	return sub
}

// Handler handles web UI requests
type Handler struct {
	cache *cache.Cache
}

// New creates a new webapp handler
func New(c *cache.Cache) *Handler {
	return &Handler{cache: c}
}

// SearchHandler returns JSON data for AJAX search requests
func (h *Handler) SearchHandler(w http.ResponseWriter, r *http.Request) {
	metars := h.cache.GetAll()

	searchQuery := r.URL.Query().Get("search")
	if len(searchQuery) > maxSearchLen {
		searchQuery = searchQuery[:maxSearchLen]
	}
	var filteredMetars []models.METAR

	if searchQuery != "" {
		searchQuery = strings.ToUpper(strings.TrimSpace(searchQuery))
		for _, m := range metars {
			if strings.Contains(strings.ToUpper(m.StationID), searchQuery) {
				filteredMetars = append(filteredMetars, m)
			}
		}
	} else {
		filteredMetars = metars
	}

	sort.Slice(filteredMetars, func(i, j int) bool {
		return filteredMetars[i].ObservationTime.After(filteredMetars[j].ObservationTime)
	})

	pageSize := 100
	page := 1
	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	totalPages := (len(filteredMetars) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	if startIdx >= len(filteredMetars) {
		startIdx = 0
		page = 1
	}
	if endIdx > len(filteredMetars) {
		endIdx = len(filteredMetars)
	}

	var displayMetars []models.METAR
	if len(filteredMetars) > 0 {
		displayMetars = filteredMetars[startIdx:endIdx]
	}

	response := struct {
		Metars      []models.METAR `json:"metars"`
		MetarCount  int            `json:"metar_count"`
		TotalCount  int            `json:"total_count"`
		Page        int            `json:"page"`
		TotalPages  int            `json:"total_pages"`
		StartIdx    int            `json:"start_idx"`
		EndIdx      int            `json:"end_idx"`
		SearchQuery string         `json:"search_query"`
	}{
		Metars:      displayMetars,
		MetarCount:  len(filteredMetars),
		TotalCount:  len(metars),
		Page:        page,
		TotalPages:  totalPages,
		StartIdx:    startIdx + 1,
		EndIdx:      endIdx,
		SearchQuery: searchQuery,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("search: encode JSON: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// IndexHandler serves the main dashboard
func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	status := h.cache.Status()
	metars := h.cache.GetAll()

	searchQuery := r.URL.Query().Get("search")
	if len(searchQuery) > maxSearchLen {
		searchQuery = searchQuery[:maxSearchLen]
	}
	var filteredMetars []models.METAR

	if searchQuery != "" {
		searchQuery = strings.ToUpper(strings.TrimSpace(searchQuery))
		for _, m := range metars {
			if strings.Contains(strings.ToUpper(m.StationID), searchQuery) {
				filteredMetars = append(filteredMetars, m)
			}
		}
	} else {
		filteredMetars = metars
	}

	sort.Slice(filteredMetars, func(i, j int) bool {
		return filteredMetars[i].ObservationTime.After(filteredMetars[j].ObservationTime)
	})

	pageSize := 100
	page := 1
	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	totalPages := (len(filteredMetars) + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}
	startIdx := (page - 1) * pageSize
	endIdx := startIdx + pageSize

	if startIdx >= len(filteredMetars) {
		startIdx = 0
		page = 1
	}
	if endIdx > len(filteredMetars) {
		endIdx = len(filteredMetars)
	}

	var displayMetars []models.METAR
	if len(filteredMetars) > 0 {
		displayMetars = filteredMetars[startIdx:endIdx]
	}

	data := struct {
		Status      cache.Status
		Metars      any
		MetarCount  int
		TotalCount  int
		Page        int
		TotalPages  int
		PageSize    int
		StartIdx    int
		EndIdx      int
		SearchQuery string
	}{
		Status:      status,
		Metars:      displayMetars,
		MetarCount:  len(filteredMetars),
		TotalCount:  len(metars),
		Page:        page,
		TotalPages:  totalPages,
		PageSize:    pageSize,
		StartIdx:    startIdx + 1,
		EndIdx:      endIdx,
		SearchQuery: searchQuery,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTmpl.Execute(w, data); err != nil {
		log.Printf("index: execute template: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

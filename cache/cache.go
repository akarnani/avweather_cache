package cache

import (
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/andrew/avweather_cache/metrics"
	"github.com/andrew/avweather_cache/models"
)

// Cache holds METAR data in memory
type Cache struct {
	mu                 sync.RWMutex
	data               map[string]models.METAR
	sourceURL          string
	updateInterval     time.Duration
	lastSuccessfulPull time.Time
	lastPullAttempt    time.Time
	lastPullError      error
	ctx                context.Context
	cancel             context.CancelFunc
}

// New creates a new cache instance
func New(sourceURL string, updateInterval time.Duration) *Cache {
	ctx, cancel := context.WithCancel(context.Background())
	return &Cache{
		data:           make(map[string]models.METAR),
		sourceURL:      sourceURL,
		updateInterval: updateInterval,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start begins the periodic update process
func (c *Cache) Start() {
	// Do initial pull
	if err := c.update(); err != nil {
		log.Printf("Initial cache update failed: %v", err)
	}

	// Start periodic updates
	ticker := time.NewTicker(c.updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := c.update(); err != nil {
					log.Printf("Cache update failed: %v", err)
				}
			case <-c.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the periodic update process
func (c *Cache) Stop() {
	c.cancel()
}

// update fetches new data and merges it into the cache
func (c *Cache) update() error {
	c.lastPullAttempt = time.Now()
	metrics.LastPullAttemptAge.Set(0)

	log.Printf("Fetching METAR data from %s", c.sourceURL)

	// Fetch data
	resp, err := http.Get(c.sourceURL)
	if err != nil {
		c.lastPullError = err
		metrics.PullErrors.Inc()
		return fmt.Errorf("failed to fetch data: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		c.lastPullError = err
		metrics.PullErrors.Inc()
		return err
	}

	// Decompress gzip
	gzReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		c.lastPullError = err
		metrics.PullErrors.Inc()
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	// Read all data
	data, err := io.ReadAll(gzReader)
	if err != nil {
		c.lastPullError = err
		metrics.PullErrors.Inc()
		return fmt.Errorf("failed to read gzip data: %w", err)
	}

	// Parse XML
	var response models.MetarResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		c.lastPullError = err
		metrics.PullErrors.Inc()
		return fmt.Errorf("failed to parse XML: %w", err)
	}

	// Merge into cache (don't purge, just update/add)
	c.mu.Lock()
	for _, metar := range response.Data {
		if metar.StationID != "" {
			c.data[metar.StationID] = metar
		}
	}
	c.mu.Unlock()

	c.lastSuccessfulPull = time.Now()
	c.lastPullError = nil
	metrics.LastSuccessfulPullAge.Set(0)

	log.Printf("Successfully updated cache with %d METARs (total in cache: %d)", len(response.Data), len(c.data))

	// Update metrics
	c.updateMetrics()

	return nil
}

// Get retrieves METARs for the given stations
func (c *Cache) Get(stationIDs []string) []models.METAR {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]models.METAR, 0, len(stationIDs))
	for _, id := range stationIDs {
		if metar, ok := c.data[id]; ok {
			result = append(result, metar)
		}
	}
	return result
}

// SetDataForTest replaces the cache contents. Test-only helper so callers
// outside this package can populate the cache without going through HTTP.
func (c *Cache) SetDataForTest(data map[string]models.METAR) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
}

// GetAll returns all METARs in the cache
func (c *Cache) GetAll() []models.METAR {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]models.METAR, 0, len(c.data))
	for _, metar := range c.data {
		result = append(result, metar)
	}
	return result
}

// ForEach iterates every cached METAR under the read lock and invokes fn for
// each one. Iteration stops early if fn returns false. Callers must not
// block inside fn — the cache write lock is blocked for the duration of
// iteration, and concurrent readers can still proceed.
func (c *Cache) ForEach(fn func(models.METAR) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, m := range c.data {
		if !fn(m) {
			return
		}
	}
}

// Status returns cache status information
func (c *Cache) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return Status{
		TotalStations:      len(c.data),
		LastSuccessfulPull: c.lastSuccessfulPull,
		LastPullAttempt:    c.lastPullAttempt,
		LastPullError:      c.lastPullError,
	}
}

// Status represents the cache status
type Status struct {
	TotalStations      int
	LastSuccessfulPull time.Time
	LastPullAttempt    time.Time
	LastPullError      error
}

// updateMetrics updates prometheus metrics based on cache state
func (c *Cache) updateMetrics() {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var oldestAge time.Duration
	under1Hour := 0
	under2Hours := 0

	for _, metar := range c.data {
		age := now.Sub(metar.ObservationTime)
		if oldestAge == 0 || age > oldestAge {
			oldestAge = age
		}
		if age < time.Hour {
			under1Hour++
		}
		if age < 2*time.Hour {
			under2Hours++
		}
	}

	metrics.TotalStations.Set(float64(len(c.data)))
	metrics.StationsUnder1Hour.Set(float64(under1Hour))
	metrics.StationsUnder2Hours.Set(float64(under2Hours))
	if oldestAge > 0 {
		metrics.OldestMetarAge.Set(oldestAge.Seconds())
	}
}

// UpdateAgeMetrics should be called periodically to update age-based metrics
func (c *Cache) UpdateAgeMetrics() {
	now := time.Now()

	if !c.lastSuccessfulPull.IsZero() {
		metrics.LastSuccessfulPullAge.Set(now.Sub(c.lastSuccessfulPull).Seconds())
	}

	if !c.lastPullAttempt.IsZero() {
		metrics.LastPullAttemptAge.Set(now.Sub(c.lastPullAttempt).Seconds())
	}

	c.updateMetrics()
}

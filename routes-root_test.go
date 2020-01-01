package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pdxfixit/hostdb"
	"github.com/stretchr/testify/assert"
)

// GET /health
func TestGetHealth(t *testing.T) {

	w := makeTestGetRequest(t, "/health", false, nil)

	hostdbHealthCheck := hostdb.GetHealthResponse{}

	err := json.NewDecoder(w.Body).Decode(&hostdbHealthCheck)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, "up", hostdbHealthCheck.App, "HostDB app")
	assert.Equal(t, "present", hostdbHealthCheck.DB, "HostDB db")

}

// GET /stats
func TestGetStats(t *testing.T) {

	w := makeTestGetRequest(t, "/stats", false, nil)

	hostdbStats := hostdb.GetStatsResponse{}

	err := json.NewDecoder(w.Body).Decode(&hostdbStats)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.NotZero(t, hostdbStats.TotalRecords, "Total number of records")

	assert.NotEmpty(t, hostdbStats.NewestRecord, "newest timestamp")
	newest, err := time.Parse("2006-01-02 15:04:05", hostdbStats.NewestRecord)
	if err != nil {
		t.Errorf(err.Error())
	}
	assert.False(t, newest.IsZero(), "is newest_record time zero")

	assert.NotEmpty(t, hostdbStats.OldestRecord, "oldest timestamp")
	oldest, err := time.Parse("2006-01-02 15:04:05", hostdbStats.OldestRecord)
	if err != nil {
		t.Errorf(err.Error())
	}
	assert.False(t, oldest.IsZero(), "is oldest_record time zero")

	assert.True(t, oldest.Before(newest), "oldest before newest")

	assert.NotZero(t, len(hostdbStats.LastSeenCollectors), "number of collectors last seen")

}

// GET /version
func TestGetVersion(t *testing.T) {

	w := makeTestGetRequest(t, "/version", true, nil)

	version := hostdb.GetVersionResponse{}

	err := json.NewDecoder(w.Body).Decode(&version)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.IsType(t, hostdb.GetVersionResponse{}, version)
	assert.IsType(t, hostdb.ServerVersion{}, version.App)
	assert.IsType(t, hostdb.MariadbVersion{}, version.DB)

	assert.NotEmpty(t, version.App.APIVersion)
	assert.Contains(t, version.DB.Version, "10.3")

}

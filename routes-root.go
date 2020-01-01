package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pdxfixit/hostdb"
)

// most of these values are provided via go build.
// $ go build -x -ldflags="-X main.appVersion=0.1.815"
var appVersion string
var apiVersion = "0.3.2" // also found in openapi.yaml
var gitCommit string
var hostname string
var buildDate string
var buildURL string
var goVersion string

func redirectExamples(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "https://github.com/pdxfixit/hostdb-server/blob/master/EXAMPLES.md")
}

func redirectOpenAPISpec(c *gin.Context) {
	c.Redirect(http.StatusSeeOther, "https://github.com/pdxfixit/hostdb-server/blob/master/openapi.yaml")
}

func getGitCommit() string {
	return gitCommit
}

func getHostname() string {
	if len(hostname) != 0 {
		return hostname
	}

	if value, ok := os.LookupEnv("HOSTNAME"); ok {
		hostname = strings.ReplaceAll(value, "hostdb-server-", "")
	} else {
		hostname = "local"
	}

	return hostname
}

func getReleaseTimestamp() string {
	layout := "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, buildDate)

	if err != nil {
		log.Println(err)
		return ""
	}

	return t.Format("Jan 2 2006 @ 15:04:05")
}

// run a health check
func getHealth(c *gin.Context) {

	health := hostdb.GetHealthResponse{
		App: "up",
		DB:  "absent",
	}

	if checkMariadb() {
		health.DB = "present"
	}

	sendResponse(c, http.StatusOK, health)

}

// get HostDB statistics
func getStats(c *gin.Context) {

	stats := hostdb.GetStatsResponse{}

	// get hostname
	stats.Hostname = getHostname()

	// get total number of records
	count, err := getTotalRecords()
	if err != nil {
		sendResponse(c, http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"reason": "failed to get total number of records",
		})
		return
	}
	stats.TotalRecords = count

	// get newest record timestamp
	newest, err := getNewestTimestamp()
	if err != nil {
		sendResponse(c, http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"reason": "failed to get newest timestamp",
		})
		return
	}
	stats.NewestRecord = newest

	// get oldest record timestamp
	oldest, err := getOldestTimestamp()
	if err != nil {
		sendResponse(c, http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"reason": "failed to get oldest timestamp",
		})
		return
	}
	stats.OldestRecord = oldest

	// get last seen times
	lastSeen, err := getRecentCommitterTimestamps()
	if err != nil {
		sendResponse(c, http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"reason": "failed to get last seen times",
		})
		return
	}
	stats.LastSeenCollectors = lastSeen

	sendResponse(c, http.StatusOK, stats)

}

// show the hostdb and mariadb versions
func getVersion(c *gin.Context) {

	app := hostdb.ServerVersion{
		Version:    appVersion,
		APIVersion: apiVersion,
		Commit:     gitCommit,
		Date:       buildDate,
		BuildURL:   buildURL,
		GoVersion:  goVersion,
	}

	// get Mariadb version
	mariadbVersion, err := getMariadbVersion()
	if err != nil {
		sendResponse(c, http.StatusInternalServerError, gin.H{
			"error":  err.Error(),
			"reason": "failed to get Mariadb version",
		})
		return
	}

	db := hostdb.MariadbVersion{
		Version: mariadbVersion,
	}

	sendResponse(c, http.StatusOK, hostdb.GetVersionResponse{App: app, DB: db})

	return

}

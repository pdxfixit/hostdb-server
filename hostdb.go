package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/_integrations/nrgin/v1"
	"github.com/pdxfixit/hostdb"
	"github.com/thinkerou/favicon"
)

var config hostdb.GlobalConfig

func main() {

	//gin.SetMode(gin.ReleaseMode)
	log.SetPrefix("[hostdb] ")

	loadConfig()

	newrelicConfig := newrelic.NewConfig(config.Hostdb.NewRelicAppName, config.Hostdb.NewRelicLicenseKey)

	if config.Hostdb.Debug {
		newrelicConfig.Logger = newrelic.NewDebugLogger(os.Stdout)
	}

	app, err := newrelic.NewApplication(newrelicConfig)
	if err != nil {
		log.Fatal(err)
	}

	if err := loadMariadb(); err != nil {
		log.Fatal(err)
	}

	r := gin.Default()

	// Add the nrgin middleware before other middlewares or routes:
	r.Use(nrgin.Middleware(app))

	// define the routes
	r = setupRoutes(r)

	log.Println("HostDB has started up.")

	// listen and serve (usually on 0.0.0.0:8080)
	err = r.Run(config.Hostdb.Host + ":" + strconv.Itoa(config.Hostdb.Port))
	if err != nil {
		log.Fatal(err)
	}

}

// define the routes
func setupRoutes(r *gin.Engine) *gin.Engine {

	r.Use(favicon.New("assets/128.png"))
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	// allows all PDXfixIT origins https://github.com/gin-contrib/cors#default-allows-all-origins
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*.pdxfixit.com"}
	corsConfig.AllowWildcard = true
	r.Use(cors.New(corsConfig))

	// basics
	r.GET("/openapi.yaml", redirectOpenAPISpec)
	r.GET("/openapi/v3", redirectOpenAPISpec)
	r.GET("/health", getHealth)
	r.GET("/stats", getStats)
	r.GET("/version", getVersion)

	// user interface
	r.GET("/", displayUI)
	r.SetFuncMap(template.FuncMap{
		"availableFields":  availableFields,
		"gitCommit":        getGitCommit,
		"hostName":         getHostname,
		"releaseTimestamp": getReleaseTimestamp,
		"renderPagination": renderPagination,
		"renderQuery":      renderQuery,
	})
	r.LoadHTMLGlob("views/*")
	r.Use(static.Serve("/", static.LocalFile("assets", false)))

	// admin
	basicAuth := gin.BasicAuthForRealm(gin.Accounts{"writer": config.Hostdb.Pass}, "HostDB")
	admin := r.Group("/admin", basicAuth)
	{
		admin.GET("/showConfig", showConfig)
	}

	// API v0 routes
	v0 := r.Group("/v0")
	{
		v0.GET("/", redirectExamples)

		v0.GET("/config/", getAPIConfig)

		// csv will return a CSV file of results
		v0.GET("/csv/", outputCSV)

		// detail will return a group of records will all possible data
		v0.GET("/detail/", getDetail)
		v0.GET("/detail/:id", getDetail)

		// list will return a list of records without their payload
		v0.GET("/list/", getList)
		v0.GET("/list/:id", getList)

		// records is for record management
		v0.GET("/records/", getList)
		v0.GET("/records/:id", getDetail)
		v0.POST("/records/", basicAuth, postBulk)
		v0.PUT("/records/:id", basicAuth, saveRecord)
		v0.DELETE("/records/:id", basicAuth, deleteRecord)

		// catalog items
		v0.GET("/catalog/:item", getCatalog)
	}

	return r

}

// checks User-Agent; if the client is likely a human, return pretty json
func sendResponse(c *gin.Context, code int, obj interface{}) {

	prettyPrint := false

	ua := strings.ToLower(c.GetHeader("User-Agent"))

	prettyClients := []string{
		"chrome",
		//"curl",
		"edge",
		"gecko",
		"firefox",
		"mozilla",
	}

	if ua != "" {
		for _, client := range prettyClients {
			if strings.Contains(ua, client) {
				prettyPrint = true
				break
			}
		}
	}

	if prettyPrint {
		c.IndentedJSON(code, obj)
	} else {
		c.PureJSON(code, obj) // https://github.com/gin-gonic/gin#purejson
	}

	return

}

// ensure we have a correct hash of the (compacted) payload
func hashPayload(payload json.RawMessage) (string, error) {

	buffer := new(bytes.Buffer)
	if err := json.Compact(buffer, payload); err != nil {
		return "", err
	}
	b := buffer.Bytes()

	// hash
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil

}

// ensure all the important data points are not null
// except id; if that is empty that's useful
func ensureDataIsComplete(record *hostdb.Record) error {

	// Type
	if record.Type == "" {
		return errors.New("record has no type")
	} else if strings.Contains(record.Type, " ") {
		return errors.New("type cannot contain a space character")
	}

	// Timestamp
	if record.Timestamp == "" {
		record.Timestamp = time.Now().UTC().Format("2006-01-02 15:04:05")
	}

	if len(record.Context) == 0 {
		return errors.New("record has no context")
	}

	// hash the payload
	if record.Hash == "" {
		hash, err := hashPayload(record.Data)
		if err != nil {
			log.Println("hashing the payload failed")
			return err
		}

		record.Hash = hash
	}

	return nil
}

// Given a new (incoming) record, and an existing record, determine if something has changed
func anythingChanged(incoming hostdb.Record, existing hostdb.Record) bool {

	changed := false

	// check data hash
	if incoming.Hash != existing.Hash {
		changed = true
	}

	// compare contexts
	if !reflect.DeepEqual(incoming.Context, existing.Context) {
		changed = true
	}

	return changed

}

func debugMessage(message interface{}) {

	if config.Hostdb.Debug {
		log.Println(fmt.Sprintf("DEBUG: %v", message))
	}

}

func closer(c io.Closer) {

	err := c.Close()
	if err != nil {
		log.Fatal(err)
	}

}

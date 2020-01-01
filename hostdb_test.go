package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pdxfixit/hostdb"
	"github.com/stretchr/testify/assert"
)

var Router *gin.Engine
var TestRecordType = "test"
var TestRecordHostname = "localhost"
var TestRecordIP = "127.0.0.1"
var TestRecordTimestamp = time.Now().UTC().Format("2006-01-02 15:04:05")
var TestRecordCommitter = "Testy McTesterton"
var TestRecordContext = map[string]interface{}{"test": true}
var TestRecordData = []byte(`{"test": "yes"}`)
var TestRecordHash = "b2869f424834f25fefbdec4a2141c26f1e413bc8cd8595389326396ff9beae99"
var TestRecord = hostdb.Record{
	ID:        getUUID("test"),
	Type:      TestRecordType, // a view must exist for the given type
	Hostname:  TestRecordHostname,
	IP:        TestRecordIP,
	Timestamp: TestRecordTimestamp,
	Committer: TestRecordCommitter,
	Context:   TestRecordContext,
	Data:      TestRecordData,
	Hash:      TestRecordHash,
}

// tests expect an empty database
func TestMain(m *testing.M) {

	loadConfig()

	// use a test database
	config.Mariadb.DB = "test"

	if err := loadMariadb(); err != nil {
		log.Fatal(err)
	}

	setupTestDatabase()

	r := gin.Default()
	Router = setupRoutes(r)

	os.Exit(m.Run())

}

func TestHashPayload(t *testing.T) {

	tests := map[string]json.RawMessage{
		// this should include examples to test compaction
		// hash : json
		"099d2e284887764ae5afa796ab56c09c14173aac32c49bbc5514afc803497e8a": json.RawMessage(`{"testing":true}`),
		"6fd977db9b2afe87a9ceee48432881299a6aaf83d935fbbe83007660287f9c2e": json.RawMessage(`{ "test" : true }`),
		"b85d7aefab2ed2f8eea1531f8d21fcef4228ca217411304c83ba30c536f95f6f": json.RawMessage(`{ "test" : " true " }`),
	}

	for verifiedHash, payload := range tests {
		hash, err := hashPayload(payload)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, verifiedHash, hash)
	}

}

// this test expects a container instance on a local port
func setupTestDatabase() {

	db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%v:%v)/%v?%v", config.Mariadb.Host, config.Mariadb.Port, config.Mariadb.DB, marshalParams()))
	if err != nil {
		log.Println("creating the setupTestDatabase connection failed")
		log.Fatal(err.Error())
	}

	// truncate table
	_, err = db.Exec("TRUNCATE TABLE hostdb")
	if err != nil {
		log.Println("truncate table failed")
		log.Fatal(err.Error())
	}
	log.Println("test table cleared")

	// mock data
	var records []hostdb.Record
	for i := 0; i < 10; i++ {
		records = append(records, generateTestRecord())
	}

	valueStrings := make([]string, 0, 10)
	valueArgs := make([]interface{}, 0, 100) // 100 = 10 records * 10 fields

	for _, record := range records {
		valueStrings = append(valueStrings, "(?,?,?,?,?,?,?,?,?)")

		// convert context to string
		contextJSONBytes, err := json.Marshal(record.Context)
		if err != nil {
			log.Fatal(err.Error())
		}
		contextJSONString := string(contextJSONBytes[:])

		// convert data to string
		buffer := new(bytes.Buffer)
		if err := json.Compact(buffer, record.Data); err != nil {
			log.Fatal(err.Error())
		}
		dataJSONString := buffer.String()

		valueArgs = append(valueArgs,
			record.ID,
			record.Type,
			record.Hostname,
			record.IP,
			record.Timestamp,
			record.Committer,
			contextJSONString,
			dataJSONString,
			record.Hash,
		)
	}

	log.Println("inserting sample data")
	_, err = db.Exec(
		fmt.Sprintf("INSERT INTO hostdb (id, type, hostname, ip, timestamp, committer, context, data, hash) VALUES %s", strings.Join(valueStrings, ",")),
		valueArgs...)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("closing the setupTestDatabase connection")
	closer(db)

}

// test helper funcs galore

func createTestRecords(t *testing.T, count int) {

	// bulk entry context
	context := map[string]interface{}{
		"test": true,
	}

	// generate test records
	var records []hostdb.Record
	for i := 0; i < count; i++ {
		records = append(records, generateTestRecord())
	}

	// structure
	set := hostdb.RecordSet{
		Type:      TestRecordType,
		Timestamp: TestRecordTimestamp,
		Context:   context,
		Committer: TestRecordCommitter,
		Records:   records,
	}

	// create json bytes, reader
	bodyBytes, err := json.Marshal(set)
	if err != nil {
		t.Errorf("%v", err)
	}

	bodyReader := bytes.NewReader(bodyBytes)

	// create!
	makeTestPostRequest(t, "/v0/records/", bodyReader)
}

func decodeRecords(t *testing.T, resp *httptest.ResponseRecorder) (records map[string]hostdb.Record) {

	response := hostdb.GetRecordsResponse{}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, response.Count, len(response.Records))

	if ok := validateRecords(t, response.Records); !ok {
		t.Errorf("could not validate records")
	}

	return response.Records

}

func generateTestRecord() (record hostdb.Record) {

	data, hash := generateTestRecordData()

	record = hostdb.Record{
		ID:        getUUID("tst"),
		Type:      TestRecordType,
		Hostname:  TestRecordHostname,
		IP:        TestRecordIP,
		Timestamp: time.Unix(rand.Int63n(time.Now().Unix()-9460800)+9460800, 0).Format("2006-01-02 15:04:05"),
		Committer: TestRecordCommitter,
		Context:   TestRecordContext,
		Data:      data,
		Hash:      hash,
	}

	return record

}

func generateTestRecordData() (data json.RawMessage, hash string) {

	testDataValues := []string{
		"foo",
		"bar",
		"baz",
		"yes",
		"no",
		"up",
		"down",
		"left",
		"right",
	}

	// init the data struct
	values := make(map[string]string)

	// pick a value at random
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)
	rando := testDataValues[r.Intn(len(testDataValues))]

	values["test"] = rando

	// put it all together and hash it up
	j, _ := json.Marshal(values)
	data = []byte(j)
	hash = fmt.Sprintf("%x", sha256.Sum256(data))

	return data, hash

}

func generateTestRecords(n int) (records map[string]hostdb.Record) {
	records = map[string]hostdb.Record{}

	for i := 1; i <= n; i++ {
		r := generateTestRecord()
		records[r.ID] = r
	}

	return records
}

func makeTestGetRequest(t *testing.T, path string, auth bool, query map[string][]string) *httptest.ResponseRecorder {
	return makeTestRequest(t, "GET", path, auth, query, nil, 200)
}

func makeTestPostRequest(t *testing.T, path string, body io.Reader) *httptest.ResponseRecorder {
	return makeTestRequest(t, "POST", path, true, nil, body, 200)
}

func makeTestPutRequest(t *testing.T, path string, body io.Reader) *httptest.ResponseRecorder {
	return makeTestRequest(t, "PUT", path, true, nil, body, 201)
}

func makeTestRequest(t *testing.T, method string, path string, auth bool, query map[string][]string, body io.Reader, respCode int) *httptest.ResponseRecorder {

	// query params
	if query != nil {
		var querySlice []string
		for k, v := range query {
			value := strings.Join(v, ",")

			querySlice = append(querySlice, fmt.Sprintf("%s=%s", k, value))
		}
		queryString := strings.Join(querySlice, "&")
		path = fmt.Sprintf("%s?%s", path, queryString)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, body)

	// auth
	if auth {
		encodedCreds := base64.StdEncoding.EncodeToString([]byte("writer:" + config.Hostdb.Pass))
		req.Header.Add("Authorization", "Basic "+encodedCreds)
	}

	Router.ServeHTTP(w, req)

	assert.Equalf(t, respCode, w.Code, "%s", w.Body)

	return w

}

func validateRecord(t *testing.T, record hostdb.Record) bool {

	assert.NotEmpty(t, record, "record")

	assert.NotEmpty(t, record.Type, "type")

	// not checking most of the other fields, b/c some can be omitted in config
	// i kinda hate this, but not all records ( detail v list ) will have data
	// todo: validate more about HostdbRecord

	if record.Data != nil {
		assert.NotEmpty(t, record.Data)

		// is the hash valid
		buffer := new(bytes.Buffer)
		if err := json.Compact(buffer, record.Data); err != nil {
			return false
		}
		assert.Equalf(t, fmt.Sprintf("%x", sha256.Sum256(buffer.Bytes())), record.Hash, "%s", record.Data)
	}

	return true

}

func validateRecords(t *testing.T, records map[string]hostdb.Record) bool {

	for _, record := range records {
		if ok := validateRecord(t, record); !ok {
			return false
		}
	}

	return true

}

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pdxfixit/hostdb"
	"github.com/stretchr/testify/assert"
)

// test that the various HostDB endpoints are responding as expected

// test the endpoints.  they should be consistent.
func TestGet(t *testing.T) {

	endpoints := []string{
		"detail",
		"list",
	}

	for _, endpoint := range endpoints {

		// test without any query parameters
		records := testGet(t, endpoint, "", nil)

		// test with a single record
		for k := range records {
			testGet(t, endpoint, k, nil)
			break // just one plz
		}

		// test a bogus record (expect a 422)
		makeTestRequest(t,
			"GET",
			fmt.Sprintf("/v0/%s/foobarbaz", endpoint),
			false,
			nil,
			nil,
			http.StatusUnprocessableEntity,
		)

		// just a single param all by itself
		queryMap := map[string][]string{"type": {"test"}}
		results := testGet(t, endpoint, "", queryMap)

		assert.NotZero(t, len(results))

		// two params
		queryMap = map[string][]string{
			"type": {"test"},
			"test": {"foo", "bar", "baz", "yes", "no", "up", "down", "left", "right"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 10)

		// _search and something else -- backwards and forwards
		queryMap = map[string][]string{
			// we do this (mux a second param into the value of the first),
			// b/c golang guarantees the random order of maps
			"type": {"test&_search=Testy McTesterton"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 10)

		// reverse it
		queryMap = map[string][]string{
			"_search": {"Testy McTesterton&type=test"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 10)

		// test negative assertions (_search)
		queryMap = map[string][]string{
			// we do this (mux a second param into the value of the first),
			// b/c golang guarantees the random order of maps
			"type": {"test&!_search=Testy McTesterton"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 3)

		// reverse it
		queryMap = map[string][]string{
			"!_search": {"Testy McTesterton&type=test"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 3)

		// test negative assertions (something other than search)
		queryMap = map[string][]string{
			// we do this (mux a second param into the value of the first),
			// b/c golang guarantees the random order of maps
			"type": {"test&!ip=127.0.0.1"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 3)

		// reverse it
		queryMap = map[string][]string{
			"!ip": {"127.0.0.1&type=test"},
		}
		results = testGet(t, endpoint, "", queryMap)

		assert.Len(t, results, 3)

	}

}

// TestPost
//func TestPost(t *testing.T) {
//
//	endpoints := []string{
//		"detail",
//		"list",
//	}
//
//	for _, endpoint := range endpoints {
//
//		testPost(t, endpoint)
//
//	}
//
//}
//
//func testPost(t *testing.T, mode string) {
//
//}

// test the endpoint, and ensure the response is valid
func testGet(t *testing.T, endpoint string, path string, query map[string][]string) (records map[string]hostdb.Record) {

	resp := makeTestGetRequest(t,
		fmt.Sprintf("/v0/%s/%s", endpoint, path),
		false,
		query,
	)

	records = decodeRecords(t, resp)

	return records

}

// TestDeleteRecords
func TestDeleteRecords(t *testing.T) {

	// get an existing record
	getResp := makeTestGetRequest(t, "/v0/records/", false, nil)

	records := decodeRecords(t, getResp)

	deleteRecord := hostdb.Record{}

	// we only want one
	for id, record := range records {
		deleteRecord = record
		deleteRecord.ID = id // id isn't in the default list view...
		break
	}

	// delete that record
	makeTestRequest(t, "DELETE", fmt.Sprintf("/v0/records/%s", deleteRecord.ID), true, nil, nil, http.StatusOK)

	// validate that it's gone
	makeTestRequest(t, "GET", fmt.Sprintf("/v0/records/%s", deleteRecord.ID), false, nil, nil, http.StatusUnprocessableEntity)

}

// TestGetRecords
func TestGetRecords(t *testing.T) {

	resp := makeTestGetRequest(t,
		"/v0/records/",
		false,
		nil)

	decodeRecords(t, resp)

}

// TestHeadRecords
//func TestHeadRecords(t *testing.T) {
//
//	// get all current records
//	resp := makeTestGetRequest(t,
//		"/v0/records/",
//		false,
//		nil)
//
//	records := decodeRecords(t, records)
//
//	// pick the first one
//	var recordId string
//	for k := range *records {
//		recordId = k
//	}
//
//	// attempt to get the HEAD
//	makeTestRequest(t,
//		"HEAD",
//		fmt.Sprintf("/v0/records/%s", recordId),
//		false,
//		nil,
//		nil,
//		200,
//	)
//
//}

// TestPostRecords
func TestPostRecords(t *testing.T) {

	// bulk entry context
	context := map[string]interface{}{
		"test": true,
	}

	// generate test records
	var records []hostdb.Record
	for i := 0; i < 10; i++ {
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

	// test!
	makeTestPostRequest(t, "/v0/records/", bodyReader)

	// validate that they exist and are correct
	for _, v := range set.Records {
		resp := makeTestGetRequest(t,
			fmt.Sprintf("/v0/records/%s", v.ID),
			false,
			nil,
		)

		records := decodeRecords(t, resp)
		record := records[v.ID]

		assert.Equal(t, v.Type, record.Type, "type")
		assert.Equal(t, v.Hostname, record.Hostname, "hostname")
		assert.Equal(t, v.IP, record.IP, "ip")
	}

}

// TestPostRecordsWithNoData
func TestPostRecordsWithNoData(t *testing.T) {

	// bulk entry context
	context := map[string]interface{}{
		"test": true,
	}

	// generate records which lack data
	var records []hostdb.Record
	for i := 0; i < 10; i++ {
		records = append(records, hostdb.Record{
			Data: nil,
		})
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

	// test!
	makeTestRequest(t, "POST", "/v0/records/", true, nil, bodyReader, 400)

}

// TestPutRecords
func TestPutRecords(t *testing.T) {

	// generate a record to submit
	record := generateTestRecord()
	recordBytes, err := json.Marshal(&record)
	if err != nil {
		t.Errorf("%v", err)
	}

	// test!
	makeTestPutRequest(t,
		fmt.Sprintf("/v0/records/%s", record.ID),
		bytes.NewReader(recordBytes),
	)

	// verify it worked
	verifyResponse := makeTestGetRequest(t,
		fmt.Sprintf("/v0/records/%s", record.ID),
		false,
		nil,
	)

	verifyRecords := decodeRecords(t, verifyResponse)
	verifyRecord := verifyRecords[record.ID]

	assert.Equal(t, record.Type, verifyRecord.Type, "type")
	assert.Equal(t, record.Hostname, verifyRecord.Hostname, "hostname")
	assert.Equal(t, record.IP, verifyRecord.IP, "ip")

}

// todo: bulk test different tenants, that one bulk-import of tenant X doesn't clobber Y
// todo: ensure context, including negative assertions

// not only should all the records be saved, but any records that aren't submitted should be deleted
func TestBulkSave(t *testing.T) {

	body := `{
"type":"test",
"timestamp":"0000-00-00 00:00:00",
"committer":"tester",
"context": {
  "test": false
},
"records":[
  {
    "id":"abc123",
    "hostname":"foo.pdxfixit.com",
    "ip":"10.20.30.40",
	"context":{"test":true},
    "data":{"id":"abc123","hostname":"foo.pdxfixit.com","ip":"10.20.30.40","test":"wahoo"}
  },{
    "id":"def456",
    "hostname":"bar.pdxfixit.com",
    "ip":"10.50.60.70",
	"context":{"test":true},
    "data":{"id":"def456","hostname":"bar.pdxfixit.com","ip":"10.50.60.70","test":"yahoo"}
  },{
    "id":"ghi789",
    "hostname":"baz.pdxfixit.com",
    "ip":"1.1.1.1",
	"context":{"test":true},
    "data":{"id":"ghi789","hostname":"baz.pdxfixit.com","ip":"1.1.1.1","test":"zahoo"}
  }
]}`

	w := httptest.NewRecorder()
	response := new(hostdb.PostRecordsResponse)
	encodedCreds := base64.StdEncoding.EncodeToString([]byte("writer:" + config.Hostdb.Pass))

	// place the initial 3 records
	req, _ := http.NewRequest("POST", "/v0/records/?TestBulkSave", strings.NewReader(body))
	req.Header.Add("Authorization", "Basic "+encodedCreds)
	Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Returned an HTTP error code of %d, expected 200.", w.Code)
		t.Errorf("%s", w.Body)
	}

	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, true, response.OK, "Bulk Save OK")
	assert.Equal(t, "3 record(s) processed", response.Error, "Bulk Save ErrorResponse")

	// verify the hash values (for the data/payload) of what we just posted
	verifyData := map[string]string{
		"abc123": "48bf123ec52756cf900372ca5f70d047e0df182a9be3c1ff4e5617b531ee8956",
		"def456": "44e4ef76dc129bcac8661b7414aee04b5e8b610959a94c0fa0a2374ea50c91e7",
		"ghi789": "2e4d4c579aebee0197e102e7d3551b8e9f922d1214245bcadbea16b1bfbd3aaf",
	}

	// get the records from the database to verify they were written
	for k, v := range verifyData {
		record, err := getMariadbRow(k)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equalf(t, v, record.Hash, "verify hash for %v", k)
	}

}

// Ensure that the POST process will delete records that aren't POSTed
func TestBulkDelete(t *testing.T) {

	body := `{
"type":"test",
"timestamp":"0000-00-00 00:00:00",
"committer":"testing",
"context": {
  "test": false
},
"records":[
  {
    "id":"ghi789",
    "hostname":"bar.pdxfixit.com",
    "ip":"1.1.1.1",
	"context":{"test":true},
    "data":{"id":"ghi789","hostname":"baz.pdxfixit.com","ip":"1.1.1.1","test":"wahoo"}
  },
  {
    "hostname":"m2.local",
    "ip":"127.0.0.1",
    "context":{"test":true},
    "data":{"id":"jkl000","hostname":"m2.local","ip":"127.0.0.1","test":"magoo"}
  }
]}`

	w := httptest.NewRecorder()
	response := new(hostdb.PostRecordsResponse)
	encodedCreds := base64.StdEncoding.EncodeToString([]byte("writer:" + config.Hostdb.Pass))

	// post second round of bulk records, which should cause all but one to be deleted.
	req, _ := http.NewRequest("POST", "/v0/records/?TestBulkDelete", strings.NewReader(body))
	req.Header.Add("Authorization", "Basic "+encodedCreds)
	Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Returned an HTTP error code of %d, expected 200.", w.Code)
		t.Errorf("%s", w.Body)
	}

	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, true, response.OK, "Bulk Save OK")
	assert.Equal(t, "2 record(s) processed", response.Error, "Bulk Save ErrorResponse")

	// get the records from the database to verify that only they remain
	records, _, err := getMariadbRows(hostdb.MariadbWhereClauses{
		Relativity: "AND",
		Groups: []hostdb.MariadbWhereGrouping{
			{
				Clauses: []hostdb.MariadbWhereClause{
					{
						Relativity: "AND",
						Key:        []string{"type"},
						Operator:   "=",
						Value:      []string{"test"},
					},
				},
			},
		},
	}, hostdb.MariadbLimit{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, 2, len(records), "verify count of test records")

}

// Ensure that the POST process will take 0 records
func TestBulkPostWithNothing(t *testing.T) {

	body := `{
"type":"test",
"timestamp":"0000-00-00 00:00:00",
"committer":"testing",
"context": {
  "test": false
},
"records":[]}`

	w := httptest.NewRecorder()
	response := new(hostdb.PostRecordsResponse)
	encodedCreds := base64.StdEncoding.EncodeToString([]byte("writer:" + config.Hostdb.Pass))

	// post second round of bulk records, which should cause all but one to be deleted.
	req, _ := http.NewRequest("POST", "/v0/records/?TestBulkPostWithNothing", strings.NewReader(body))
	req.Header.Add("Authorization", "Basic "+encodedCreds)
	Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Returned an HTTP error code of %d, expected 200.", w.Code)
		t.Errorf("%s", w.Body)
	}

	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, true, response.OK, "Bulk Save OK")
	assert.Equal(t, "0 record(s) processed", response.Error, "Bulk Save ErrorResponse")

	// get the records from the database to verify that only they remain
	records, _, err := getMariadbRows(hostdb.MariadbWhereClauses{
		Relativity: "AND",
		Groups: []hostdb.MariadbWhereGrouping{
			{
				Clauses: []hostdb.MariadbWhereClause{
					{
						Relativity: "AND",
						Key:        []string{"type"},
						Operator:   "=",
						Value:      []string{"test"},
					},
				},
			},
		},
	}, hostdb.MariadbLimit{})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equalf(t, 0, len(records), "verify count of test records")

}

// test the two forms of json response
func TestPrettyJson(t *testing.T) {

	// ensure we have sample data to work with
	body := `{
"type":"test",
"timestamp":"0000-00-00 00:00:00",
"committer":"testing",
"context": {
  "test": false
},
"records":[
  {
    "id":"ghi789",
    "hostname":"baz.pdxfixit.com",
    "ip":"1.1.1.1",
	"context":{"test":true},
    "data":{"id":"ghi789","hostname":"baz.pdxfixit.com","ip":"1.1.1.1","test":"wahoo"}
  },
  {
    "hostname":"m2.local",
    "ip":"127.0.0.1",
    "context":{"test":true},
    "data":{"id":"jkl000","hostname":"m2.local","ip":"127.0.0.1","test":"magoo"}
  }
]}`

	w := httptest.NewRecorder()
	response := new(hostdb.PostRecordsResponse)
	encodedCreds := base64.StdEncoding.EncodeToString([]byte("writer:" + config.Hostdb.Pass))

	// post second round of bulk records, which should cause all but one to be deleted.
	req, _ := http.NewRequest("POST", "/v0/records/?TestBulkDelete", strings.NewReader(body))
	req.Header.Add("Authorization", "Basic "+encodedCreds)
	Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Returned an HTTP error code of %d, expected 200.", w.Code)
		t.Errorf("%s", w.Body)
	}

	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, true, response.OK, "Bulk Save OK")
	assert.Equal(t, "2 record(s) processed", response.Error, "Bulk Save ErrorResponse")

	// test compact json
	resp := makeTestGetRequest(t, "/v0/records/ghi789", false, nil)
	body = resp.Body.String()

	assert.NotContains(t, body, "  ", "compact json")

	// test pretty json
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/v0/records/ghi789", nil)
	req.Header["User-Agent"] = []string{"edge"}
	Router.ServeHTTP(w, req)
	body = w.Body.String()

	assert.Contains(t, body, "  ", "pretty json")

}

// test the public api configuration endpoint
func TestApiConfig(t *testing.T) {

	response := makeTestGetRequest(t, "/v0/config/", false, nil)

	var testConfig hostdb.GlobalConfig

	if err := json.NewDecoder(response.Body).Decode(&testConfig.API.V0); err != nil {
		t.Errorf("%v", err)
	}

	assert.GreaterOrEqual(t, len(testConfig.API.V0.ContextFields), 2, "number of record types for which we have required context fields")
	assert.GreaterOrEqual(t, len(testConfig.API.V0.ListFields), 3, "default fields to display when querying the list endpoint")
	assert.GreaterOrEqual(t, len(testConfig.API.V0.QueryParams), 20, "number of supported query parameters")

}

// test the catalog function
func TestCatalog(t *testing.T) {

	urls := []string{
		"/v0/catalog/test",
		"/v0/catalog/test?count=0",
		"/v0/catalog/test?count=false",
		"/v0/catalog/test?filter=/./",
		"/v0/catalog/test?count=0&filter=/./",
		"/v0/catalog/test?count=false&filter=/./",
	}

	for _, url := range urls {
		response := makeTestGetRequest(t, url, false, nil)

		object := hostdb.GetCatalogResponse{}

		if err := json.NewDecoder(response.Body).Decode(&object); err != nil {
			t.Errorf("%v", err)
		}

		assert.Equal(t, object.Count, len(object.Catalog))

		assert.GreaterOrEqual(t, len(object.Catalog), 2, "unexpected number of items in the catalog")
	}

}

// test the catalog filter function
func TestCatalogFilter(t *testing.T) {

	response := makeTestGetRequest(t, "/v0/catalog/test?filter=/goo$/", false, nil)

	object := hostdb.GetCatalogResponse{}

	if err := json.NewDecoder(response.Body).Decode(&object); err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, object.Count, len(object.Catalog))

	assert.Equal(t, len(object.Catalog), 1, "unexpected number of items in the filtered catalog")

}

// test the catalog count function
func TestCatalogQuantities(t *testing.T) {

	response := makeTestGetRequest(t, "/v0/catalog/test?count", false, nil)

	object := hostdb.GetCatalogQuantityResponse{}

	if err := json.NewDecoder(response.Body).Decode(&object); err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, object.Count, len(object.Catalog))

	assert.GreaterOrEqual(t, len(object.Catalog), 2, "unexpected number of items in the catalog")

}

// test that the catalog endpoint, with no item, returns a bad request
func TestBadCatalog(t *testing.T) {

	makeTestRequest(t,
		"GET",
		"/v0/catalog/foo",
		false,
		nil,
		nil,
		http.StatusUnprocessableEntity,
	)

}

// test the catalog endpoint, with a filter matching 0 records
func TestBadCatalogFilter(t *testing.T) {

	response := makeTestRequest(t,
		"GET",
		"/v0/catalog/type",
		false,
		map[string][]string{
			"filter": {
				"/foo/",
			},
		},
		nil,
		http.StatusNotFound,
	)

	object := hostdb.GetCatalogResponse{}

	if err := json.NewDecoder(response.Body).Decode(&object); err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, len(object.Catalog), 0, "should be an empty catalog")

}

// try POSTing some examples of data that has gotten mixed up before
// and ensure it's being handled appropriately
func TestUnusualData(t *testing.T) {

	// ensure we have sample data to work with
	body := `{
"type":"test",
"timestamp":"0000-00-00 00:00:00",
"committer":"testing",
"context": {
  "test": false
},
"records":[
  {
    "id":"xyz123",
	"type":"test-regular",
    "hostname":"pdxfixit.com",
    "ip":"127.0.0.1",
	"context":{"test":true},
    "data":{"test":"stuff"}
  },
  {
    "id":"zzz999",
	"type":"test-urlencode",
    "hostname":"spongiforms-are-tasty.com",
    "ip":"169.254.169.254",
	"context":{"test":true},
    "data":{"test":"<a href />"}
  }
]}`

	w := httptest.NewRecorder()
	response := new(hostdb.PostRecordsResponse)
	encodedCreds := base64.StdEncoding.EncodeToString([]byte("writer:" + config.Hostdb.Pass))

	// post second round of bulk records, which should cause all but one to be deleted.
	req, _ := http.NewRequest("POST", "/v0/records/?TestUnusualData", strings.NewReader(body))
	req.Header.Add("Authorization", "Basic "+encodedCreds)
	Router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Returned an HTTP error code of %d, expected 200.", w.Code)
		t.Errorf("%s", w.Body)
	}

	err := json.NewDecoder(w.Body).Decode(&response)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.Equal(t, true, response.OK, "Bulk Save OK")
	assert.Equal(t, "2 record(s) processed", response.Error, "Bulk Save ErrorResponse")

	// verify the values (for the data/payload) of what we just posted
	verifyData := map[string]string{
		"xyz123": `{"test":"stuff"}`,
		"zzz999": `{"test":"<a href />"}`,
	}

	// get the records from the database to verify they were written
	for k, v := range verifyData {

		verifyResponse := makeTestGetRequest(t,
			fmt.Sprintf("/v0/records/%s", k),
			false,
			nil,
		)

		verifyRecords := decodeRecords(t, verifyResponse)
		verifyRecord := verifyRecords[k]

		buffer := new(bytes.Buffer)
		if err := json.Compact(buffer, verifyRecord.Data); err != nil {
			t.Fatal(err)
		}

		assert.Equalf(t, k, verifyRecord.ID, "verify record ID")
		assert.Contains(t, verifyRecord.Type, "test", "verify record type")
		assert.NotEmpty(t, verifyRecord.Hostname, "verify record hostname")
		assert.NotEmpty(t, verifyRecord.IP, "verify record IP")
		assert.NotEmpty(t, verifyRecord.Timestamp, "verify record timestamp")
		assert.Equalf(t, "testing", verifyRecord.Committer, "verify record committer")
		assert.Len(t, verifyRecord.Context, 1, "verify record context length")
		assert.Equalf(t, true, verifyRecord.Context["test"], "verify record context value")
		assert.Equalf(t, v, buffer.String(), "verify record data")
		assert.NotEmpty(t, verifyRecord.Hash, "verify record hash")
	}

}

func TestOutputCSV(t *testing.T) {

	w := makeTestGetRequest(t,
		"/v0/csv/",
		false,
		map[string][]string{
			"test": {"foo"},
		},
	)

	assert.Regexp(t, "^ID,Type,Hostname,IP Address,Last Updated", w.Body, "CSV header missing")

	for key, values := range w.Header() {
		switch key {
		case "Content-Description":
			assert.Equal(t, "File Transfer", values[0], "CSV Content-Description")
		case "Content-Disposition":
			assert.Equal(t, "attachment; filename=metadata-"+time.Now().Format("20060102150405")+".csv", values[0], "CSV Content-Disposition")
		case "Content-Type":
			assert.Equal(t, "text/csv", values[0], "CSV Content-Type")
		}
	}

}

func TestOutputCSVError(t *testing.T) {

	w := makeTestRequest(t,
		"GET",
		"/v0/csv/",
		false,
		nil,
		nil,
		400,
	)

	assert.Regexp(t, "no search terms provided</pre>", w.Body, "missing error message")

}

// TestFields
func TestFields(t *testing.T) {

	// make request w/ _fields arg
	resp := makeTestGetRequest(t, "/v0/list/", false, map[string][]string{
		"_fields": {
			"id",
			"timestamp",
			"hash",
		},
	})

	// decode response
	response := hostdb.GetRecordsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("%v", err)
	}

	// check values
	for _, r := range response.Records {
		assert.NotEmpty(t, r.ID, "ID should have a value")
		assert.Empty(t, r.Type, "Type should be empty")
		assert.Empty(t, r.Hostname, "Hostname should be empty")
		assert.Empty(t, r.IP, "IP should be empty")
		assert.NotEmpty(t, r.Timestamp, "Timestamp should have a value")
		assert.Empty(t, r.Committer, "Committer should be empty")
		assert.Empty(t, r.Context, "Context should be empty")
		assert.Empty(t, r.Data, "Data should be empty")
		assert.NotEmpty(t, r.Hash, "Hash should have a value")
	}

}

// test the _limit argument for limiting the number of records returned
func TestLimit(t *testing.T) {

	createTestRecords(t, 20)

	limit := 7

	// make request w/ _limit arg
	resp := makeTestGetRequest(t, "/v0/list/", false, map[string][]string{
		"_limit": {
			fmt.Sprintf("%d", limit),
		},
	})

	// decode response
	response := hostdb.GetRecordsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("%v", err)
	}

	// check that we got the expected number of records
	assert.Equal(t, limit, len(response.Records), "number of records in result")

	// and that the count is still the total number of records returned by the query
	assert.Equal(t, 22, response.Count, "result count")

	// negative value should result in a 400
	makeTestRequest(t,
		"GET",
		"/v0/list/",
		false,
		map[string][]string{
			"_limit": {
				fmt.Sprintf("-%d", limit),
			},
		},
		nil,
		400,
	)

}

// test the _offset argument, for specifying a certain number of records to skip in the result set
func TestOffset(t *testing.T) {

	limit := 3
	offset := 6

	// get all of the record IDs so we know the index
	allRecords := testGet(t, "detail", "", nil)
	var sortedIDs []string
	for id := range allRecords {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	// make req w/ _offset arg also
	resp := makeTestGetRequest(t, "/v0/detail/", false, map[string][]string{
		"_limit": {
			fmt.Sprintf("%d", limit),
		},
		"_offset": {
			fmt.Sprintf("%d", offset),
		},
	})

	// decode response
	response := hostdb.GetRecordsResponse{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Errorf("%v", err)
	}

	// sort the response records
	var sortedResponseIDs []string
	for id := range response.Records {
		sortedResponseIDs = append(sortedResponseIDs, id)
	}
	sort.Strings(sortedResponseIDs)

	i := offset
	for _, id := range sortedResponseIDs {
		assert.Equal(t, sortedIDs[i], id)
		i++
	}

	// negative value should result in a 400
	makeTestRequest(t,
		"GET",
		"/v0/list/",
		false,
		map[string][]string{
			"_limit": {
				fmt.Sprintf("-%d", limit),
			},
			"_offset": {
				fmt.Sprintf("-%d", offset),
			},
		},
		nil,
		400,
	)

}

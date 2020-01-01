package main

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/pdxfixit/hostdb"
	"github.com/stretchr/testify/assert"
)

func TestCheckMariadb(t *testing.T) {

	if !checkMariadb() {
		t.Error("database is not present")
	}

}

func TestCheckTable(t *testing.T) {

	if !checkTable() {
		t.Error("table is not present")
	}

}

func TestDeleteMariaRow(t *testing.T) {

	where := hostdb.MariadbWhereClauses{
		Groups: []hostdb.MariadbWhereGrouping{
			{
				Clauses: []hostdb.MariadbWhereClause{
					{
						Relativity: "",
						Key:        []string{"type"},
						Operator:   "=",
						Value:      []string{"test"},
					},
				},
			},
		},
	}

	recordIds, err := getRowIds(where)
	if err != nil {
		t.Errorf("%v", err)
	}

	err = deleteMariadbRow(recordIds[0])
	if err != nil {
		t.Errorf("%v", err)
	}

	record, err := getMariadbRow(recordIds[0])
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.Empty(t, record, "should get 0 records")

}

func TestGetRowIds(t *testing.T) {

	where := hostdb.MariadbWhereClauses{
		Groups: []hostdb.MariadbWhereGrouping{
			{
				Clauses: []hostdb.MariadbWhereClause{
					{
						Relativity: "",
						Key:        []string{"type"},
						Operator:   "=",
						Value:      []string{"test"},
					},
				},
			},
		},
	}

	recordIds, err := getRowIds(where)
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.NotEmpty(t, recordIds, "more than 0 recordIds")

}

func TestGetMariadbRow(t *testing.T) {

	record, err := getMariadbRow(TestRecord.ID)
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.IsType(t, new(hostdb.Record), &record, "hostdb record type check")

	if reflect.DeepEqual(record, new(hostdb.Record)) {
		t.Error("Empty record returned.")
	}

}

func TestGetMariadbRows(t *testing.T) {

	where := hostdb.MariadbWhereClauses{
		Groups: []hostdb.MariadbWhereGrouping{
			{
				Clauses: []hostdb.MariadbWhereClause{
					{
						Relativity: "",
						Key:        []string{"type"},
						Operator:   "=",
						Value:      []string{"test"},
					},
				},
			},
		},
	}

	records, _, err := getMariadbRows(where, hostdb.MariadbLimit{})
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.NotEmpty(t, records, "records ! > 0")

}

func TestGetMariadbVersion(t *testing.T) {

	version, err := getMariadbVersion()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.NotEmpty(t, version)

}

func TestGetTotalRecords(t *testing.T) {

	count, err := getTotalRecords()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.NotZero(t, count)

}

func TestGetNewestTimestamp(t *testing.T) {

	timeString, err := getNewestTimestamp()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.NotEmpty(t, timeString)

	timeObject, err := time.Parse("2006-01-02 15:04:05", timeString)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.NotZero(t, timeObject)

}

func TestGetOldestTimestamp(t *testing.T) {

	timeString, err := getOldestTimestamp()
	if err != nil {
		t.Errorf("%v", err)
	}

	assert.NotEmpty(t, timeString)

	timeObject, err := time.Parse("2006-01-02 15:04:05", timeString)
	if err != nil {
		t.Errorf(err.Error())
	}

	assert.NotZero(t, timeObject)

}

func TestGetRecentCommitterTimestamps(t *testing.T) {

	lastSeen, err := getRecentCommitterTimestamps()
	if err != nil {
		t.Errorf(err.Error())
	}

	for committer, timestamp := range lastSeen {

		assert.NotEmpty(t, committer)

		timeObject, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err != nil {
			t.Errorf(err.Error())
		}

		assert.NotEmpty(t, timestamp)
		assert.NotZero(t, timeObject)

	}

}

// loadMariadb is exercised in TestMain

func TestMarshalParams(t *testing.T) {

	config.Mariadb.Params = []string{
		"test=true",
		"foobar=yes",
	}

	params := marshalParams()

	assert.Equal(t, "test=true&foobar=yes", params)

}

func TestSaveMariadbRow(t *testing.T) {

	if err := saveMariadbRow(TestRecord); err != nil {
		t.Errorf("%v", err)
	}

}

func TestSaveMariadbRows(t *testing.T) {

	records := []hostdb.Record{
		{
			ID:        "abc123",
			Type:      "test",
			Hostname:  "foo.pdxfixit.com",
			IP:        "10.20.30.40",
			Timestamp: time.Unix(rand.Int63n(time.Now().Unix()-9460800)+9460800, 0).Format("2006-01-02 15:04:05"),
			Committer: "tester",
			Context:   map[string]interface{}{"test": true},
			Data:      json.RawMessage(`{"id":"abc123","hostname":"foo.pdxfixit.com","ip":"10.20.30.40","test":"wahoo"}`),
			Hash:      "48bf123ec52756cf900372ca5f70d047e0df182a9be3c1ff4e5617b531ee8956",
		}, {
			ID:        "def456",
			Type:      "test",
			Hostname:  "bar.pdxfixit.com",
			IP:        "10.50.60.70",
			Timestamp: time.Unix(rand.Int63n(time.Now().Unix()-9460800)+9460800, 0).Format("2006-01-02 15:04:05"),
			Committer: "tester",
			Context:   map[string]interface{}{"test": true},
			Data:      json.RawMessage(`{"id":"def456","hostname":"bar.pdxfixit.com","ip":"10.50.60.70","test":"yahoo"}`),
			Hash:      "44e4ef76dc129bcac8661b7414aee04b5e8b610959a94c0fa0a2374ea50c91e7",
		}, {
			ID:        "ghi789",
			Type:      "test",
			Hostname:  "baz.pdxfixit.com",
			IP:        "1.1.1.1",
			Timestamp: time.Unix(rand.Int63n(time.Now().Unix()-9460800)+9460800, 0).Format("2006-01-02 15:04:05"),
			Committer: "tester",
			Context:   map[string]interface{}{"test": true},
			Data:      json.RawMessage(`{"id":"ghi789","hostname":"baz.pdxfixit.com","ip":"1.1.1.1","test":"zahoo"}`),
			Hash:      "2e4d4c579aebee0197e102e7d3551b8e9f922d1214245bcadbea16b1bfbd3aaf",
		},
	}

	if err := saveMariadbRows(records); err != nil {
		t.Errorf("%v", err)
	}

	for _, testRecord := range records {
		verifyRecord, err := getMariadbRow(testRecord.ID)
		if err != nil {
			t.Errorf("%v", err)
		}

		assert.Equal(t, testRecord.Type, verifyRecord.Type, "type")
		assert.Equal(t, testRecord.Hostname, verifyRecord.Hostname, "hostname")
		assert.Equal(t, testRecord.IP, verifyRecord.IP, "ip")
		assert.Equal(t, testRecord.Timestamp, verifyRecord.Timestamp, "timestamp")
		assert.Equal(t, testRecord.Committer, verifyRecord.Committer, "committer")
		assert.Equal(t, testRecord.Hash, verifyRecord.Hash, "hash")
	}

}

// setupDatabase is exercised in loadMariadb

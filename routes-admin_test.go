package main

import (
	"encoding/json"
	"testing"

	"github.com/pdxfixit/hostdb"
	"github.com/stretchr/testify/assert"
)

// GET /admin/showConfig
func TestShowConfig(t *testing.T) {
	w := makeTestGetRequest(t, "/admin/showConfig", true, nil)

	var testConfig hostdb.GlobalConfig

	if err := json.NewDecoder(w.Body).Decode(&testConfig); err != nil {
		t.Errorf("%v", err)
	}

	assert.Equal(t, "0.0.0.0", testConfig.Hostdb.Host, "Hostdb host")
	assert.Equal(t, 8080, testConfig.Hostdb.Port, "Hostdb port")
	assert.Equal(t, "*******ord", testConfig.Hostdb.Pass, "Hostdb writer password")
	assert.Equal(t, 3306, testConfig.Mariadb.Port, "Mariadb port")
}

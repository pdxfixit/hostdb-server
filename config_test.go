package main

import (
	"os"
	"testing"

	"github.com/pdxfixit/hostdb"
	"github.com/stretchr/testify/assert"
)

// This will examine the config global, and ensure the values match config.yaml
func TestLoadConfig(t *testing.T) {
	assert.IsType(t, hostdb.GlobalConfig{}, config, "type check")

	assert.NotEmpty(t, config, "config should not be empty")

	assert.Equal(t, "0.0.0.0", config.Hostdb.Host, "Configuration - Hostdb.Host")
	assert.Equal(t, 8080, config.Hostdb.Port, "Configuration - Hostdb.Port")
	assert.Equal(t, "badpassword", config.Hostdb.Pass, "Configuration - Hostdb.Pass")
	assert.Equal(t, "https://hostdb.pdxfixit.com", config.Hostdb.URL, "Configuration - Hostdb.URL")

	assert.Equal(t, 3306, config.Mariadb.Port, "Configuration - Mariadb.Port")
	assert.Equal(t, "test", config.Mariadb.DB, "Configuration - Mariadb.DB")
	assert.Equal(t, "app", config.Mariadb.User, "Configuration - Mariadb.User")
	assert.Equal(t, "badpassword", config.Mariadb.Pass, "Configuration - Mariadb.Pass")
	assert.GreaterOrEqual(t, len(config.Mariadb.Params), 1, "Configuration - Mariadb.Params")

	// test the ability to override k8s specific stuff
	if err := os.Setenv("HOSTDB_HOSTDB_SERVER_SERVICE_PORT", "1234"); err != nil {
		t.Errorf("%v", err)
	}
	if err := os.Setenv("MARIADB_SERVICE_PORT", "1234"); err != nil {
		t.Errorf("%v", err)
	}

	loadConfig() // re-parse config to invoke overrides

	assert.Equal(t, 1234, config.Hostdb.Port, "Configuration Overrides - Hostdb.Port")
	assert.Equal(t, 1234, config.Mariadb.Port, "Configuration Overrides - Mariadb.Port")

	if err := os.Setenv("HOSTDB_HOSTDB_SERVER_SERVICE_PORT", "8080"); err != nil {
		t.Errorf("%v", err)
	}
	if err := os.Setenv("MARIADB_SERVICE_PORT", "3306"); err != nil {
		t.Errorf("%v", err)
	}

	// I tried os.Unsetenv, but that didn't result in the desired effect.
	// TODO: Find out why, and avoid setting the overrides back to expected values
	loadConfig() // re-parse config to remove overrides

	assert.Equal(t, 8080, config.Hostdb.Port, "Configuration Overrides - Hostdb.Port (restored)")
	assert.Equal(t, 3306, config.Mariadb.Port, "Configuration Overrides - Mariadb.Port (restored)")
}

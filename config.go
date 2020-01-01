package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

func loadConfig() {

	// load the config
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/hostdb")
	viper.AddConfigPath(".")

	// load env vars
	viper.SetEnvPrefix("hostdb")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		log.Fatal(fmt.Errorf("fatal error config file: %s", err))
	}

	// allow specific environment variables to override config
	if value, ok := os.LookupEnv("HOSTDB_HOSTDB_SERVER_SERVICE_PORT"); ok {
		log.Println("Overriding Hostdb port with: " + value)
		viper.Set("hostdb.port", value)
	}
	if value, ok := os.LookupEnv("MARIADB_SERVICE_HOST"); ok {
		log.Println("Overriding Mariadb host with: " + value)
		viper.Set("mariadb.host", value)
	}
	if value, ok := os.LookupEnv("MARIADB_SERVICE_PORT"); ok {
		log.Println("Overriding Mariadb port with: " + value)
		viper.Set("mariadb.port", value)
	}

	// unmarshal into our structs
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatal(fmt.Errorf("unable to decode into struct, %v", err))
	}

	if config.Hostdb.Debug {
		// log the env vars
		envvars := os.Environ()
		sort.Slice(envvars, func(i, j int) bool { return envvars[i] < envvars[j] })
		for _, pair := range envvars {
			debugMessage(pair)
		}

		// log the current config
		debugMessage(config)
	}

}

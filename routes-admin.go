package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// show the current (redacted) hostdb configuration
func showConfig(c *gin.Context) {

	displayConfig := config

	displayConfig.Hostdb.Pass = "*******" + config.Hostdb.Pass[len(config.Hostdb.Pass)-3:]
	displayConfig.Mariadb.Pass = "*******" + config.Mariadb.Pass[len(config.Mariadb.Pass)-3:]

	sendResponse(c, http.StatusOK, displayConfig)

}

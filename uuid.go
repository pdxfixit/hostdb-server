package main

import (
	"fmt"

	"github.com/satori/go.uuid"
)

func getUUID(prefix string) string {
	if prefix == "" {
		prefix = "hdb"
	}

	id := uuid.NewV4()
	return fmt.Sprintf("%s-%s", prefix, id.String())
}

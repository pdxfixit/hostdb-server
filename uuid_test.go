package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUuid(t *testing.T) {
	uuid := getUUID("test")

	assert.NotEmpty(t, uuid, "ensure uuid not empty")
	assert.Regexp(t, "^test-.*", uuid, "uuid must have 'test-' prefix")
}

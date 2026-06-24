package main

import (
	"testing"

	"agent-go/pkg/loader"
)

func init() {
	loader.InitSchemas()
	loader.InitPromptLoader()
	loader.InitPrompts()
}

func TestInit(t *testing.T) {
	if loader.PRODUCT_MANAGER_SCHEMA == nil {
		t.Fatal("PRODUCT_MANAGER_SCHEMA not initialized")
	}
	if loader.PRODUCT_MANAGER_PROMPT == "" {
		t.Fatal("PRODUCT_MANAGER_PROMPT not initialized")
	}
}

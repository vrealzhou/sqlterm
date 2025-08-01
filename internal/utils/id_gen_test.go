package utils

import (
	"testing"
)

func TestNewIDGen(t *testing.T) {
	gen, err := NewIDGen()
	if err != nil {
		t.Fatalf("Failed to create IDGen: %v", err)
	}
	if gen == nil {
		t.Fatal("NewIDGen returned nil")
	}
	if gen.node == nil {
		t.Error("IDGen.node is nil")
	}
}

func TestIDGen_Generate(t *testing.T) {
	gen, err := NewIDGen()
	if err != nil {
		t.Fatalf("Failed to create IDGen: %v", err)
	}

	id1 := gen.Generate()
	id2 := gen.Generate()

	if id1 == 0 {
		t.Error("Generate returned zero ID")
	}
	if id2 == 0 {
		t.Error("Generate returned zero ID")
	}
	if id1 == id2 {
		t.Error("Generate returned duplicate IDs")
	}
}

func TestIDGen_GenerateString(t *testing.T) {
	gen, err := NewIDGen()
	if err != nil {
		t.Fatalf("Failed to create IDGen: %v", err)
	}

	id1 := gen.GenerateString()
	id2 := gen.GenerateString()

	if id1 == "" {
		t.Error("GenerateString returned empty string")
	}
	if id2 == "" {
		t.Error("GenerateString returned empty string")
	}
	if id1 == id2 {
		t.Error("GenerateString returned duplicate IDs")
	}
}
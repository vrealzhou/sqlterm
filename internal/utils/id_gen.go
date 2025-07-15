package utils

import (
	"fmt"

	"github.com/bwmarrin/snowflake"
)

type IDGen struct {
	node *snowflake.Node
}

func NewIDGen() (*IDGen, error) {
	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, fmt.Errorf("failed on create IDGen: %w", err)
	}
	return &IDGen{
		node: node,
	}, nil
}

func (g *IDGen) Generate() int64 {
	return g.node.Generate().Int64()
}

func (g *IDGen) GenerateString() string {
	return g.node.Generate().Base58()
}

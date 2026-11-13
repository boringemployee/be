package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestWrite(t *testing.T) {
	var buf bytes.Buffer
	if err := write(&buf); err != nil {
		t.Fatal(err)
	}
	var got catalog
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Agents) < 5 || got.Agents[0].Name != "claude" || got.Agents[0].Title != "Clock Code" || got.Agents[0].Accent == "" {
		t.Errorf("agents = %+v", got.Agents)
	}
	if len(got.Topics) < 5 || got.Topics[0].Description == "" {
		t.Errorf("topics = %+v", got.Topics)
	}
}

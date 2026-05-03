package consolelog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHubKeepsLineLimit(t *testing.T) {
	hub := NewHub(3)

	hub.Add("line1")
	hub.Add("line2")
	hub.Add("line3")
	hub.Add("line4")

	assert.Equal(t, []string{"line2", "line3", "line4"}, hub.Snapshot())
}

func TestHubSubscribeReceivesNewLines(t *testing.T) {
	hub := NewHub(10)
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	hub.Add("live line")

	select {
	case got := <-ch:
		assert.Equal(t, "live line", got)
	default:
		t.Fatal("expected subscribed log line")
	}
}

func TestHubNormalizesLines(t *testing.T) {
	hub := NewHub(10)

	hub.Add(" \x1b[31mERRO[2026] failed\x1b[0m ")
	hub.Add("")

	assert.Equal(t, []string{"ERRO[2026] failed"}, hub.Snapshot())
}

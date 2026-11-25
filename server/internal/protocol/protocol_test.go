package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	data, err := Encode("ping", map[string]string{"client": "test"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	env, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if env.T != "ping" {
		t.Fatalf("tipo = %q, quería %q", env.T, "ping")
	}

	var payload map[string]string
	if err := json.Unmarshal(env.D, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if payload["client"] != "test" {
		t.Fatalf("payload.client = %q, quería %q", payload["client"], "test")
	}
}

func TestEncodeNilPayloadOmitsD(t *testing.T) {
	data, err := Encode("heartbeat", nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if strings.Contains(string(data), `"d"`) {
		t.Fatalf("el payload nil no debe serializar %q: %s", "d", data)
	}
}

func TestEncodeRejectsEmptyType(t *testing.T) {
	if _, err := Encode("", nil); err == nil {
		t.Fatal("esperaba error con tipo vacío")
	}
}

func TestDecodeErrors(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"mensaje vacío", nil},
		{"json inválido", []byte("{")},
		{"sin tipo", []byte(`{"d":{}}`)},
		{"demasiado grande", bytes.Repeat([]byte("a"), MaxMessageSize+1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(tc.data); err == nil {
				t.Fatal("esperaba error, no ocurrió ninguno")
			}
		})
	}
}

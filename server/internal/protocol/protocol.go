// Package protocol define el sobre de mensajes compartido por cliente y
// servidor. Ver docs/04-red.md para el catálogo completo de mensajes.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MaxMessageSize es el tamaño máximo aceptado de un mensaje (32 KB).
// Los mensajes más grandes se rechazan en la capa de red.
const MaxMessageSize = 32 * 1024

// Envelope es el sobre común: T identifica el tipo de mensaje y D el payload.
type Envelope struct {
	T string          `json:"t"`
	D json.RawMessage `json:"d,omitempty"`
}

// Encode serializa un mensaje del tipo t con el payload d.
// Si d es nil, el campo "d" se omite.
func Encode(t string, d any) ([]byte, error) {
	if t == "" {
		return nil, errors.New("protocol: el tipo de mensaje no puede estar vacío")
	}

	var raw json.RawMessage
	if d != nil {
		b, err := json.Marshal(d)
		if err != nil {
			return nil, fmt.Errorf("protocol: payload inválido: %w", err)
		}
		raw = b
	}

	data, err := json.Marshal(Envelope{T: t, D: raw})
	if err != nil {
		return nil, fmt.Errorf("protocol: no se pudo serializar: %w", err)
	}
	if len(data) > MaxMessageSize {
		return nil, fmt.Errorf("protocol: mensaje de %d bytes excede el máximo de %d", len(data), MaxMessageSize)
	}
	return data, nil
}

// Decode valida el tamaño y deserializa un mensaje entrante.
func Decode(data []byte) (Envelope, error) {
	if len(data) == 0 {
		return Envelope{}, errors.New("protocol: mensaje vacío")
	}
	if len(data) > MaxMessageSize {
		return Envelope{}, fmt.Errorf("protocol: mensaje de %d bytes excede el máximo de %d", len(data), MaxMessageSize)
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, fmt.Errorf("protocol: JSON inválido: %w", err)
	}
	if env.T == "" {
		return Envelope{}, errors.New(`protocol: falta el campo "t"`)
	}
	return env, nil
}

package outbox

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

type decoderFunc func(payload json.RawMessage) (interface{}, error)

type registryKey struct {
	eventType enums.OutboxEventType
	version   int
}

type DecoderRegistry struct {
	mtx      sync.RWMutex
	registry map[registryKey]decoderFunc
}

func NewDecoderRegistry() *DecoderRegistry {
	return &DecoderRegistry{registry: make(map[registryKey]decoderFunc)}
}

func (r *DecoderRegistry) Register(eventType enums.OutboxEventType, version int, decoder decoderFunc) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.registry[registryKey{eventType: eventType, version: version}] = decoder
}

func (r *DecoderRegistry) Decode(eventType enums.OutboxEventType, version int, payload json.RawMessage) (interface{}, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if decoder, ok := r.registry[registryKey{eventType: eventType, version: version}]; ok {
		return decoder(payload)
	}
	return nil, fmt.Errorf("decoder not registered for %s@v%d", eventType, version)
}

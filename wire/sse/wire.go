package ssewire

import (
	sse "webookApp/internal/sse"

	"github.com/google/wire"
)

var SseSet = wire.NewSet(sse.NewSSEManagerHandler)

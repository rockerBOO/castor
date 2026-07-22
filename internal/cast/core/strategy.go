package core

import (
	"context"

	"github.com/stupside/castor/internal/media"
)

// Strategy executes a full cast for one device family: given the resolved source
// and the local stream-server IP, it drives discovery/connect timing, planning,
// transcoding, and delivery however that family requires. The top-level package
// selects one by device type and calls Cast; it never knows spool from
// passthrough. Each device package provides exactly one implementation.
type Strategy interface {
	Cast(ctx context.Context, resolved *media.Stream, localIP string) error
}

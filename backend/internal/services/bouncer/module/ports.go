package module

import dom "swearjar/internal/services/bouncer/domain"

// Ports holds the ports exposed by the bouncer module
type Ports struct {
	Worker   dom.WorkerPort
	Enqueuer dom.EnqueuePort
}

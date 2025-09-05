package module

import "swearjar/internal/services/hallmonitor/domain"

// Ports defines hallmonitor module ports exposed via the registry
type Ports struct {
	Worker    domain.WorkerPort
	Seeder    domain.SeederPort
	Refresher domain.RefresherPort
	Signals   domain.SignalsPort
	Reader    domain.ReaderPort
}

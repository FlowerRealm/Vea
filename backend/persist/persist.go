package persist

import (
	"vea/backend/domain"
)

func Save(path string, state domain.ServiceState) error {
	return SaveV2(path, state)
}

func Load(path string) (domain.ServiceState, error) {
	return LoadV2(path)
}

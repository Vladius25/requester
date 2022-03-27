package api

import "context"

// GetHealthStatus returns OK if the service is healthy.
func (h *handler) GetHealthStatus(context.Context) error {
	return nil
}

package handler

import "context"

type workerStub struct {
	data map[string]any
	err  error
}

func (s *workerStub) PostJSON(ctx context.Context, path string, payload any, requestID string) (map[string]any, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.data, nil
}

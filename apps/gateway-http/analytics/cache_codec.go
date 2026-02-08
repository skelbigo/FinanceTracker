package analytics

import (
	"encoding/json"
	"errors"
)

func MarshalAnalyticsResponse(resp AnalyticsResponse) ([]byte, error) {
	return json.Marshal(resp)
}

func UnmarshalAnalyticsResponse(b []byte) (AnalyticsResponse, error) {
	if len(b) == 0 {
		return AnalyticsResponse{}, errors.New("empty cache payload")
	}
	var resp AnalyticsResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return AnalyticsResponse{}, err
	}
	return resp, nil
}

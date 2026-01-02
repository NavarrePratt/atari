package daemon

// Request represents a JSON-RPC request from a client.
type Request struct {
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
	ID     int    `json:"id,omitempty"`
}

// Response represents a JSON-RPC response to a client.
type Response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	ID     int    `json:"id,omitempty"`
}

// StatusResponse contains daemon status information.
type StatusResponse struct {
	Status      string      `json:"status"`
	CurrentBead string      `json:"current_bead,omitempty"`
	Uptime      string      `json:"uptime"`
	StartTime   string      `json:"start_time"`
	Stats       StatusStats `json:"stats"`
}

// StatusStats contains queue statistics for the status response.
type StatusStats struct {
	Iteration int `json:"iteration"`
	TotalSeen int `json:"total_seen"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Abandoned int `json:"abandoned"`
	InBackoff int `json:"in_backoff"`
}

// StopParams contains parameters for the stop method.
type StopParams struct {
	Force bool `json:"force,omitempty"`
}

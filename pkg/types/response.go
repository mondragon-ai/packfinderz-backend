package types

type SuccessEnvelope struct {
	Data any `json:"data"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type ErrorEnvelope struct {
	Error APIError `json:"error"`
}

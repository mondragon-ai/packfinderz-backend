package enums

type OutboxDLQErrorReason string

const (
	OutboxDLQReasonMaxAttempts  OutboxDLQErrorReason = "max_attempts"
	OutboxDLQReasonNonRetryable OutboxDLQErrorReason = "non_retryable"
)

var validOutboxDLQErrorReasons = []OutboxDLQErrorReason{
	OutboxDLQReasonMaxAttempts,
	OutboxDLQReasonNonRetryable,
}

func (r OutboxDLQErrorReason) IsValid() bool {
	for _, candidate := range validOutboxDLQErrorReasons {
		if candidate == r {
			return true
		}
	}
	return false
}

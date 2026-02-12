package subscriptions

import "context"

type stubSquareSubscriptionClient struct {
	createResp               *SquareSubscription
	cancelResp               *SquareSubscription
	getResp                  *SquareSubscription
	pauseResp                *SquareSubscription
	resumeResp               *SquareSubscription
	deleteResp               *SquareSubscription
	createErr                error
	cancelErr                error
	getErr                   error
	pauseErr                 error
	resumeErr                error
	deleteErr                error
	calledCreate             bool
	calledCancel             bool
	calledPause              bool
	calledResume             bool
	calledGet                bool
	lastGetParams            *SquareSubscriptionParams
	lastPauseParams          *SquareSubscriptionPauseParams
	lastDeleteActionID       string
	lastDeleteSubscriptionID string
	calledDelete             bool
}

func (s *stubSquareSubscriptionClient) Create(ctx context.Context, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	s.calledCreate = true
	return s.createResp, s.createErr
}

func (s *stubSquareSubscriptionClient) Cancel(ctx context.Context, id string, params *SquareSubscriptionCancelParams) (*SquareSubscription, error) {
	s.calledCancel = true
	return s.cancelResp, s.cancelErr
}

func (s *stubSquareSubscriptionClient) Get(ctx context.Context, id string, params *SquareSubscriptionParams) (*SquareSubscription, error) {
	s.calledGet = true
	s.lastGetParams = params
	return s.getResp, s.getErr
}

func (s *stubSquareSubscriptionClient) Pause(ctx context.Context, id string, params *SquareSubscriptionPauseParams) (*SquareSubscription, error) {
	s.calledPause = true
	s.lastPauseParams = params
	return s.pauseResp, s.pauseErr
}

func (s *stubSquareSubscriptionClient) Resume(ctx context.Context, id string, params *SquareSubscriptionResumeParams) (*SquareSubscription, error) {
	s.calledResume = true
	return s.resumeResp, s.resumeErr
}

func (s *stubSquareSubscriptionClient) DeleteAction(ctx context.Context, id, actionID string) (*SquareSubscription, error) {
	s.calledDelete = true
	s.lastDeleteActionID = actionID
	s.lastDeleteSubscriptionID = id
	return s.deleteResp, s.deleteErr
}

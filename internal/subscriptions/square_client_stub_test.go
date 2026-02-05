package subscriptions

import "context"

type stubSquareSubscriptionClient struct {
	createResp   *SquareSubscription
	cancelResp   *SquareSubscription
	getResp      *SquareSubscription
	pauseResp    *SquareSubscription
	resumeResp   *SquareSubscription
	createErr    error
	cancelErr    error
	getErr       error
	pauseErr     error
	resumeErr    error
	calledCreate bool
	calledCancel bool
	calledPause  bool
	calledResume bool
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
	return s.getResp, s.getErr
}

func (s *stubSquareSubscriptionClient) Pause(ctx context.Context, id string, params *SquareSubscriptionPauseParams) (*SquareSubscription, error) {
	s.calledPause = true
	return s.pauseResp, s.pauseErr
}

func (s *stubSquareSubscriptionClient) Resume(ctx context.Context, id string, params *SquareSubscriptionResumeParams) (*SquareSubscription, error) {
	s.calledResume = true
	return s.resumeResp, s.resumeErr
}

package subscriptions

import "context"

type stubSquareSubscriptionClient struct {
	createResp   *SquareSubscription
	cancelResp   *SquareSubscription
	getResp      *SquareSubscription
	createErr    error
	cancelErr    error
	getErr       error
	calledCreate bool
	calledCancel bool
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

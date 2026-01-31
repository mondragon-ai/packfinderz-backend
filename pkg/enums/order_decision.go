package enums

// VendorOrderDecision represents the high-level decision a vendor can take.
type VendorOrderDecision string

const (
	// VendorOrderDecisionAccept indicates the vendor accepts the order.
	VendorOrderDecisionAccept VendorOrderDecision = "accept"
	// VendorOrderDecisionReject indicates the vendor rejects the order.
	VendorOrderDecisionReject VendorOrderDecision = "reject"
)

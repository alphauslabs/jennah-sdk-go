package jennah

import (
	"context"

	billingv1 "github.com/alphauslabs/jennah-sdk-go/jennah/billing/v1"
	platformv1 "github.com/alphauslabs/jennah-sdk-go/jennah/platform/v1"
)

// PlatformAPI is operator topology: where tenant resources can be created and what
// each place supports.
type PlatformAPI struct{ c *Client }

// Locations lists the places an agent workspace or dataset can live, and what each
// one is provisioned for. It describes the operator's fleet rather than any
// tenant's data, so it answers without a credential.
func (p PlatformAPI) Locations(ctx context.Context) (*platformv1.ListLocationsResponse, error) {
	return p.c.platform.ListLocations(ctx, &platformv1.ListLocationsRequest{})
}

// BillingAPI reads the enterprise's subscription and entitlement state, and binds
// a marketplace registration to it.
type BillingAPI struct{ c *Client }

// State reads the enterprise's current plan, subscription state and entitlements.
// This is the authoritative answer the platform's own limit enforcement reads, so
// it is what to check when a call is rejected for entitlement reasons.
func (b BillingAPI) State(ctx context.Context) (*billingv1.GetBillingStateResponse, error) {
	return b.c.billing.GetBillingState(ctx, &billingv1.GetBillingStateRequest{})
}

// ResolveMarketplace exchanges a marketplace registration token for the
// subscription it identifies. It runs before the caller is authenticated, since a
// buyer arriving from the marketplace may not have an enterprise yet.
func (b BillingAPI) ResolveMarketplace(ctx context.Context, registrationToken string) (*billingv1.ResolveMarketplaceRegistrationResponse, error) {
	return b.c.billing.ResolveMarketplaceRegistration(ctx, &billingv1.ResolveMarketplaceRegistrationRequest{
		RegistrationToken: registrationToken,
	})
}

// BindMarketplace attaches a resolved marketplace registration to the caller's
// enterprise, committing it to that agreement. It is gated by identity rather than
// by a permission, so treat it as a root-level action.
func (b BillingAPI) BindMarketplace(ctx context.Context, handle string) (*billingv1.BindMarketplaceRegistrationResponse, error) {
	return b.c.billing.BindMarketplaceRegistration(ctx, &billingv1.BindMarketplaceRegistrationRequest{Handle: handle})
}

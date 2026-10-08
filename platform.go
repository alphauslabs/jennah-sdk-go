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

// FormationTokenUsage reads the enterprise's recorded formation token usage as
// aggregated buckets: by day, ISO week or month in a time zone, and grouped by at
// most one of scope, caller, model or region, with input and output tokens kept
// apart. It needs billing.usage:read, which the built-in member role does not
// carry, and a scope breakdown additionally needs reach over every scope. It
// reports tokens only, never a price.
func (b BillingAPI) FormationTokenUsage(ctx context.Context, in *billingv1.GetFormationTokenUsageRequest) (*billingv1.GetFormationTokenUsageResponse, error) {
	return b.c.billing.GetFormationTokenUsage(ctx, in)
}

// SetFormationOverageCap sets how far past its monthly formation allowance the
// enterprise may form, as a multiple (0 to 10, in steps of 0.5) of its tier's
// overage unit band. Only a signed-in root or administrator can call it: an API
// key is refused, including one created by an administrator, so an agent cannot
// raise its own spending limit.
func (b BillingAPI) SetFormationOverageCap(ctx context.Context, multiple float64) (*billingv1.SetFormationOverageCapResponse, error) {
	return b.c.billing.SetFormationOverageCap(ctx, &billingv1.SetFormationOverageCapRequest{OverageMultiple: multiple})
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

package jennah

import (
	"context"

	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
)

// AuthAPI is identity and enterprise administration: who the credential is, and
// the membership, key and role surfaces that govern what it may do.
//
// Most of what hangs off here is management-class, and an API key can never hold
// a management-class scope. A key-authenticated caller is therefore refused on
// these calls, over gRPC exactly as over HTTP, and needs a signed-in user's
// access token instead. That is a property of the credential, not of the door it
// arrived at.
type AuthAPI struct {
	c *Client

	// Session is the sign-in and token lifecycle.
	Session SessionAPI
	// Keys mints, lists and revokes API keys.
	Keys KeysAPI
	// Members lists and administers the enterprise's members.
	Members MembersAPI
	// Invitations invites people to the enterprise and accepts those invites.
	Invitations InvitationsAPI
	// Roles is the permission catalog and the enterprise's custom roles.
	Roles RolesAPI
	// Enterprise is enterprise-level settings and root ownership.
	Enterprise EnterpriseAPI
}

// WhoAmI resolves the credential to an identity: the user or key, the enterprise
// it is scoped to, its permissions, and every enterprise it can switch to.
func (a AuthAPI) WhoAmI(ctx context.Context) (*authv1.WhoAmIResponse, error) {
	return a.c.auth.WhoAmI(ctx, &authv1.WhoAmIRequest{})
}

// SessionAPI is the sign-in and token lifecycle.
//
// These calls are publicly reachable and mostly unauthenticated, which is safe
// because what protects them lives in the handlers: the OAuth state is sealed and
// validated server-side, so it cannot be forged by reaching the RPC directly.
//
// The browser flows (StartLogin, CompleteLogin, ExchangeCode) still belong to the
// HTTP gateway in practice: they turn on redirects and cookies that a gRPC client
// has no way to carry. What is genuinely useful here is the device-code pair, for
// a headless or CLI-shaped integrator, and Refresh.
type SessionAPI struct{ c *Client }

// StartLogin begins a provider sign-in and returns the authorization URL to send
// the user to.
func (s SessionAPI) StartLogin(ctx context.Context, in *authv1.StartLoginRequest) (*authv1.StartLoginResponse, error) {
	return s.c.auth.StartLogin(ctx, in)
}

// CompleteLogin redeems a provider callback. The sealed state carried in the
// request is validated server-side, which is what makes the callback forgery-proof
// wherever it arrives.
func (s SessionAPI) CompleteLogin(ctx context.Context, in *authv1.CompleteLoginRequest) (*authv1.CompleteLoginResponse, error) {
	return s.c.auth.CompleteLogin(ctx, in)
}

// ExchangeCode redeems a one-time login code for tokens.
func (s SessionAPI) ExchangeCode(ctx context.Context, code string) (*authv1.ExchangeCodeResponse, error) {
	return s.c.auth.ExchangeCode(ctx, &authv1.ExchangeCodeRequest{Code: code})
}

// StartDeviceLogin begins the device-code flow, returning the user code and the
// URL to enter it at.
func (s SessionAPI) StartDeviceLogin(ctx context.Context, provider authv1.Provider) (*authv1.StartDeviceLoginResponse, error) {
	return s.c.auth.StartDeviceLogin(ctx, &authv1.StartDeviceLoginRequest{Provider: provider})
}

// PollDeviceLogin checks a device login's progress. Poll no faster than the
// interval the start call returned.
func (s SessionAPI) PollDeviceLogin(ctx context.Context, deviceCode string) (*authv1.PollDeviceLoginResponse, error) {
	return s.c.auth.PollDeviceLogin(ctx, &authv1.PollDeviceLoginRequest{DeviceCode: deviceCode})
}

// Refresh re-mints an access token from a refresh token.
//
// Passing a non-empty enterpriseID switches the session to that enterprise, which
// the caller must be a member of. Switching is exactly this call with a different
// scope, not a separate operation, because an access token is enterprise-scoped:
// re-minting it is what changes which enterprise the caller is acting in.
func (s SessionAPI) Refresh(ctx context.Context, refreshToken, enterpriseID string) (*authv1.RefreshTokenResponse, error) {
	return s.c.auth.RefreshToken(ctx, &authv1.RefreshTokenRequest{
		RefreshToken: refreshToken,
		EnterpriseId: enterpriseID,
	})
}

// Logout revokes the refresh session.
func (s SessionAPI) Logout(ctx context.Context, refreshToken string) (*authv1.LogoutResponse, error) {
	return s.c.auth.Logout(ctx, &authv1.LogoutRequest{RefreshToken: refreshToken})
}

// KeysAPI mints, lists and revokes the enterprise's API keys.
type KeysAPI struct{ c *Client }

// Create mints an API key. The response carries the only copy of the plaintext
// key: it is stored hashed, so it cannot be shown again.
//
// Scopes are validated at creation and again when the key is used, so a key can
// never carry management-class permissions however its row was written.
func (k KeysAPI) Create(ctx context.Context, in *authv1.CreateApiKeyRequest) (*authv1.CreateApiKeyResponse, error) {
	return k.c.auth.CreateApiKey(ctx, in)
}

// List returns a page of the enterprise's keys and their non-secret metadata.
func (k KeysAPI) List(ctx context.Context, in *authv1.ListApiKeysRequest) (*authv1.ListApiKeysResponse, error) {
	if in == nil {
		in = &authv1.ListApiKeysRequest{}
	}
	return k.c.auth.ListApiKeys(ctx, in)
}

// Revoke withdraws a key. Revocation is published to the fleet rather than waited
// on, so a request already in flight with that key can still complete.
func (k KeysAPI) Revoke(ctx context.Context, keyID string) (*authv1.RevokeApiKeyResponse, error) {
	return k.c.auth.RevokeApiKey(ctx, &authv1.RevokeApiKeyRequest{KeyId: keyID})
}

// MembersAPI lists and administers the enterprise's members.
type MembersAPI struct{ c *Client }

// List returns a page of the enterprise's members and their roles.
func (m MembersAPI) List(ctx context.Context, in *authv1.ListMembersRequest) (*authv1.ListMembersResponse, error) {
	if in == nil {
		in = &authv1.ListMembersRequest{}
	}
	return m.c.auth.ListMembers(ctx, in)
}

// ChangeRole sets a member's role. Pass either a built-in Role or, for a custom
// role, the request's CustomRoleId.
func (m MembersAPI) ChangeRole(ctx context.Context, in *authv1.ChangeMemberRoleRequest) (*authv1.ChangeMemberRoleResponse, error) {
	return m.c.auth.ChangeMemberRole(ctx, in)
}

// Remove drops a member from the enterprise.
func (m MembersAPI) Remove(ctx context.Context, userID string) (*authv1.RemoveMemberResponse, error) {
	return m.c.auth.RemoveMember(ctx, &authv1.RemoveMemberRequest{UserId: userID})
}

// InvitationsAPI invites people into the enterprise and accepts those invites.
type InvitationsAPI struct{ c *Client }

// Create invites an email address at a role, and sends the invitation.
func (i InvitationsAPI) Create(ctx context.Context, in *authv1.InviteMemberRequest) (*authv1.InviteMemberResponse, error) {
	return i.c.auth.InviteMember(ctx, in)
}

// List returns a page of the enterprise's invitations and their statuses.
func (i InvitationsAPI) List(ctx context.Context, in *authv1.ListInvitationsRequest) (*authv1.ListInvitationsResponse, error) {
	if in == nil {
		in = &authv1.ListInvitationsRequest{}
	}
	return i.c.auth.ListInvitations(ctx, in)
}

// Revoke withdraws a pending invitation.
func (i InvitationsAPI) Revoke(ctx context.Context, invitationID string) (*authv1.RevokeInvitationResponse, error) {
	return i.c.auth.RevokeInvitation(ctx, &authv1.RevokeInvitationRequest{InvitationId: invitationID})
}

// Accept redeems an invitation token, joining the invited user to the enterprise.
// The membership is additive: a user can belong to several enterprises, and the
// one they act in is whichever their access token is scoped to.
func (i InvitationsAPI) Accept(ctx context.Context, token string) (*authv1.AcceptInvitationResponse, error) {
	return i.c.auth.AcceptInvitation(ctx, &authv1.AcceptInvitationRequest{Token: token})
}

// RolesAPI is the platform's permission catalog and the enterprise's custom roles.
type RolesAPI struct{ c *Client }

// Permissions lists every permission the platform defines, which is the catalog a
// custom role's Permissions must be drawn from.
func (r RolesAPI) Permissions(ctx context.Context) (*authv1.ListPermissionsResponse, error) {
	return r.c.auth.ListPermissions(ctx, &authv1.ListPermissionsRequest{})
}

// Create defines a custom role: a set of permissions plus the agent and dataset
// selectors that bound which resources it reaches.
func (r RolesAPI) Create(ctx context.Context, in *authv1.CreateRoleRequest) (*authv1.CreateRoleResponse, error) {
	return r.c.auth.CreateRole(ctx, in)
}

// List returns a page of the enterprise's custom roles.
func (r RolesAPI) List(ctx context.Context, in *authv1.ListRolesRequest) (*authv1.ListRolesResponse, error) {
	if in == nil {
		in = &authv1.ListRolesRequest{}
	}
	return r.c.auth.ListRoles(ctx, in)
}

// Get reads one custom role.
func (r RolesAPI) Get(ctx context.Context, roleID string) (*authv1.GetRoleResponse, error) {
	return r.c.auth.GetRole(ctx, &authv1.GetRoleRequest{RoleId: roleID})
}

// Update edits a custom role.
//
// The selector fields are wrapper messages, not bare slices, so that clearing a
// role's reach is expressible: an unset wrapper leaves the selectors alone, while
// a set wrapper holding no entries removes them.
func (r RolesAPI) Update(ctx context.Context, in *authv1.UpdateRoleRequest) (*authv1.UpdateRoleResponse, error) {
	return r.c.auth.UpdateRole(ctx, in)
}

// Delete removes a custom role.
func (r RolesAPI) Delete(ctx context.Context, roleID string) (*authv1.DeleteRoleResponse, error) {
	return r.c.auth.DeleteRole(ctx, &authv1.DeleteRoleRequest{RoleId: roleID})
}

// EnterpriseAPI is enterprise-level settings and root ownership.
type EnterpriseAPI struct{ c *Client }

// Update edits the enterprise's settings.
func (e EnterpriseAPI) Update(ctx context.Context, in *authv1.UpdateEnterpriseRequest) (*authv1.UpdateEnterpriseResponse, error) {
	return e.c.auth.UpdateEnterprise(ctx, in)
}

// TransferRoot hands the enterprise's root ownership to another member.
//
// It is authorized by identity rather than by permission: root and admin hold the
// same permissions, so no permission could express "root only". The handler
// refuses an API-key caller outright, which is why this call needs a signed-in
// root user's token.
func (e EnterpriseAPI) TransferRoot(ctx context.Context, newRootUserID string) (*authv1.TransferRootResponse, error) {
	return e.c.auth.TransferRoot(ctx, &authv1.TransferRootRequest{NewRootUserId: newRootUserID})
}

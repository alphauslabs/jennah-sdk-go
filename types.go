package jennah

import (
	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	billingv1 "github.com/alphauslabs/jennah-sdk-go/jennah/billing/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	platformv1 "github.com/alphauslabs/jennah-sdk-go/jennah/platform/v1"
)

// Type aliases for generated messages and enums.
// Enum constants retain their generated names without package qualifiers.
// The datastore Dataset message is available as datastorev1.Dataset.

// Agent memory (jennah/agent/v1).
type (
	AgentInstance           = agentv1.AgentInstance
	AgentStatus             = agentv1.AgentStatus
	CommitMemoryRequest     = agentv1.CommitMemoryRequest
	CommitMemoryResponse    = agentv1.CommitMemoryResponse
	CreateAgentRequest      = agentv1.CreateAgentRequest
	CreateAgentResponse     = agentv1.CreateAgentResponse
	DeleteAgentRequest      = agentv1.DeleteAgentRequest
	DeleteAgentResponse     = agentv1.DeleteAgentResponse
	ExecutionLogStep        = agentv1.ExecutionLogStep
	FusedResult             = agentv1.FusedResult
	FusionDirection         = agentv1.FusionDirection
	GetAgentRequest         = agentv1.GetAgentRequest
	GetAgentResponse        = agentv1.GetAgentResponse
	GraphDirection          = agentv1.GraphDirection
	GraphEdge               = agentv1.GraphEdge
	GraphInspectResult      = agentv1.GraphInspectResult
	GraphNode               = agentv1.GraphNode
	GraphNodeMatch          = agentv1.GraphNodeMatch
	GraphQuery              = agentv1.GraphQuery
	GraphResult             = agentv1.GraphResult
	GraphStep               = agentv1.GraphStep
	GraphWrite              = agentv1.GraphWrite
	InspectGraph            = agentv1.InspectGraph
	InspectLog              = agentv1.InspectLog
	InspectMemoryRequest    = agentv1.InspectMemoryRequest
	InspectMemoryResponse   = agentv1.InspectMemoryResponse
	InspectVectors          = agentv1.InspectVectors
	ListAgentsRequest       = agentv1.ListAgentsRequest
	ListAgentsResponse      = agentv1.ListAgentsResponse
	LogQuery                = agentv1.LogQuery
	LogResult               = agentv1.LogResult
	MetadataFilter          = agentv1.MetadataFilter
	MetadataFilter_Operator = agentv1.MetadataFilter_Operator
	PropertyFilter          = agentv1.PropertyFilter
	QueryMemoryRequest      = agentv1.QueryMemoryRequest
	QueryMemoryResponse     = agentv1.QueryMemoryResponse
	SemanticMatch           = agentv1.SemanticMatch
	SemanticQuery           = agentv1.SemanticQuery
	SemanticResult          = agentv1.SemanticResult
	SupersedeEdgeRequest    = agentv1.SupersedeEdgeRequest
	SupersedeEdgeResponse   = agentv1.SupersedeEdgeResponse
	VectorChunk             = agentv1.VectorChunk
	VectorChunkInfo         = agentv1.VectorChunkInfo
	VectorInspectResult     = agentv1.VectorInspectResult
)

// Agent memory enum values (jennah/agent/v1).
const (
	AgentStatus_AGENT_STATUS_ACTIVE               = agentv1.AgentStatus_AGENT_STATUS_ACTIVE
	AgentStatus_AGENT_STATUS_COMPLETED            = agentv1.AgentStatus_AGENT_STATUS_COMPLETED
	AgentStatus_AGENT_STATUS_FAILED               = agentv1.AgentStatus_AGENT_STATUS_FAILED
	AgentStatus_AGENT_STATUS_PAUSED               = agentv1.AgentStatus_AGENT_STATUS_PAUSED
	AgentStatus_AGENT_STATUS_PROVISIONING         = agentv1.AgentStatus_AGENT_STATUS_PROVISIONING
	AgentStatus_AGENT_STATUS_UNSPECIFIED          = agentv1.AgentStatus_AGENT_STATUS_UNSPECIFIED
	FusionDirection_FUSION_DIRECTION_GRAPH_FIRST  = agentv1.FusionDirection_FUSION_DIRECTION_GRAPH_FIRST
	FusionDirection_FUSION_DIRECTION_UNSPECIFIED  = agentv1.FusionDirection_FUSION_DIRECTION_UNSPECIFIED
	FusionDirection_FUSION_DIRECTION_VECTOR_FIRST = agentv1.FusionDirection_FUSION_DIRECTION_VECTOR_FIRST
	GraphDirection_GRAPH_DIRECTION_ANY            = agentv1.GraphDirection_GRAPH_DIRECTION_ANY
	GraphDirection_GRAPH_DIRECTION_INCOMING       = agentv1.GraphDirection_GRAPH_DIRECTION_INCOMING
	GraphDirection_GRAPH_DIRECTION_OUTGOING       = agentv1.GraphDirection_GRAPH_DIRECTION_OUTGOING
	GraphDirection_GRAPH_DIRECTION_UNSPECIFIED    = agentv1.GraphDirection_GRAPH_DIRECTION_UNSPECIFIED
	MetadataFilter_OPERATOR_EQUALS                = agentv1.MetadataFilter_OPERATOR_EQUALS
	MetadataFilter_OPERATOR_GREATER_THAN          = agentv1.MetadataFilter_OPERATOR_GREATER_THAN
	MetadataFilter_OPERATOR_GREATER_THAN_OR_EQUAL = agentv1.MetadataFilter_OPERATOR_GREATER_THAN_OR_EQUAL
	MetadataFilter_OPERATOR_LESS_THAN             = agentv1.MetadataFilter_OPERATOR_LESS_THAN
	MetadataFilter_OPERATOR_LESS_THAN_OR_EQUAL    = agentv1.MetadataFilter_OPERATOR_LESS_THAN_OR_EQUAL
	MetadataFilter_OPERATOR_UNSPECIFIED           = agentv1.MetadataFilter_OPERATOR_UNSPECIFIED
)

// Human approvals (jennah/approval/v1).
type (
	AddApproverRequest                 = approvalv1.AddApproverRequest
	AddApproverResponse                = approvalv1.AddApproverResponse
	AllowlistKind                      = approvalv1.AllowlistKind
	Approval                           = approvalv1.Approval
	ApprovalDecision                   = approvalv1.ApprovalDecision
	ApprovalStatus                     = approvalv1.ApprovalStatus
	Approver                           = approvalv1.Approver
	ApproverAllowlistEntry             = approvalv1.ApproverAllowlistEntry
	ApproverChannel                    = approvalv1.ApproverChannel
	ApproverSpec                       = approvalv1.ApproverSpec
	CancelApprovalRequest              = approvalv1.CancelApprovalRequest
	CancelApprovalResponse             = approvalv1.CancelApprovalResponse
	CreateApprovalRequest              = approvalv1.CreateApprovalRequest
	CreateApprovalResponse             = approvalv1.CreateApprovalResponse
	DecisionValue                      = approvalv1.DecisionValue
	DeliveryState                      = approvalv1.DeliveryState
	DescribeApprovalByTokenRequest     = approvalv1.DescribeApprovalByTokenRequest
	DescribeApprovalByTokenResponse    = approvalv1.DescribeApprovalByTokenResponse
	GetApprovalRequest                 = approvalv1.GetApprovalRequest
	GetApprovalResponse                = approvalv1.GetApprovalResponse
	ListApprovalsRequest               = approvalv1.ListApprovalsRequest
	ListApprovalsResponse              = approvalv1.ListApprovalsResponse
	ListApproversRequest               = approvalv1.ListApproversRequest
	ListApproversResponse              = approvalv1.ListApproversResponse
	OnExpire                           = approvalv1.OnExpire
	Quorum                             = approvalv1.Quorum
	RemoveApproverRequest              = approvalv1.RemoveApproverRequest
	RemoveApproverResponse             = approvalv1.RemoveApproverResponse
	ResendApprovalNotificationRequest  = approvalv1.ResendApprovalNotificationRequest
	ResendApprovalNotificationResponse = approvalv1.ResendApprovalNotificationResponse
	SubmitApprovalDecisionRequest      = approvalv1.SubmitApprovalDecisionRequest
	SubmitApprovalDecisionResponse     = approvalv1.SubmitApprovalDecisionResponse
	WaitApprovalRequest                = approvalv1.WaitApprovalRequest
	WaitApprovalResponse               = approvalv1.WaitApprovalResponse
)

// Human approvals enum values (jennah/approval/v1).
const (
	AllowlistKind_ALLOWLIST_KIND_DOMAIN          = approvalv1.AllowlistKind_ALLOWLIST_KIND_DOMAIN
	AllowlistKind_ALLOWLIST_KIND_EMAIL           = approvalv1.AllowlistKind_ALLOWLIST_KIND_EMAIL
	AllowlistKind_ALLOWLIST_KIND_UNSPECIFIED     = approvalv1.AllowlistKind_ALLOWLIST_KIND_UNSPECIFIED
	ApprovalStatus_APPROVAL_STATUS_APPROVED      = approvalv1.ApprovalStatus_APPROVAL_STATUS_APPROVED
	ApprovalStatus_APPROVAL_STATUS_CANCELLED     = approvalv1.ApprovalStatus_APPROVAL_STATUS_CANCELLED
	ApprovalStatus_APPROVAL_STATUS_EXPIRED       = approvalv1.ApprovalStatus_APPROVAL_STATUS_EXPIRED
	ApprovalStatus_APPROVAL_STATUS_PENDING       = approvalv1.ApprovalStatus_APPROVAL_STATUS_PENDING
	ApprovalStatus_APPROVAL_STATUS_REJECTED      = approvalv1.ApprovalStatus_APPROVAL_STATUS_REJECTED
	ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED   = approvalv1.ApprovalStatus_APPROVAL_STATUS_UNSPECIFIED
	ApproverChannel_APPROVER_CHANNEL_EMAIL       = approvalv1.ApproverChannel_APPROVER_CHANNEL_EMAIL
	ApproverChannel_APPROVER_CHANNEL_UNSPECIFIED = approvalv1.ApproverChannel_APPROVER_CHANNEL_UNSPECIFIED
	DecisionValue_DECISION_VALUE_APPROVE         = approvalv1.DecisionValue_DECISION_VALUE_APPROVE
	DecisionValue_DECISION_VALUE_REJECT          = approvalv1.DecisionValue_DECISION_VALUE_REJECT
	DecisionValue_DECISION_VALUE_UNSPECIFIED     = approvalv1.DecisionValue_DECISION_VALUE_UNSPECIFIED
	DeliveryState_DELIVERY_STATE_DELIVERED       = approvalv1.DeliveryState_DELIVERY_STATE_DELIVERED
	DeliveryState_DELIVERY_STATE_FAILED          = approvalv1.DeliveryState_DELIVERY_STATE_FAILED
	DeliveryState_DELIVERY_STATE_PENDING         = approvalv1.DeliveryState_DELIVERY_STATE_PENDING
	DeliveryState_DELIVERY_STATE_UNSPECIFIED     = approvalv1.DeliveryState_DELIVERY_STATE_UNSPECIFIED
	OnExpire_ON_EXPIRE_DENY                      = approvalv1.OnExpire_ON_EXPIRE_DENY
	OnExpire_ON_EXPIRE_EXPIRE                    = approvalv1.OnExpire_ON_EXPIRE_EXPIRE
	OnExpire_ON_EXPIRE_UNSPECIFIED               = approvalv1.OnExpire_ON_EXPIRE_UNSPECIFIED
	Quorum_QUORUM_ALL                            = approvalv1.Quorum_QUORUM_ALL
	Quorum_QUORUM_ANY_ONE                        = approvalv1.Quorum_QUORUM_ANY_ONE
	Quorum_QUORUM_UNSPECIFIED                    = approvalv1.Quorum_QUORUM_UNSPECIFIED
)

// Identity, enterprise administration, and API keys (jennah/auth/v1).
type (
	AcceptInvitationRequest        = authv1.AcceptInvitationRequest
	AcceptInvitationResponse       = authv1.AcceptInvitationResponse
	AgentSelectorList              = authv1.AgentSelectorList
	ApiKey                         = authv1.ApiKey
	ChangeMemberRoleRequest        = authv1.ChangeMemberRoleRequest
	ChangeMemberRoleResponse       = authv1.ChangeMemberRoleResponse
	ClientType                     = authv1.ClientType
	CompleteLoginRequest           = authv1.CompleteLoginRequest
	CompleteLoginResponse          = authv1.CompleteLoginResponse
	CreateApiKeyRequest            = authv1.CreateApiKeyRequest
	CreateApiKeyResponse           = authv1.CreateApiKeyResponse
	CreateRoleRequest              = authv1.CreateRoleRequest
	CreateRoleResponse             = authv1.CreateRoleResponse
	CustomRole                     = authv1.CustomRole
	DatasetSelectorList            = authv1.DatasetSelectorList
	DeleteRoleRequest              = authv1.DeleteRoleRequest
	DeleteRoleResponse             = authv1.DeleteRoleResponse
	Enterprise                     = authv1.Enterprise
	Entitlement                    = authv1.Entitlement
	ExchangeCodeRequest            = authv1.ExchangeCodeRequest
	ExchangeCodeResponse           = authv1.ExchangeCodeResponse
	GetRoleRequest                 = authv1.GetRoleRequest
	GetRoleResponse                = authv1.GetRoleResponse
	Identity                       = authv1.Identity
	Invitation                     = authv1.Invitation
	InvitationStatus               = authv1.InvitationStatus
	InviteMemberRequest            = authv1.InviteMemberRequest
	InviteMemberResponse           = authv1.InviteMemberResponse
	ListApiKeysRequest             = authv1.ListApiKeysRequest
	ListApiKeysResponse            = authv1.ListApiKeysResponse
	ListInvitationsRequest         = authv1.ListInvitationsRequest
	ListInvitationsResponse        = authv1.ListInvitationsResponse
	ListMembersRequest             = authv1.ListMembersRequest
	ListMembersResponse            = authv1.ListMembersResponse
	ListPermissionsRequest         = authv1.ListPermissionsRequest
	ListPermissionsResponse        = authv1.ListPermissionsResponse
	ListRolesRequest               = authv1.ListRolesRequest
	ListRolesResponse              = authv1.ListRolesResponse
	LogoutRequest                  = authv1.LogoutRequest
	LogoutResponse                 = authv1.LogoutResponse
	Member                         = authv1.Member
	Membership                     = authv1.Membership
	Permission                     = authv1.Permission
	PollDeviceLoginRequest         = authv1.PollDeviceLoginRequest
	PollDeviceLoginResponse        = authv1.PollDeviceLoginResponse
	PollDeviceLoginResponse_Status = authv1.PollDeviceLoginResponse_Status
	Provider                       = authv1.Provider
	RefreshTokenRequest            = authv1.RefreshTokenRequest
	RefreshTokenResponse           = authv1.RefreshTokenResponse
	RemoveMemberRequest            = authv1.RemoveMemberRequest
	RemoveMemberResponse           = authv1.RemoveMemberResponse
	ResponseMode                   = authv1.ResponseMode
	RevokeApiKeyRequest            = authv1.RevokeApiKeyRequest
	RevokeApiKeyResponse           = authv1.RevokeApiKeyResponse
	RevokeInvitationRequest        = authv1.RevokeInvitationRequest
	RevokeInvitationResponse       = authv1.RevokeInvitationResponse
	Role                           = authv1.Role
	StartDeviceLoginRequest        = authv1.StartDeviceLoginRequest
	StartDeviceLoginResponse       = authv1.StartDeviceLoginResponse
	StartLoginRequest              = authv1.StartLoginRequest
	StartLoginResponse             = authv1.StartLoginResponse
	TransferRootRequest            = authv1.TransferRootRequest
	TransferRootResponse           = authv1.TransferRootResponse
	UpdateEnterpriseRequest        = authv1.UpdateEnterpriseRequest
	UpdateEnterpriseResponse       = authv1.UpdateEnterpriseResponse
	UpdateRoleRequest              = authv1.UpdateRoleRequest
	UpdateRoleResponse             = authv1.UpdateRoleResponse
	WhoAmIRequest                  = authv1.WhoAmIRequest
	WhoAmIResponse                 = authv1.WhoAmIResponse
)

// Identity, enterprise administration, and API keys enum values (jennah/auth/v1).
const (
	ClientType_CLIENT_TYPE_CLI                     = authv1.ClientType_CLIENT_TYPE_CLI
	ClientType_CLIENT_TYPE_UNSPECIFIED             = authv1.ClientType_CLIENT_TYPE_UNSPECIFIED
	ClientType_CLIENT_TYPE_WEB                     = authv1.ClientType_CLIENT_TYPE_WEB
	InvitationStatus_INVITATION_STATUS_ACCEPTED    = authv1.InvitationStatus_INVITATION_STATUS_ACCEPTED
	InvitationStatus_INVITATION_STATUS_PENDING     = authv1.InvitationStatus_INVITATION_STATUS_PENDING
	InvitationStatus_INVITATION_STATUS_REVOKED     = authv1.InvitationStatus_INVITATION_STATUS_REVOKED
	InvitationStatus_INVITATION_STATUS_UNSPECIFIED = authv1.InvitationStatus_INVITATION_STATUS_UNSPECIFIED
	PollDeviceLoginResponse_STATUS_APPROVED        = authv1.PollDeviceLoginResponse_STATUS_APPROVED
	PollDeviceLoginResponse_STATUS_DENIED          = authv1.PollDeviceLoginResponse_STATUS_DENIED
	PollDeviceLoginResponse_STATUS_EXPIRED         = authv1.PollDeviceLoginResponse_STATUS_EXPIRED
	PollDeviceLoginResponse_STATUS_PENDING         = authv1.PollDeviceLoginResponse_STATUS_PENDING
	PollDeviceLoginResponse_STATUS_UNSPECIFIED     = authv1.PollDeviceLoginResponse_STATUS_UNSPECIFIED
	Provider_PROVIDER_GITHUB                       = authv1.Provider_PROVIDER_GITHUB
	Provider_PROVIDER_GOOGLE                       = authv1.Provider_PROVIDER_GOOGLE
	Provider_PROVIDER_UNSPECIFIED                  = authv1.Provider_PROVIDER_UNSPECIFIED
	ResponseMode_RESPONSE_MODE_CODE                = authv1.ResponseMode_RESPONSE_MODE_CODE
	ResponseMode_RESPONSE_MODE_FRAGMENT            = authv1.ResponseMode_RESPONSE_MODE_FRAGMENT
	ResponseMode_RESPONSE_MODE_UNSPECIFIED         = authv1.ResponseMode_RESPONSE_MODE_UNSPECIFIED
	Role_ROLE_ADMIN                                = authv1.Role_ROLE_ADMIN
	Role_ROLE_MEMBER                               = authv1.Role_ROLE_MEMBER
	Role_ROLE_ROOT                                 = authv1.Role_ROLE_ROOT
	Role_ROLE_UNSPECIFIED                          = authv1.Role_ROLE_UNSPECIFIED
)

// Billing and entitlement state (jennah/billing/v1).
type (
	BillingSource                          = billingv1.BillingSource
	BindMarketplaceRegistrationRequest     = billingv1.BindMarketplaceRegistrationRequest
	BindMarketplaceRegistrationResponse    = billingv1.BindMarketplaceRegistrationResponse
	BoundSubscription                      = billingv1.BoundSubscription
	GetBillingStateRequest                 = billingv1.GetBillingStateRequest
	GetBillingStateResponse                = billingv1.GetBillingStateResponse
	ResolveMarketplaceRegistrationRequest  = billingv1.ResolveMarketplaceRegistrationRequest
	ResolveMarketplaceRegistrationResponse = billingv1.ResolveMarketplaceRegistrationResponse
	SubscriptionState                      = billingv1.SubscriptionState
)

// Billing and entitlement state enum values (jennah/billing/v1).
const (
	BillingSource_BILLING_SOURCE_AWS_MARKETPLACE              = billingv1.BillingSource_BILLING_SOURCE_AWS_MARKETPLACE
	BillingSource_BILLING_SOURCE_OPERATOR                     = billingv1.BillingSource_BILLING_SOURCE_OPERATOR
	BillingSource_BILLING_SOURCE_PLATFORM                     = billingv1.BillingSource_BILLING_SOURCE_PLATFORM
	BillingSource_BILLING_SOURCE_UNSPECIFIED                  = billingv1.BillingSource_BILLING_SOURCE_UNSPECIFIED
	SubscriptionState_SUBSCRIPTION_STATE_ACTIVE               = billingv1.SubscriptionState_SUBSCRIPTION_STATE_ACTIVE
	SubscriptionState_SUBSCRIPTION_STATE_CANCELLED            = billingv1.SubscriptionState_SUBSCRIPTION_STATE_CANCELLED
	SubscriptionState_SUBSCRIPTION_STATE_NEEDS_ATTENTION      = billingv1.SubscriptionState_SUBSCRIPTION_STATE_NEEDS_ATTENTION
	SubscriptionState_SUBSCRIPTION_STATE_PENDING_VERIFICATION = billingv1.SubscriptionState_SUBSCRIPTION_STATE_PENDING_VERIFICATION
	SubscriptionState_SUBSCRIPTION_STATE_UNBOUND              = billingv1.SubscriptionState_SUBSCRIPTION_STATE_UNBOUND
	SubscriptionState_SUBSCRIPTION_STATE_UNSPECIFIED          = billingv1.SubscriptionState_SUBSCRIPTION_STATE_UNSPECIFIED
)

// Application datasets (jennah/datastore/v1).
type (
	ColumnDeclaration            = datastorev1.ColumnDeclaration
	ColumnExpression             = datastorev1.ColumnExpression
	ColumnExpression_Operator    = datastorev1.ColumnExpression_Operator
	ColumnSchema                 = datastorev1.ColumnSchema
	ColumnType                   = datastorev1.ColumnType
	CommitDataRequest            = datastorev1.CommitDataRequest
	CommitDataResponse           = datastorev1.CommitDataResponse
	CreateDatasetRequest         = datastorev1.CreateDatasetRequest
	CreateDatasetResponse        = datastorev1.CreateDatasetResponse
	DatasetStatus                = datastorev1.DatasetStatus
	DeclareTablesRequest         = datastorev1.DeclareTablesRequest
	DeclareTablesResponse        = datastorev1.DeclareTablesResponse
	DeleteDatasetRequest         = datastorev1.DeleteDatasetRequest
	DeleteDatasetResponse        = datastorev1.DeleteDatasetResponse
	GetDatasetRequest            = datastorev1.GetDatasetRequest
	GetDatasetResponse           = datastorev1.GetDatasetResponse
	GetSchemaRequest             = datastorev1.GetSchemaRequest
	GetSchemaResponse            = datastorev1.GetSchemaResponse
	IndexDeclaration             = datastorev1.IndexDeclaration
	Join                         = datastorev1.Join
	JoinType                     = datastorev1.JoinType
	ListDatasetsRequest          = datastorev1.ListDatasetsRequest
	ListDatasetsResponse         = datastorev1.ListDatasetsResponse
	OperationType                = datastorev1.OperationType
	OrderBy                      = datastorev1.OrderBy
	Predicate                    = datastorev1.Predicate
	Predicate_Operator           = datastorev1.Predicate_Operator
	QueryDataRequest             = datastorev1.QueryDataRequest
	QueryDataResponse            = datastorev1.QueryDataResponse
	ReadStaleness                = datastorev1.ReadStaleness
	ReadStaleness_ExactStaleness = datastorev1.ReadStaleness_ExactStaleness
	ReadStaleness_MaxStaleness   = datastorev1.ReadStaleness_MaxStaleness
	ReadStaleness_ReadTimestamp  = datastorev1.ReadStaleness_ReadTimestamp
	RelationalQuery              = datastorev1.RelationalQuery
	RelationalResult             = datastorev1.RelationalResult
	Row                          = datastorev1.Row
	RowExpectation               = datastorev1.RowExpectation
	RowExpectation_AtLeastOne    = datastorev1.RowExpectation_AtLeastOne
	RowExpectation_Exactly       = datastorev1.RowExpectation_Exactly
	RowOperation                 = datastorev1.RowOperation
	SchemaStatus                 = datastorev1.SchemaStatus
	SchemaUsage                  = datastorev1.SchemaUsage
	TableDeclaration             = datastorev1.TableDeclaration
	TableSchema                  = datastorev1.TableSchema
	TableStatus                  = datastorev1.TableStatus
	TruncationReport             = datastorev1.TruncationReport
	Value                        = datastorev1.Value
	Value_BoolValue              = datastorev1.Value_BoolValue
	Value_BytesValue             = datastorev1.Value_BytesValue
	Value_DateValue              = datastorev1.Value_DateValue
	Value_Float64Value           = datastorev1.Value_Float64Value
	Value_Int64Value             = datastorev1.Value_Int64Value
	Value_JsonValue              = datastorev1.Value_JsonValue
	Value_NullValue              = datastorev1.Value_NullValue
	Value_StringValue            = datastorev1.Value_StringValue
	Value_TimestampValue         = datastorev1.Value_TimestampValue
	Value_VectorValue            = datastorev1.Value_VectorValue
	VectorMatch                  = datastorev1.VectorMatch
	VectorOptions                = datastorev1.VectorOptions
	VectorQuery                  = datastorev1.VectorQuery
	VectorResult                 = datastorev1.VectorResult
	VectorValue                  = datastorev1.VectorValue
)

// Application datasets enum values (jennah/datastore/v1).
const (
	ColumnExpression_OPERATOR_ADD             = datastorev1.ColumnExpression_OPERATOR_ADD
	ColumnExpression_OPERATOR_SUBTRACT        = datastorev1.ColumnExpression_OPERATOR_SUBTRACT
	ColumnExpression_OPERATOR_UNSPECIFIED     = datastorev1.ColumnExpression_OPERATOR_UNSPECIFIED
	ColumnType_COLUMN_TYPE_BOOL               = datastorev1.ColumnType_COLUMN_TYPE_BOOL
	ColumnType_COLUMN_TYPE_BYTES              = datastorev1.ColumnType_COLUMN_TYPE_BYTES
	ColumnType_COLUMN_TYPE_DATE               = datastorev1.ColumnType_COLUMN_TYPE_DATE
	ColumnType_COLUMN_TYPE_FLOAT64            = datastorev1.ColumnType_COLUMN_TYPE_FLOAT64
	ColumnType_COLUMN_TYPE_INT64              = datastorev1.ColumnType_COLUMN_TYPE_INT64
	ColumnType_COLUMN_TYPE_JSON               = datastorev1.ColumnType_COLUMN_TYPE_JSON
	ColumnType_COLUMN_TYPE_STRING             = datastorev1.ColumnType_COLUMN_TYPE_STRING
	ColumnType_COLUMN_TYPE_TIMESTAMP          = datastorev1.ColumnType_COLUMN_TYPE_TIMESTAMP
	ColumnType_COLUMN_TYPE_UNSPECIFIED        = datastorev1.ColumnType_COLUMN_TYPE_UNSPECIFIED
	ColumnType_COLUMN_TYPE_VECTOR             = datastorev1.ColumnType_COLUMN_TYPE_VECTOR
	DatasetStatus_DATASET_STATUS_ACTIVE       = datastorev1.DatasetStatus_DATASET_STATUS_ACTIVE
	DatasetStatus_DATASET_STATUS_DELETING     = datastorev1.DatasetStatus_DATASET_STATUS_DELETING
	DatasetStatus_DATASET_STATUS_FAILED       = datastorev1.DatasetStatus_DATASET_STATUS_FAILED
	DatasetStatus_DATASET_STATUS_PROVISIONING = datastorev1.DatasetStatus_DATASET_STATUS_PROVISIONING
	DatasetStatus_DATASET_STATUS_UNSPECIFIED  = datastorev1.DatasetStatus_DATASET_STATUS_UNSPECIFIED
	JoinType_JOIN_TYPE_INNER                  = datastorev1.JoinType_JOIN_TYPE_INNER
	JoinType_JOIN_TYPE_LEFT                   = datastorev1.JoinType_JOIN_TYPE_LEFT
	JoinType_JOIN_TYPE_UNSPECIFIED            = datastorev1.JoinType_JOIN_TYPE_UNSPECIFIED
	OperationType_OPERATION_TYPE_DELETE       = datastorev1.OperationType_OPERATION_TYPE_DELETE
	OperationType_OPERATION_TYPE_INSERT       = datastorev1.OperationType_OPERATION_TYPE_INSERT
	OperationType_OPERATION_TYPE_UNSPECIFIED  = datastorev1.OperationType_OPERATION_TYPE_UNSPECIFIED
	OperationType_OPERATION_TYPE_UPDATE       = datastorev1.OperationType_OPERATION_TYPE_UPDATE
	OperationType_OPERATION_TYPE_UPSERT       = datastorev1.OperationType_OPERATION_TYPE_UPSERT
	Predicate_OPERATOR_EQUALS                 = datastorev1.Predicate_OPERATOR_EQUALS
	Predicate_OPERATOR_GREATER_THAN           = datastorev1.Predicate_OPERATOR_GREATER_THAN
	Predicate_OPERATOR_GREATER_THAN_OR_EQUAL  = datastorev1.Predicate_OPERATOR_GREATER_THAN_OR_EQUAL
	Predicate_OPERATOR_LESS_THAN              = datastorev1.Predicate_OPERATOR_LESS_THAN
	Predicate_OPERATOR_LESS_THAN_OR_EQUAL     = datastorev1.Predicate_OPERATOR_LESS_THAN_OR_EQUAL
	Predicate_OPERATOR_UNSPECIFIED            = datastorev1.Predicate_OPERATOR_UNSPECIFIED
	SchemaStatus_SCHEMA_STATUS_FAILED         = datastorev1.SchemaStatus_SCHEMA_STATUS_FAILED
	SchemaStatus_SCHEMA_STATUS_PENDING        = datastorev1.SchemaStatus_SCHEMA_STATUS_PENDING
	SchemaStatus_SCHEMA_STATUS_READY          = datastorev1.SchemaStatus_SCHEMA_STATUS_READY
	SchemaStatus_SCHEMA_STATUS_UNSPECIFIED    = datastorev1.SchemaStatus_SCHEMA_STATUS_UNSPECIFIED
)

// Platform topology (jennah/platform/v1).
type (
	AgentSupport          = platformv1.AgentSupport
	DatasetSupport        = platformv1.DatasetSupport
	Geography             = platformv1.Geography
	ListLocationsRequest  = platformv1.ListLocationsRequest
	ListLocationsResponse = platformv1.ListLocationsResponse
	Location              = platformv1.Location
)

// Platform topology enum values (jennah/platform/v1).
const (
	Geography_GEOGRAPHY_GLOBAL        = platformv1.Geography_GEOGRAPHY_GLOBAL
	Geography_GEOGRAPHY_MULTI_REGION  = platformv1.Geography_GEOGRAPHY_MULTI_REGION
	Geography_GEOGRAPHY_SINGLE_REGION = platformv1.Geography_GEOGRAPHY_SINGLE_REGION
	Geography_GEOGRAPHY_UNSPECIFIED   = platformv1.Geography_GEOGRAPHY_UNSPECIFIED
)

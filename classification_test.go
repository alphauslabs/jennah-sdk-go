package jennah

import (
	"testing"

	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	billingv1 "github.com/alphauslabs/jennah-sdk-go/jennah/billing/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	platformv1 "github.com/alphauslabs/jennah-sdk-go/jennah/platform/v1"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Every method the API publishes must be classified exactly once: replayable
// because it only reads, conditional because its safety depends on the request, or
// excluded from retries.
//
// The default in safeToReplay is "do not replay", which is the safe default but
// also a silent one: a new RPC would inherit it without anyone deciding. This test
// is where that decision is forced, and it is the same discipline the backend
// applies to its own per-method registries.
func TestEveryMethodIsClassified(t *testing.T) {
	descs := []grpc.ServiceDesc{
		agentv1.AgentService_ServiceDesc,
		agentv1.MemoryService_ServiceDesc,
		datastorev1.DatasetService_ServiceDesc,
		datastorev1.SchemaService_ServiceDesc,
		datastorev1.DataService_ServiceDesc,
		authv1.AuthService_ServiceDesc,
		approvalv1.ApprovalService_ServiceDesc,
		billingv1.BillingService_ServiceDesc,
		platformv1.PlatformService_ServiceDesc,
		healthpb.Health_ServiceDesc,
	}

	classified := map[string]bool{}
	var total int
	for _, d := range descs {
		for _, m := range d.Methods {
			full := "/" + d.ServiceName + "/" + m.MethodName
			total++
			classified[full] = true

			var in int
			for _, set := range []map[string]bool{replayableReads, conditionalReplay, neverReplay} {
				if set[full] {
					in++
				}
			}
			if in != 1 {
				t.Errorf("%s is in %d classification sets, want exactly 1", full, in)
			}
		}
	}

	// The API publishes 55 methods across nine services; health adds its own.
	if total < 55 {
		t.Errorf("walked %d methods, expected at least the 55 the API publishes", total)
	}

	// The reverse direction: a classification entry naming a method that no longer
	// exists is dead weight and hides the fact that its RPC was renamed.
	for _, set := range []map[string]bool{replayableReads, conditionalReplay, neverReplay} {
		for full := range set {
			if !classified[full] {
				t.Errorf("%s is classified but no service publishes it", full)
			}
		}
	}
}

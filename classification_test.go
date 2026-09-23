package jennah

import (
	"strings"
	"testing"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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
	// Services are discovered from the proto registry, not listed by hand: a hand
	// list is exactly what let ScopeService go unclassified, since adding a
	// service never reminded anyone to add it here. The registry holds every
	// jennahapi package this module imports, which is all of them while the Client
	// wraps each one; a wholly new package still needs importing somewhere.
	type method struct{ service, name string }
	var methods []method
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "jennahapi.") {
			return true
		}
		for i := range fd.Services().Len() {
			sd := fd.Services().Get(i)
			for j := range sd.Methods().Len() {
				methods = append(methods, method{string(sd.FullName()), string(sd.Methods().Get(j).Name())})
			}
		}
		return true
	})
	for _, m := range healthpb.Health_ServiceDesc.Methods {
		methods = append(methods, method{healthpb.Health_ServiceDesc.ServiceName, m.MethodName})
	}

	classified := map[string]bool{}
	var total int
	for _, m := range methods {
		full := "/" + m.service + "/" + m.name
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

	// The API publishes 66 methods across ten services; health adds its own. A
	// floor rather than an exact count, so it catches a registry walk that found
	// nothing without failing every time an RPC is added.
	if total < 66 {
		t.Errorf("walked %d methods, expected at least the 66 the API publishes", total)
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

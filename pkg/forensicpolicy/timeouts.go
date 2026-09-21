// Package forensicpolicy shares bounded request policies across the sidecar,
// agent caller and public proxy.
package forensicpolicy

import "time"

const DefaultSynthesisTimeout = 120 * time.Second
const MaximumModelTimeout = 180 * time.Second

// A governed interaction can spend one model budget selecting an operation,
// thirty seconds executing it and another model budget synthesizing the facts.
// This transport ceiling includes fifteen seconds to deliver the response; it
// does not widen an individual model call or an earlier caller deadline.
const QueryTransportTimeout = 2*MaximumModelTimeout + 45*time.Second

package main

import (
	"github.com/mudler/LocalAI/pkg/forensicpolicy"
	"time"
)

const defaultSynthesisWait = forensicpolicy.DefaultSynthesisTimeout
const maximumInferenceWait = forensicpolicy.MaximumModelTimeout

// Both narrative and legacy synthesis share this policy. Context.WithTimeout
// at the call site also preserves an earlier caller deadline or cancellation.
func synthesisRequestTimeout(cfg config) time.Duration {
	if cfg.SynthesisTimeout <= 0 {
		return defaultSynthesisWait
	}
	if cfg.SynthesisTimeout > maximumInferenceWait {
		return maximumInferenceWait
	}
	return cfg.SynthesisTimeout
}

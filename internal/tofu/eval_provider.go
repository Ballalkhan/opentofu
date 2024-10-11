// Copyright (c) The OpenTofu Authors
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2023 HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tofu

import (
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"

	"github.com/opentofu/opentofu/internal/addrs"
	"github.com/opentofu/opentofu/internal/configs"
	"github.com/opentofu/opentofu/internal/configs/hcl2shim"
	"github.com/opentofu/opentofu/internal/providers"
)

func buildProviderConfig(ctx EvalContext, addr addrs.ConfigProviderInstance, config *configs.Provider) hcl.Body {
	var configBody hcl.Body
	if config != nil {
		configBody = config.Config
	}

	var inputBody hcl.Body
	inputConfig := ctx.ProviderInput(addr)
	if len(inputConfig) > 0 {
		inputBody = configs.SynthBody("<input-prompt>", inputConfig)
	}

	switch {
	case configBody != nil && inputBody != nil:
		log.Printf("[TRACE] buildProviderConfig for %s: merging explicit config and input", addr)
		return hcl.MergeBodies([]hcl.Body{inputBody, configBody})
	case configBody != nil:
		log.Printf("[TRACE] buildProviderConfig for %s: using explicit config only", addr)
		return configBody
	case inputBody != nil:
		log.Printf("[TRACE] buildProviderConfig for %s: using input only", addr)
		return inputBody
	default:
		log.Printf("[TRACE] buildProviderConfig for %s: no configuration at all", addr)
		addr := fmt.Sprintf("%s with no configuration", addr)
		return hcl2shim.SynthBody(addr, make(map[string]cty.Value))
	}
}

// getProvider returns the providers.Interface and schema for a given provider.
func getProvider(ctx EvalContext, addr addrs.ConfigProviderInstance) (providers.Interface, providers.ProviderSchema, error) {
	if addr.Provider.Type == "" {
		// Should never happen
		panic("GetProvider used with uninitialized provider configuration address")
	}
	provider := ctx.Provider(addr)
	if provider == nil {
		return nil, providers.ProviderSchema{}, fmt.Errorf("provider %s not initialized", addr)
	}
	// Not all callers require a schema, so we will leave checking for a nil
	// schema to the callers.
	schema, err := ctx.ProviderSchema(addr)
	if err != nil {
		return nil, providers.ProviderSchema{}, fmt.Errorf("failed to read schema for provider %s: %w", addr, err)
	}
	return provider, schema, nil
}

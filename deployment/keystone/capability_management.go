package keystone

import (
	"fmt"
	"math/big"

	"github.com/smartcontractkit/ccip-owner-contracts/pkg/proposal/mcms"
	"github.com/smartcontractkit/ccip-owner-contracts/pkg/proposal/timelock"
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/deployment"
	kcr "github.com/smartcontractkit/chainlink/v2/core/gethwrappers/keystone/generated/capabilities_registry"
)

// AddCapabilities adds the capabilities to the registry
func AddCapabilities(lggr logger.Logger, contractSet *ContractSet, chain deployment.Chain, capabilities []kcr.CapabilitiesRegistryCapability, useMCMS bool) (*timelock.BatchChainOperation, error) {
	if len(capabilities) == 0 {
		return nil, nil
	}
	registry := contractSet.CapabilitiesRegistry
	deduped, err := dedupCapabilities(registry, capabilities)
	if err != nil {
		return nil, fmt.Errorf("failed to dedup capabilities: %w", err)
	}
	txOpts := chain.DeployerKey
	if useMCMS {
		txOpts = deployment.SimTransactOpts()
	}
	tx, err := registry.AddCapabilities(txOpts, deduped)
	if err != nil {
		err = DecodeErr(kcr.CapabilitiesRegistryABI, err)
		return nil, fmt.Errorf("failed to add capabilities: %w", err)
	}
	var batch *timelock.BatchChainOperation
	if !useMCMS {
		_, err = chain.Confirm(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to confirm AddCapabilities confirm transaction %s: %w", tx.Hash().String(), err)
		}
		lggr.Info("registered capabilities", "capabilities", deduped)
	} else {
		batch = &timelock.BatchChainOperation{
			ChainIdentifier: mcms.ChainIdentifier(chain.Selector),
			Batch: []mcms.Operation{
				{
					To:    registry.Address(),
					Data:  tx.Data(),
					Value: big.NewInt(0),
				},
			},
		}
	}
	return batch, nil
}

// CapabilityID returns a unique id for the capability
// TODO: mv to chainlink-common? ref https://github.com/smartcontractkit/chainlink/blob/4fb06b4525f03c169c121a68defa9b13677f5f20/contracts/src/v0.8/keystone/CapabilitiesRegistry.sol#L170
func CapabilityID(c kcr.CapabilitiesRegistryCapability) string {
	return fmt.Sprintf("%s@%s", c.LabelledName, c.Version)
}

// dedupCapabilities deduplicates the capabilities
// dedup capabilities with respect to the registry
// contract reverts on adding the same capability twice and that would cause the whole transaction to revert
// which is very bad for us for mcms
func dedupCapabilities(registry *kcr.CapabilitiesRegistry, capabilities []kcr.CapabilitiesRegistryCapability) ([]kcr.CapabilitiesRegistryCapability, error) {
	var out []kcr.CapabilitiesRegistryCapability
	existing, err := registry.GetCapabilities(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call GetCapabilities: %w", err)
	}
	existingByID := make(map[[32]byte]struct{})
	for _, cap := range existing {
		existingByID[cap.HashedId] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, candidate := range capabilities {
		h, err := registry.GetHashedCapabilityId(nil, candidate.LabelledName, candidate.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to call GetHashedCapabilityId: %w", err)
		}
		// dedup input capabilities
		if _, exists := seen[CapabilityID(candidate)]; exists {
			continue
		}
		seen[CapabilityID(candidate)] = struct{}{}
		// dedup with respect to the registry
		if _, exists := existingByID[h]; !exists {
			out = append(out, candidate)
		}
	}
	return out, nil
}

package changeset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	commonchangeset "github.com/smartcontractkit/chainlink/deployment/common/changeset"
	"github.com/smartcontractkit/chainlink/deployment/common/proposalutils"
	"github.com/smartcontractkit/chainlink/deployment/keystone/changeset"
	kcr "github.com/smartcontractkit/chainlink/v2/core/gethwrappers/keystone/generated/capabilities_registry"
	"github.com/smartcontractkit/chainlink/v2/core/services/keystore/keys/p2pkey"
)

func TestAppendNodeCapabilities(t *testing.T) {
	t.Parallel()

	var (
		capA = kcr.CapabilitiesRegistryCapability{
			LabelledName: "capA",
			Version:      "0.4.2",
		}
		capB = kcr.CapabilitiesRegistryCapability{
			LabelledName: "capB",
			Version:      "3.16.0",
		}
		caps = []kcr.CapabilitiesRegistryCapability{capA, capB}
	)
	t.Run("no mcms", func(t *testing.T) {
		te := SetupTestEnv(t, TestConfig{
			WFDonConfig:     DonConfig{N: 4},
			AssetDonConfig:  DonConfig{N: 4},
			WriterDonConfig: DonConfig{N: 4},
			NumChains:       1,
		})

		newCapabilities := make(map[p2pkey.PeerID][]kcr.CapabilitiesRegistryCapability)
		for id, _ := range te.WFNodes {
			k, err := p2pkey.MakePeerID(id)
			require.NoError(t, err)
			newCapabilities[k] = caps
		}

		t.Run("succeeds if existing capabilities not explicit", func(t *testing.T) {
			cfg := changeset.AppendNodeCapabilitiesRequest{
				RegistryChainSel:  te.RegistrySelector,
				P2pToCapabilities: newCapabilities,
			}

			csOut, err := changeset.AppendNodeCapabilities(te.Env, &cfg)
			require.NoError(t, err)
			require.Len(t, csOut.Proposals, 0)
			require.Nil(t, csOut.AddressBook)

			validateCapabilityAppends(t, te, newCapabilities)
		})
	})
	t.Run("with mcms", func(t *testing.T) {
		te := SetupTestEnv(t, TestConfig{
			WFDonConfig:     DonConfig{N: 4},
			AssetDonConfig:  DonConfig{N: 4},
			WriterDonConfig: DonConfig{N: 4},
			NumChains:       1,
			UseMCMS:         true,
		})

		newCapabilities := make(map[p2pkey.PeerID][]kcr.CapabilitiesRegistryCapability)
		for id, _ := range te.WFNodes {
			k, err := p2pkey.MakePeerID(id)
			require.NoError(t, err)
			newCapabilities[k] = caps
		}

		cfg := changeset.AppendNodeCapabilitiesRequest{
			RegistryChainSel:  te.RegistrySelector,
			P2pToCapabilities: newCapabilities,
			MCMSConfig:        &changeset.MCMSConfig{MinDuration: 0},
		}

		csOut, err := changeset.AppendNodeCapabilities(te.Env, &cfg)
		require.NoError(t, err)
		require.Len(t, csOut.Proposals, 1)
		require.Len(t, csOut.Proposals[0].Transactions, 1)
		require.Len(t, csOut.Proposals[0].Transactions[0].Batch, 2) // add capabilities, update nodes
		require.Nil(t, csOut.AddressBook)

		// now apply the changeset such that the proposal is signed and execed
		contracts := te.ContractSets()[te.RegistrySelector]
		timelockContracts := map[uint64]*proposalutils.TimelockExecutionContracts{
			te.RegistrySelector: {
				Timelock:  contracts.Timelock,
				CallProxy: contracts.CallProxy,
			},
		}

		_, err = commonchangeset.ApplyChangesets(t, te.Env, timelockContracts, []commonchangeset.ChangesetApplication{
			{
				Changeset: commonchangeset.WrapChangeSet(changeset.AppendNodeCapabilities),
				Config:    &cfg,
			},
		})
		require.NoError(t, err)
		validateCapabilityAppends(t, te, newCapabilities)
	})

}

// validateUpdate checks reads nodes from the registry and checks they have the expected updates
func validateCapabilityAppends(t *testing.T, te TestEnv, appended map[p2pkey.PeerID][]kcr.CapabilitiesRegistryCapability) {
	registry := te.ContractSets()[te.RegistrySelector].CapabilitiesRegistry
	wfP2PIDs := p2pIDs(t, maps.Keys(te.WFNodes))
	nodes, err := registry.GetNodesByP2PIds(nil, wfP2PIDs)
	require.NoError(t, err)
	require.Len(t, nodes, len(wfP2PIDs))
	for _, node := range nodes {
		want := appended[node.P2pId]
		require.NotNil(t, want)
		assertContainsCapabilities(t, registry, want, node)
	}
}

func assertContainsCapabilities(t *testing.T, registry *kcr.CapabilitiesRegistry, want []kcr.CapabilitiesRegistryCapability, got kcr.INodeInfoProviderNodeInfo) {
	wantHashes := make([][32]byte, len(want))
	for i, c := range want {
		h, err := registry.GetHashedCapabilityId(nil, c.LabelledName, c.Version)
		require.NoError(t, err)
		wantHashes[i] = h
		assert.Contains(t, got.HashedCapabilityIds, h, "missing capability %v", c)
	}
	assert.LessOrEqual(t, len(want), len(got.HashedCapabilityIds))
}
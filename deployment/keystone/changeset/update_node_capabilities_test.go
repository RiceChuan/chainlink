package changeset_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink/deployment"
	kslib "github.com/smartcontractkit/chainlink/deployment/keystone"
	"github.com/smartcontractkit/chainlink/deployment/keystone/changeset"
	kstest "github.com/smartcontractkit/chainlink/deployment/keystone/test"
	kcr "github.com/smartcontractkit/chainlink/v2/core/gethwrappers/keystone/generated/capabilities_registry"
	"github.com/smartcontractkit/chainlink/v2/core/services/keystore/keys/p2pkey"
)

func TestUpdateNodeCapabilities(t *testing.T) {
	var (
		initialp2pToCapabilities = map[p2pkey.PeerID][]kcr.CapabilitiesRegistryCapability{
			testPeerID(t, "0x1"): []kcr.CapabilitiesRegistryCapability{
				{
					LabelledName:   "test",
					Version:        "1.0.0",
					CapabilityType: 0,
				},
			},
		}
		nopToNodes = map[kcr.CapabilitiesRegistryNodeOperator][]*kslib.P2PSignerEnc{
			testNop(t, "testNop"): []*kslib.P2PSignerEnc{
				&kslib.P2PSignerEnc{
					Signer:              [32]byte{0: 1},
					P2PKey:              testPeerID(t, "0x1"),
					EncryptionPublicKey: [32]byte{3: 16, 4: 2},
				},
			},
		}
	)

	lggr := logger.Test(t)

	type args struct {
		lggr         logger.Logger
		req          *changeset.UpdateNodeCapabilitiesRequest
		initialState *kstest.SetupTestRegistryRequest
	}
	tests := []struct {
		name    string
		args    args
		want    deployment.ChangesetOutput
		wantErr bool
	}{
		{
			name: "invalid request",
			args: args{
				lggr: lggr,
				req: &changeset.UpdateNodeCapabilitiesRequest{
					Chain: deployment.Chain{},
				},
				initialState: &kstest.SetupTestRegistryRequest{},
			},
			wantErr: true,
		},
		{
			name: "happy path",
			args: args{
				lggr: lggr,
				initialState: &kstest.SetupTestRegistryRequest{
					P2pToCapabilities: initialp2pToCapabilities,
					NopToNodes:        nopToNodes,
				},
				req: &changeset.UpdateNodeCapabilitiesRequest{
					P2pToCapabilities: map[p2pkey.PeerID][]kcr.CapabilitiesRegistryCapability{
						testPeerID(t, "0x1"): []kcr.CapabilitiesRegistryCapability{
							{
								LabelledName:   "cap2",
								Version:        "1.0.0",
								CapabilityType: 0,
							},
							{
								LabelledName:   "cap3",
								Version:        "1.0.0",
								CapabilityType: 3,
							},
						},
					},
					NopToNodes: nopToNodes,
				},
			},
			want:    deployment.ChangesetOutput{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// chagen the name and args to be mor egeneral
			setupResp := kstest.SetupTestRegistry(t, lggr, tt.args.initialState)

			tt.args.req.Registry = setupResp.Registry
			tt.args.req.Chain = setupResp.Chain

			got, err := changeset.UpdateNodeCapabilitiesImpl(tt.args.lggr, tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateNodeCapabilities() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			require.NotNil(t, got)
			// update is a set, so there should be 2 capabilities on the node
			assert.Len(t, got.NodeParams, len(tt.args.req.P2pToCapabilities))
			for _, nodeParam := range got.NodeParams {
				setCaps := tt.args.req.P2pToCapabilities[nodeParam.P2pId]
				assert.Len(t, nodeParam.HashedCapabilityIds, len(setCaps))
			}
		})
	}
}
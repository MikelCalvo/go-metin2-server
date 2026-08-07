package minimal

import (
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
)

func TestGameRuntimeCombatIngressGuardsAreNoOp(t *testing.T) {
	ticketStore := loginticket.NewFileStore(t.TempDir())
	accounts := accountstore.NewFileStore(t.TempDir())
	owner := peerVisibilityCharacter("CombatGuard", 0x01030840, 0x02040840, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, ticketStore, "combat-guard-owner", 0x80808040, owner)
	if err := accounts.Save(accountstore.Account{Login: "combat-guard-owner", Empire: owner.Empire, Characters: cloneCharacters([]loginticket.Character{owner})}); err != nil {
		t.Fatalf("seed combat ingress guard account: %v", err)
	}
	runtime, err := newGameRuntimeWithStoresAndTransferTriggers(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, ticketStore, accounts, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected combat ingress guard runtime error: %v", err)
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "combat-guard-owner", 0x80808040)
	defer closeSessionFlow(t, flow)

	for _, tc := range []struct {
		name string
		raw  []byte
	}{
		{name: "fly-targeting", raw: combatproto.EncodeClientFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: 0x02040107, X: 123456, Y: -234567})},
		{name: "add-fly-targeting", raw: combatproto.EncodeClientAddFlyTargeting(combatproto.ClientFlyTargetingPacket{TargetVID: 0, X: 1700, Y: -2800})},
		{name: "on-click", raw: combatproto.EncodeClientOnClick(combatproto.ClientOnClickPacket{VID: 0x02040107})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := flow.HandleClientFrame(decodeSingleFrame(t, tc.raw))
			if err != nil {
				t.Fatalf("unexpected %s packet error: %v", tc.name, err)
			}
			if len(out) != 0 {
				t.Fatalf("expected %s guard to emit no frames, got %d", tc.name, len(out))
			}
			if queued := flushServerFrames(t, flow); len(queued) != 0 {
				t.Fatalf("expected no queued frames after %s guard, got %d", tc.name, len(queued))
			}
		})
	}
	persisted, err := accounts.Load("combat-guard-owner")
	if err != nil {
		t.Fatalf("load persisted combat ingress guard account: %v", err)
	}
	if persisted.Characters[0].X != owner.X || persisted.Characters[0].Y != owner.Y || persisted.Characters[0].Points[bootstrapPlayerPointValueIndex] != owner.Points[bootstrapPlayerPointValueIndex] {
		t.Fatalf("combat ingress guards mutated persisted selected character: got %+v want position=(%d,%d) hp=%d", persisted.Characters[0], owner.X, owner.Y, owner.Points[bootstrapPlayerPointValueIndex])
	}
}

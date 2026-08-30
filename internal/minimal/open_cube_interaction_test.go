package minimal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/cubestore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
)

func TestGameSessionFlowStaticActorOpenCubeInteractionEmitsCubeOpenCommand(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenCube,
		Ref:  "npc:qa_cube",
		Text: "The craftsman lights the forge.",
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("CubeMaster", bootstrapMapIndex, 1200, 2200, cubestore.BootstrapDefaultNPCVnum, interactionstore.KindOpenCube, "npc:qa_cube")
	if !ok {
		t.Fatal("expected open_cube static actor registration to succeed")
	}
	flow, enterOut := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	if len(enterOut) < 8 {
		t.Fatalf("expected bootstrap frames with visible cube actor, got %d", len(enterOut))
	}
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected open_cube interaction error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected chat + cube open frames, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode open_cube chat delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "The craftsman lights the forge." {
		t.Fatalf("unexpected open_cube chat delivery: %+v", delivery)
	}
	assertCubeCommandChatFrame(t, out[1], "cube open 20022", "open_cube interact")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for open_cube, got %d", len(queued))
	}

	subject, ok := runtime.sharedWorld.entities.PlayerByName(peer.Name)
	if !ok {
		t.Fatalf("expected live shared-world entity for %q after enter", peer.Name)
	}
	resolution := runtime.resolveStaticActorInteraction(subject.Entity.ID, uint32(actor.EntityID))
	if !resolution.Accepted || resolution.Failure != "" || resolution.Definition.Kind != interactionstore.KindOpenCube {
		t.Fatalf("unexpected open_cube resolution: %+v", resolution)
	}
}

func TestGameSessionFlowStaticActorQuestGatedOpenCubeRejectsWhenRequirementMissing(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	questStatePath := filepath.Join(t.TempDir(), "quest-state.json")
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind:      interactionstore.KindOpenCube,
		Ref:       "npc:gated_cube",
		Text:      "The craftsman lights the forge.",
		QuestRef:  "quest:first_steps",
		QuestFlag: "met_guide",
		QuestFrom: 1,
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1", QuestStateStorePath: questStatePath}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	actor, ok := runtime.RegisterStaticActorWithInteraction("CubeMaster", bootstrapMapIndex, 1200, 2200, cubestore.BootstrapDefaultNPCVnum, interactionstore.KindOpenCube, "npc:gated_cube")
	if !ok {
		t.Fatal("expected gated open_cube static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected gated open_cube mismatch interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 self-only gated open_cube mismatch frame, got %d", len(out))
	}
	delivery, err := chatproto.DecodeChatDelivery(decodeSingleFrame(t, out[0]))
	if err != nil {
		t.Fatalf("decode gated open_cube mismatch delivery: %v", err)
	}
	if delivery.Type != chatproto.ChatTypeInfo || delivery.VID != 0 || delivery.Empire != 0 || delivery.Message != "Quest requirements are not met." {
		t.Fatalf("unexpected gated open_cube mismatch delivery: %+v", delivery)
	}
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for gated open_cube mismatch, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorOpenCubeAlreadyOpenEmitsInfoWithoutSecondOpen(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	peer := peerVisibilityCharacter("PeerOne", 0x01030101, 0x02040101, 1100, 2100, 0, 101, 201)
	issuePeerTicket(t, store, "peer-one", 0x11111111, peer)
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{{
		Kind: interactionstore.KindOpenCube,
		Ref:  "npc:qa_cube",
		Text: "The craftsman lights the forge.",
	}})

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, nil, interactionStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000000, 0)
	runtime.now = func() time.Time { return currentTime }
	actor, ok := runtime.RegisterStaticActorWithInteraction("CubeMaster", bootstrapMapIndex, 1200, 2200, cubestore.BootstrapDefaultNPCVnum, interactionstore.KindOpenCube, "npc:qa_cube")
	if !ok {
		t.Fatal("expected open_cube static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "peer-one", 0x11111111)
	defer closeSessionFlow(t, flow)

	firstOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected first open_cube interaction error: %v", err)
	}
	if len(firstOut) != 2 {
		t.Fatalf("expected chat + cube open frames on first interact, got %d", len(firstOut))
	}
	assertCubeCommandChatFrame(t, firstOut[1], "cube open 20022", "first open_cube interact")

	currentTime = currentTime.Add(staticActorInteractionCooldown)

	secondOut, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected second open_cube interaction error: %v", err)
	}
	if len(secondOut) != 1 {
		t.Fatalf("expected already-open info frame only, got %d", len(secondOut))
	}
	assertCubeInfoChatFrame(t, secondOut[0], cubeAlreadyOpenInfoMessage, "already-open open_cube interact")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for already-open open_cube, got %d", len(queued))
	}
}

func TestGameSessionFlowStaticActorOpenCubeRejectsBusyShellWithoutMutation(t *testing.T) {
	store := loginticket.NewFileStore(t.TempDir())
	buyer := merchantBuyerCharacter("CubeBusyBuyer", 0x01040140, 0x02050140, 125, nil)
	issuePeerTicket(t, store, "cube-busy-buyer", 0x40404040, buyer)
	accounts := accountstore.NewFileStore(t.TempDir())
	if err := accounts.Save(accountstore.Account{Login: "cube-busy-buyer", Empire: buyer.Empire, Characters: cloneCharacters([]loginticket.Character{buyer})}); err != nil {
		t.Fatalf("seed cube busy-shell buyer account: %v", err)
	}
	interactionStore := newInteractionDefinitionStore(t, []interactionstore.Definition{
		defaultMerchantCatalogDefinition(),
		{Kind: interactionstore.KindOpenCube, Ref: "npc:qa_cube", Text: "The craftsman lights the forge."},
	})
	itemStore := newItemTemplateStore(t, defaultMerchantItemTemplates())

	runtime, err := newGameRuntimeWithAccountStoreAndInteractionAndItemStore(config.Service{LegacyAddr: ":13000", PublicAddr: "127.0.0.1"}, store, accounts, interactionStore, itemStore)
	if err != nil {
		t.Fatalf("unexpected game runtime error: %v", err)
	}
	currentTime := time.Unix(1700000000, 0)
	runtime.now = func() time.Time { return currentTime }
	merchant, ok := runtime.RegisterStaticActorWithInteraction("Merchant", bootstrapMapIndex, 1200, 2200, 20301, interactionstore.KindShopPreview, "npc:merchant")
	if !ok {
		t.Fatal("expected merchant registration to succeed")
	}
	cube, ok := runtime.RegisterStaticActorWithInteraction("CubeMaster", bootstrapMapIndex, 1250, 2200, cubestore.BootstrapDefaultNPCVnum, interactionstore.KindOpenCube, "npc:qa_cube")
	if !ok {
		t.Fatal("expected open_cube static actor registration to succeed")
	}
	flow, _ := enterGameWithLoginTicket(t, runtime.SessionFactory(), "cube-busy-buyer", 0x40404040)
	defer closeSessionFlow(t, flow)

	interactWithMerchantForBuy(t, flow, merchant.EntityID)
	currentTime = currentTime.Add(staticActorInteractionCooldown)

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, interactproto.EncodeRequest(interactproto.RequestPacket{TargetVID: uint32(cube.EntityID)})))
	if err != nil {
		t.Fatalf("unexpected busy-shell open_cube interaction error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one busy-shell info frame, got %d", len(out))
	}
	assertCubeInfoChatFrame(t, out[0], cubeBusyShellInfoMessage, "busy-shell open_cube interact")
	if queued := flushServerFrames(t, flow); len(queued) != 0 {
		t.Fatalf("expected no queued peer frames for busy-shell open_cube, got %d", len(queued))
	}
}

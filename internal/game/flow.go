package game

import (
	"errors"

	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	itemproto "github.com/MikelCalvo/go-metin2-server/internal/proto/item"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
	shopproto "github.com/MikelCalvo/go-metin2-server/internal/proto/shop"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

var (
	ErrInvalidPhase           = errors.New("game flow is not in game phase")
	ErrUnexpectedClientPacket = errors.New("unexpected client packet during game")
)

type HandleMoveFunc func(movep.MovePacket) Result

type HandleSyncPositionFunc func(movep.SyncPositionPacket) SyncPositionResult

type HandleChatFunc func(chatproto.ClientChatPacket) ChatResult

type HandleWhisperFunc func(chatproto.ClientWhisperPacket) WhisperResult

type HandleInteractionFunc func(interactproto.RequestPacket) InteractionResult

type HandleTargetFunc func(combatproto.ClientTargetPacket) TargetResult

type HandleAttackFunc func(combatproto.ClientAttackPacket) AttackResult

type HandleItemUseFunc func(itemproto.ClientUsePacket) ItemUseResult

type HandleItemDropFunc func(itemproto.ClientDropPacket) ItemDropResult

type HandleItemDrop2Func func(itemproto.ClientDrop2Packet) ItemDrop2Result

type HandleItemMoveFunc func(itemproto.ClientMovePacket) ItemMoveResult

type HandleItemPickupFunc func(itemproto.ClientPickupPacket) ItemPickupResult

type HandleQuickslotAddFunc func(quickslotproto.ClientAddPacket) QuickslotResult

type HandleQuickslotDelFunc func(quickslotproto.ClientDelPacket) QuickslotResult

type HandleQuickslotSwapFunc func(quickslotproto.ClientSwapPacket) QuickslotResult

type HandleShopBuyFunc func(shopproto.ClientBuyPacket) ShopResult

type HandleShopCloseFunc func() ShopResult

type HandleShopSellFunc func(shopproto.ClientSellPacket) ShopResult

type HandleShopSell2Func func(shopproto.ClientSell2Packet) ShopResult

type Config struct {
	HandleMove          HandleMoveFunc
	HandleSyncPosition  HandleSyncPositionFunc
	HandleChat          HandleChatFunc
	HandleWhisper       HandleWhisperFunc
	HandleInteraction   HandleInteractionFunc
	HandleTarget        HandleTargetFunc
	HandleAttack        HandleAttackFunc
	HandleItemUse       HandleItemUseFunc
	HandleItemDrop      HandleItemDropFunc
	HandleItemDrop2     HandleItemDrop2Func
	HandleItemMove      HandleItemMoveFunc
	HandleItemPickup    HandleItemPickupFunc
	HandleQuickslotAdd  HandleQuickslotAddFunc
	HandleQuickslotDel  HandleQuickslotDelFunc
	HandleQuickslotSwap HandleQuickslotSwapFunc
	HandleShopBuy       HandleShopBuyFunc
	HandleShopClose     HandleShopCloseFunc
	HandleShopSell      HandleShopSellFunc
	HandleShopSell2     HandleShopSell2Func
}

type Result struct {
	Accepted    bool
	Replication movep.MoveAckPacket
	Frames      [][]byte
}

type SyncPositionResult struct {
	Accepted        bool
	Synchronization movep.SyncPositionAckPacket
	Frames          [][]byte
}

type ChatResult struct {
	Accepted  bool
	Delivery  *chatproto.ChatDeliveryPacket
	Frames    [][]byte
	NextPhase session.Phase
}

type WhisperResult struct {
	Accepted bool
	Delivery *chatproto.ServerWhisperPacket
}

type InteractionResult struct {
	Accepted bool
	Frames   [][]byte
}

type TargetResult struct {
	Accepted bool
	Frames   [][]byte
}

type AttackResult struct {
	Accepted bool
	Frames   [][]byte
}

type ItemUseResult struct {
	Accepted bool
	Frames   [][]byte
}

type ItemDropResult struct {
	Accepted bool
	Frames   [][]byte
}

type ItemDrop2Result struct {
	Accepted bool
	Frames   [][]byte
}

type ItemMoveResult struct {
	Accepted bool
	Frames   [][]byte
}

type ItemPickupResult struct {
	Accepted bool
	Frames   [][]byte
}

type QuickslotResult struct {
	Accepted bool
	Frames   [][]byte
}

type ShopResult struct {
	Accepted bool
	Frames   [][]byte
}

type Flow struct {
	machine             *session.StateMachine
	handleMove          HandleMoveFunc
	handleSyncPosition  HandleSyncPositionFunc
	handleChat          HandleChatFunc
	handleWhisper       HandleWhisperFunc
	handleInteraction   HandleInteractionFunc
	handleTarget        HandleTargetFunc
	handleAttack        HandleAttackFunc
	handleItemUse       HandleItemUseFunc
	handleItemDrop      HandleItemDropFunc
	handleItemDrop2     HandleItemDrop2Func
	handleItemMove      HandleItemMoveFunc
	handleItemPickup    HandleItemPickupFunc
	handleQuickslotAdd  HandleQuickslotAddFunc
	handleQuickslotDel  HandleQuickslotDelFunc
	handleQuickslotSwap HandleQuickslotSwapFunc
	handleShopBuy       HandleShopBuyFunc
	handleShopClose     HandleShopCloseFunc
	handleShopSell      HandleShopSellFunc
	handleShopSell2     HandleShopSell2Func
}

func NewFlow(machine *session.StateMachine, cfg Config) *Flow {
	handler := cfg.HandleMove
	if handler == nil {
		handler = func(movep.MovePacket) Result { return Result{Accepted: false} }
	}
	syncHandler := cfg.HandleSyncPosition
	if syncHandler == nil {
		syncHandler = func(movep.SyncPositionPacket) SyncPositionResult { return SyncPositionResult{Accepted: false} }
	}
	chatHandler := cfg.HandleChat
	if chatHandler == nil {
		chatHandler = func(chatproto.ClientChatPacket) ChatResult { return ChatResult{Accepted: false} }
	}
	whisperHandler := cfg.HandleWhisper
	if whisperHandler == nil {
		whisperHandler = func(chatproto.ClientWhisperPacket) WhisperResult { return WhisperResult{Accepted: false} }
	}
	interactionHandler := cfg.HandleInteraction
	if interactionHandler == nil {
		interactionHandler = func(interactproto.RequestPacket) InteractionResult { return InteractionResult{Accepted: false} }
	}
	targetHandler := cfg.HandleTarget
	if targetHandler == nil {
		targetHandler = func(combatproto.ClientTargetPacket) TargetResult { return TargetResult{Accepted: false} }
	}
	attackHandler := cfg.HandleAttack
	if attackHandler == nil {
		attackHandler = func(combatproto.ClientAttackPacket) AttackResult { return AttackResult{Accepted: false} }
	}
	itemUseHandler := cfg.HandleItemUse
	if itemUseHandler == nil {
		itemUseHandler = func(itemproto.ClientUsePacket) ItemUseResult { return ItemUseResult{Accepted: false} }
	}
	itemDropHandler := cfg.HandleItemDrop
	if itemDropHandler == nil {
		itemDropHandler = func(itemproto.ClientDropPacket) ItemDropResult { return ItemDropResult{Accepted: false} }
	}
	itemDrop2Handler := cfg.HandleItemDrop2
	if itemDrop2Handler == nil {
		itemDrop2Handler = func(itemproto.ClientDrop2Packet) ItemDrop2Result { return ItemDrop2Result{Accepted: false} }
	}
	itemMoveHandler := cfg.HandleItemMove
	if itemMoveHandler == nil {
		itemMoveHandler = func(itemproto.ClientMovePacket) ItemMoveResult { return ItemMoveResult{Accepted: false} }
	}
	itemPickupHandler := cfg.HandleItemPickup
	if itemPickupHandler == nil {
		itemPickupHandler = func(itemproto.ClientPickupPacket) ItemPickupResult { return ItemPickupResult{Accepted: false} }
	}
	quickslotAddHandler := cfg.HandleQuickslotAdd
	if quickslotAddHandler == nil {
		quickslotAddHandler = func(quickslotproto.ClientAddPacket) QuickslotResult { return QuickslotResult{Accepted: false} }
	}
	quickslotDelHandler := cfg.HandleQuickslotDel
	if quickslotDelHandler == nil {
		quickslotDelHandler = func(quickslotproto.ClientDelPacket) QuickslotResult { return QuickslotResult{Accepted: false} }
	}
	quickslotSwapHandler := cfg.HandleQuickslotSwap
	if quickslotSwapHandler == nil {
		quickslotSwapHandler = func(quickslotproto.ClientSwapPacket) QuickslotResult { return QuickslotResult{Accepted: false} }
	}
	shopBuyHandler := cfg.HandleShopBuy
	if shopBuyHandler == nil {
		shopBuyHandler = func(shopproto.ClientBuyPacket) ShopResult { return ShopResult{Accepted: false} }
	}
	shopCloseHandler := cfg.HandleShopClose
	if shopCloseHandler == nil {
		shopCloseHandler = func() ShopResult { return ShopResult{Accepted: false} }
	}
	shopSellHandler := cfg.HandleShopSell
	if shopSellHandler == nil {
		shopSellHandler = func(shopproto.ClientSellPacket) ShopResult { return ShopResult{Accepted: false} }
	}
	shopSell2Handler := cfg.HandleShopSell2
	if shopSell2Handler == nil {
		shopSell2Handler = func(shopproto.ClientSell2Packet) ShopResult { return ShopResult{Accepted: false} }
	}
	return &Flow{
		machine:             machine,
		handleMove:          handler,
		handleSyncPosition:  syncHandler,
		handleChat:          chatHandler,
		handleWhisper:       whisperHandler,
		handleInteraction:   interactionHandler,
		handleTarget:        targetHandler,
		handleAttack:        attackHandler,
		handleItemUse:       itemUseHandler,
		handleItemDrop:      itemDropHandler,
		handleItemDrop2:     itemDrop2Handler,
		handleItemMove:      itemMoveHandler,
		handleItemPickup:    itemPickupHandler,
		handleQuickslotAdd:  quickslotAddHandler,
		handleQuickslotDel:  quickslotDelHandler,
		handleQuickslotSwap: quickslotSwapHandler,
		handleShopBuy:       shopBuyHandler,
		handleShopClose:     shopCloseHandler,
		handleShopSell:      shopSellHandler,
		handleShopSell2:     shopSell2Handler,
	}
}

func (f *Flow) HandleClientFrame(in frame.Frame) ([][]byte, error) {
	if f.machine.Current() != session.PhaseGame {
		return nil, ErrInvalidPhase
	}
	switch in.Header {
	case control.HeaderPong:
		if _, err := control.DecodePong(in); err != nil {
			return nil, err
		}
		return nil, nil
	case movep.HeaderMove:
		packet, err := movep.DecodeMove(in)
		if err != nil {
			return nil, err
		}
		result := f.handleMove(packet)
		if !result.Accepted {
			return nil, nil
		}
		if result.Frames != nil {
			return result.Frames, nil
		}
		return [][]byte{movep.EncodeMoveAck(result.Replication)}, nil
	case movep.HeaderSyncPosition:
		packet, err := movep.DecodeSyncPosition(in)
		if err != nil {
			return nil, err
		}
		result := f.handleSyncPosition(packet)
		if !result.Accepted {
			return nil, nil
		}
		if result.Frames != nil {
			return result.Frames, nil
		}
		return [][]byte{movep.EncodeSyncPositionAck(result.Synchronization)}, nil
	case chatproto.HeaderClientChat:
		packet, err := chatproto.DecodeClientChat(in)
		if err != nil {
			return nil, err
		}
		result := f.handleChat(packet)
		if !result.Accepted {
			return nil, nil
		}
		out := make([][]byte, 0, len(result.Frames)+1)
		out = append(out, result.Frames...)
		if result.NextPhase != "" {
			phaseRaw, err := control.EncodePhase(result.NextPhase)
			if err != nil {
				return nil, err
			}
			if err := f.machine.Transition(result.NextPhase); err != nil {
				return nil, err
			}
			out = append(out, phaseRaw)
		}
		if result.Delivery != nil {
			out = append(out, chatproto.EncodeChatDelivery(*result.Delivery))
		}
		return out, nil
	case chatproto.HeaderClientWhisper:
		packet, err := chatproto.DecodeClientWhisper(in)
		if err != nil {
			return nil, err
		}
		result := f.handleWhisper(packet)
		if !result.Accepted {
			return nil, nil
		}
		if result.Delivery == nil {
			return nil, nil
		}
		return [][]byte{chatproto.EncodeServerWhisper(*result.Delivery)}, nil
	case interactproto.HeaderRequest:
		packet, err := interactproto.DecodeRequest(in)
		if err != nil {
			return nil, err
		}
		result := f.handleInteraction(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case combatproto.HeaderClientTarget:
		packet, err := combatproto.DecodeClientTarget(in)
		if err != nil {
			return nil, err
		}
		result := f.handleTarget(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case combatproto.HeaderClientAttack:
		packet, err := combatproto.DecodeClientAttack(in)
		if err != nil {
			return nil, err
		}
		result := f.handleAttack(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case itemproto.HeaderClientUse:
		if len(in.Payload) == 3 {
			packet, err := itemproto.DecodeClientUse(in)
			if err != nil {
				return nil, err
			}
			result := f.handleItemUse(packet)
			if !result.Accepted {
				return nil, nil
			}
			return result.Frames, nil
		}
		packet, err := itemproto.DecodeClientDrop(in)
		if err != nil {
			return nil, err
		}
		result := f.handleItemDrop(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case itemproto.HeaderClientDrop2:
		packet, err := itemproto.DecodeClientDrop2(in)
		if err != nil {
			return nil, err
		}
		result := f.handleItemDrop2(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case itemproto.HeaderClientMove:
		packet, err := itemproto.DecodeClientMove(in)
		if err != nil {
			return nil, err
		}
		result := f.handleItemMove(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case itemproto.HeaderClientPickup:
		packet, err := itemproto.DecodeClientPickup(in)
		if err != nil {
			return nil, err
		}
		result := f.handleItemPickup(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case quickslotproto.HeaderClientAdd:
		packet, err := quickslotproto.DecodeClientAdd(in)
		if err != nil {
			return nil, err
		}
		result := f.handleQuickslotAdd(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case quickslotproto.HeaderClientDel:
		packet, err := quickslotproto.DecodeClientDel(in)
		if err != nil {
			return nil, err
		}
		result := f.handleQuickslotDel(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case quickslotproto.HeaderClientSwap:
		packet, err := quickslotproto.DecodeClientSwap(in)
		if err != nil {
			return nil, err
		}
		result := f.handleQuickslotSwap(packet)
		if !result.Accepted {
			return nil, nil
		}
		return result.Frames, nil
	case shopproto.HeaderClientShop:
		if len(in.Payload) == 0 {
			return nil, shopproto.ErrInvalidPayload
		}
		switch in.Payload[0] {
		case shopproto.ClientSubheaderEnd:
			if err := shopproto.DecodeClientEnd(in); err != nil {
				return nil, err
			}
			result := f.handleShopClose()
			if !result.Accepted {
				return nil, nil
			}
			return result.Frames, nil
		case shopproto.ClientSubheaderBuy:
			packet, err := shopproto.DecodeClientBuy(in)
			if err != nil {
				return nil, err
			}
			result := f.handleShopBuy(packet)
			if !result.Accepted {
				return nil, nil
			}
			return result.Frames, nil
		case shopproto.ClientSubheaderSell:
			packet, err := shopproto.DecodeClientSell(in)
			if err != nil {
				return nil, err
			}
			result := f.handleShopSell(packet)
			if !result.Accepted {
				return nil, nil
			}
			return result.Frames, nil
		case shopproto.ClientSubheaderSell2:
			packet, err := shopproto.DecodeClientSell2(in)
			if err != nil {
				return nil, err
			}
			result := f.handleShopSell2(packet)
			if !result.Accepted {
				return nil, nil
			}
			return result.Frames, nil
		default:
			return nil, shopproto.ErrUnexpectedSubheader
		}
	default:
		return nil, ErrUnexpectedClientPacket
	}
}

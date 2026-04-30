package game

import (
	"errors"

	chatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/chat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	interactproto "github.com/MikelCalvo/go-metin2-server/internal/proto/interact"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
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

type HandleShopBuyFunc func(shopproto.ClientBuyPacket) ShopResult

type HandleShopCloseFunc func() ShopResult

type Config struct {
	HandleMove         HandleMoveFunc
	HandleSyncPosition HandleSyncPositionFunc
	HandleChat         HandleChatFunc
	HandleWhisper      HandleWhisperFunc
	HandleInteraction  HandleInteractionFunc
	HandleShopBuy      HandleShopBuyFunc
	HandleShopClose    HandleShopCloseFunc
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

type ShopResult struct {
	Accepted bool
	Frames   [][]byte
}

type Flow struct {
	machine            *session.StateMachine
	handleMove         HandleMoveFunc
	handleSyncPosition HandleSyncPositionFunc
	handleChat         HandleChatFunc
	handleWhisper      HandleWhisperFunc
	handleInteraction  HandleInteractionFunc
	handleShopBuy      HandleShopBuyFunc
	handleShopClose    HandleShopCloseFunc
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
	shopBuyHandler := cfg.HandleShopBuy
	if shopBuyHandler == nil {
		shopBuyHandler = func(shopproto.ClientBuyPacket) ShopResult { return ShopResult{Accepted: false} }
	}
	shopCloseHandler := cfg.HandleShopClose
	if shopCloseHandler == nil {
		shopCloseHandler = func() ShopResult { return ShopResult{Accepted: false} }
	}
	return &Flow{
		machine:            machine,
		handleMove:         handler,
		handleSyncPosition: syncHandler,
		handleChat:         chatHandler,
		handleWhisper:      whisperHandler,
		handleInteraction:  interactionHandler,
		handleShopBuy:      shopBuyHandler,
		handleShopClose:    shopCloseHandler,
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
		out := make([][]byte, 0, 1+len(result.Frames))
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
		out = append(out, result.Frames...)
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
		default:
			return nil, shopproto.ErrUnexpectedSubheader
		}
	default:
		return nil, ErrUnexpectedClientPacket
	}
}

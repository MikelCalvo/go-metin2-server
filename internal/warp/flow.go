package warp

import (
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
)

type Target struct {
	MapIndex uint32
	X        int32
	Y        int32
}

type Result struct {
	Applied bool
	Updated loginticket.Character
	// SelfFrames are queued only after persist and commit both succeed.
	// A rejected transfer must not advertise a self-visible warp.
	SelfFrames [][]byte
}

type Config struct {
	Persist  func(updated loginticket.Character) bool
	Rollback func(previous loginticket.Character) bool
	Commit   func(updated loginticket.Character) (Result, bool)
	// Endpoint is the advertised login address and port of this same process.
	// When set, a successful commit also carries one self-only GC::WARP for the
	// committed destination. A zero port leaves that companion off.
	Endpoint *worldproto.WarpPacket
}

type Flow struct {
	persist  func(updated loginticket.Character) bool
	rollback func(previous loginticket.Character) bool
	commit   func(updated loginticket.Character) (Result, bool)
	endpoint *worldproto.WarpPacket
}

func NewFlow(cfg Config) Flow {
	return Flow{persist: cfg.Persist, rollback: cfg.Rollback, commit: cfg.Commit, endpoint: cfg.Endpoint}
}

func (f Flow) Apply(selected loginticket.Character, target Target) (Result, bool) {
	if selected.ID == 0 || target.MapIndex == 0 || f.commit == nil {
		return Result{}, false
	}

	updated := selected
	updated.MapIndex = target.MapIndex
	updated.X = target.X
	updated.Y = target.Y

	if f.persist != nil && !f.persist(updated) {
		return Result{}, false
	}

	result, ok := f.commit(updated)
	if !ok {
		if f.rollback != nil {
			_ = f.rollback(selected)
		}
		return Result{}, false
	}
	if f.endpoint != nil && f.endpoint.Port != 0 {
		warpFrame := worldproto.EncodeWarp(worldproto.WarpPacket{
			X:    updated.X,
			Y:    updated.Y,
			Addr: f.endpoint.Addr,
			Port: f.endpoint.Port,
		})
		result.SelfFrames = append(append([][]byte(nil), result.SelfFrames...), warpFrame)
	}
	return result, true
}

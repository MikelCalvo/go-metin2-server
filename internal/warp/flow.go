package warp

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

type Target struct {
	MapIndex uint32
	X        int32
	Y        int32
}

type Result struct {
	Applied bool
	Updated loginticket.Character
}

type Config struct {
	Persist  func(updated loginticket.Character) bool
	Rollback func(previous loginticket.Character) bool
	Commit   func(updated loginticket.Character) (Result, bool)
}

type Flow struct {
	persist  func(updated loginticket.Character) bool
	rollback func(previous loginticket.Character) bool
	commit   func(updated loginticket.Character) (Result, bool)
}

func NewFlow(cfg Config) Flow {
	return Flow{persist: cfg.Persist, rollback: cfg.Rollback, commit: cfg.Commit}
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
	if ok {
		return result, true
	}
	if f.rollback != nil {
		_ = f.rollback(selected)
	}
	return Result{}, false
}

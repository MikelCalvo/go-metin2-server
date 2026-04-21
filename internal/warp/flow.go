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
	Commit func(updated loginticket.Character) (Result, bool)
}

type Flow struct {
	commit func(updated loginticket.Character) (Result, bool)
}

func NewFlow(cfg Config) Flow {
	return Flow{commit: cfg.Commit}
}

func (f Flow) Apply(selected loginticket.Character, target Target) (Result, bool) {
	if selected.ID == 0 || target.MapIndex == 0 || f.commit == nil {
		return Result{}, false
	}

	updated := selected
	updated.MapIndex = target.MapIndex
	updated.X = target.X
	updated.Y = target.Y

	return f.commit(updated)
}

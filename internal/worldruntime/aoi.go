package worldruntime

import "github.com/MikelCalvo/go-metin2-server/internal/loginticket"

type SectorKey struct {
	MapIndex uint32
	SX       int32
	SY       int32
}

type RadiusVisibilityPolicy struct {
	Radius     int32
	SectorSize int32
}

func SectorKeyForPosition(position Position, sectorSize int32) SectorKey {
	if !position.Valid() {
		return SectorKey{}
	}
	if sectorSize <= 0 {
		sectorSize = 1
	}
	return SectorKey{
		MapIndex: position.MapIndex,
		SX:       position.X / sectorSize,
		SY:       position.Y / sectorSize,
	}
}

func (p RadiusVisibilityPolicy) CanSee(topology BootstrapTopology, subject loginticket.Character, peer loginticket.Character) bool {
	if topology.EffectiveChannelID(subject) != topology.EffectiveChannelID(peer) {
		return false
	}

	subjectPosition := NewPosition(topology.EffectiveMapIndex(subject), subject.X, subject.Y)
	peerPosition := NewPosition(topology.EffectiveMapIndex(peer), peer.X, peer.Y)
	if !subjectPosition.SameMap(peerPosition) {
		return false
	}
	if p.Radius <= 0 {
		return false
	}

	dx := int64(subjectPosition.X) - int64(peerPosition.X)
	dy := int64(subjectPosition.Y) - int64(peerPosition.Y)
	radiusSquared := int64(p.Radius) * int64(p.Radius)
	return dx*dx+dy*dy <= radiusSquared
}

package minimal

import (
	"encoding/binary"
	"errors"
	"math"
	"net"
	"strconv"

	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/authboot"
	"github.com/MikelCalvo/go-metin2-server/internal/boot"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	worldentry "github.com/MikelCalvo/go-metin2-server/internal/worldentry"
)

const (
	StubLogin    = "mkmk"
	StubPassword = "hunter2"
	StubLoginKey = 0x01020304
)

var (
	ErrInvalidLegacyAddr = errors.New("invalid legacy addr")
	ErrInvalidPublicAddr = errors.New("invalid public addr")
)

func NewAuthSessionFactory() service.SessionFactory {
	return func() service.SessionFlow {
		return authboot.NewFlow(authboot.Config{
			Handshake: handshake.Config{
				KeyChallenge: defaultKeyChallenge(),
				KeyComplete:  defaultKeyComplete(),
			},
			Auth: authflow.Config{
				Authenticate: func(packet authproto.Login3Packet) authflow.Result {
					if packet.Login == StubLogin && packet.Password == StubPassword {
						return authflow.Result{Accepted: true, LoginKey: StubLoginKey}
					}
					return authflow.Result{Accepted: false, FailureStatus: "WRONGPWD"}
				},
			},
		})
	}
}

func NewGameSessionFactory(cfg config.Service) (service.SessionFactory, error) {
	advertisedPort, err := parsePort(cfg.LegacyAddr)
	if err != nil {
		return nil, err
	}

	advertisedAddr, err := parseIPv4(cfg.PublicAddr)
	if err != nil {
		return nil, err
	}

	return func() service.SessionFlow {
		return boot.NewFlow(boot.Config{
			Handshake: handshake.Config{
				KeyChallenge: defaultKeyChallenge(),
				KeyComplete:  defaultKeyComplete(),
			},
			Login: loginflow.Config{
				Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
					if packet.Login != StubLogin || packet.LoginKey != StubLoginKey {
						return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
					}

					return loginflow.Result{
						Accepted:      true,
						Empire:        2,
						LoginSuccess4: sampleLoginSuccessPacket(advertisedAddr, advertisedPort),
					}
				},
			},
			WorldEntry: worldentry.Config{
				SelectCharacter: func(index uint8) worldentry.Result {
					if index > 1 {
						return worldentry.Result{Accepted: false}
					}
					return worldentry.Result{
						Accepted:      true,
						MainCharacter: sampleMainCharacter(),
						PlayerPoints:  samplePlayerPoints(),
					}
				},
			},
		})
	}, nil
}

func parsePort(addr string) (uint16, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, ErrInvalidLegacyAddr
	}

	parsed, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0, ErrInvalidLegacyAddr
	}

	return uint16(parsed), nil
}

func parseIPv4(addr string) (uint32, error) {
	ip := net.ParseIP(addr).To4()
	if ip == nil {
		return 0, ErrInvalidPublicAddr
	}

	return binary.LittleEndian.Uint32(ip), nil
}

func defaultKeyChallenge() control.KeyChallengePacket {
	return control.KeyChallengePacket{
		ServerPublicKey: sequentialBytes32(0x00),
		Challenge:       sequentialBytes32(0x20),
		ServerTime:      0x01020304,
	}
}

func defaultKeyComplete() control.KeyCompletePacket {
	return control.KeyCompletePacket{
		EncryptedToken: sequentialBytes48(0x80),
		Nonce:          sequentialBytes24(0xb0),
	}
}

func sampleLoginSuccessPacket(addr uint32, port uint16) loginproto.LoginSuccess4Packet {
	packet := loginproto.LoginSuccess4Packet{
		GuildIDs: [loginproto.PlayerCount]uint32{10, 0, 0, 0},
		GuildNames: [loginproto.PlayerCount]string{
			"Alpha",
			"",
			"",
			"",
		},
		Handle:    0x11223344,
		RandomKey: 0x55667788,
	}

	packet.Players[0] = loginproto.SimplePlayer{
		ID:          1,
		Name:        "Mkmk",
		Job:         1,
		Level:       15,
		PlayMinutes: 4321,
		ST:          6,
		HT:          5,
		DX:          4,
		IQ:          3,
		MainPart:    101,
		ChangeName:  0,
		HairPart:    201,
		Dummy:       [4]byte{1, 2, 3, 4},
		X:           1000,
		Y:           2000,
		Addr:        addr,
		Port:        port,
		SkillGroup:  1,
	}

	return packet
}

func sampleMainCharacter() worldproto.MainCharacterPacket {
	return worldproto.MainCharacterPacket{
		VID:        0x01020304,
		RaceNum:    2,
		Name:       "Mkmk",
		BGMName:    "",
		BGMVolume:  math.Float32frombits(0),
		X:          1000,
		Y:          2000,
		Z:          0,
		Empire:     2,
		SkillGroup: 1,
	}
}

func samplePlayerPoints() worldproto.PlayerPointsPacket {
	var points worldproto.PlayerPointsPacket
	points.Points[0] = 15
	points.Points[1] = 1234
	points.Points[2] = 5678
	points.Points[3] = 900
	points.Points[4] = 1000
	points.Points[5] = 200
	points.Points[6] = 300
	points.Points[7] = 999999
	points.Points[8] = 50
	return points
}

func sequentialBytes32(start byte) [32]byte {
	var out [32]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func sequentialBytes48(start byte) [48]byte {
	var out [48]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

func sequentialBytes24(start byte) [24]byte {
	var out [24]byte
	for i := range out {
		out[i] = start + byte(i)
	}
	return out
}

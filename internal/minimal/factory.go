package minimal

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"

	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/authboot"
	"github.com/MikelCalvo/go-metin2-server/internal/boot"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
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
)

var (
	ErrInvalidLegacyAddr = errors.New("invalid legacy addr")
	ErrInvalidPublicAddr = errors.New("invalid public addr")
)

type loginKeyGenerator func() (uint32, error)

func NewAuthSessionFactory() service.SessionFactory {
	return newAuthSessionFactory(loginticket.NewFileStore(defaultTicketStoreDir()), randomLoginKey)
}

func newAuthSessionFactory(store loginticket.Store, generateLoginKey loginKeyGenerator) service.SessionFactory {
	if store == nil {
		store = loginticket.NewFileStore(defaultTicketStoreDir())
	}
	if generateLoginKey == nil {
		generateLoginKey = randomLoginKey
	}

	return func() service.SessionFlow {
		return authboot.NewFlow(authboot.Config{
			Handshake: handshake.Config{
				KeyChallenge: defaultKeyChallenge(),
				KeyComplete:  defaultKeyComplete(),
			},
			Auth: authflow.Config{
				Authenticate: func(packet authproto.Login3Packet) authflow.Result {
					if packet.Login != StubLogin || packet.Password != StubPassword {
						return authflow.Result{Accepted: false, FailureStatus: "WRONGPWD"}
					}

					loginKey, ok := issueLoginTicket(store, packet.Login, generateLoginKey)
					if !ok {
						return authflow.Result{Accepted: false, FailureStatus: "FAILED"}
					}

					return authflow.Result{Accepted: true, LoginKey: loginKey}
				},
			},
		})
	}
}

func NewGameSessionFactory(cfg config.Service) (service.SessionFactory, error) {
	return newGameSessionFactory(cfg, loginticket.NewFileStore(defaultTicketStoreDir()))
}

func newGameSessionFactory(cfg config.Service, store loginticket.Store) (service.SessionFactory, error) {
	advertisedPort, err := parsePort(cfg.LegacyAddr)
	if err != nil {
		return nil, err
	}

	advertisedAddr, err := parseIPv4(cfg.PublicAddr)
	if err != nil {
		return nil, err
	}

	if store == nil {
		store = loginticket.NewFileStore(defaultTicketStoreDir())
	}

	return func() service.SessionFlow {
		var sessionTicket loginticket.Ticket
		var hasTicket bool

		return boot.NewFlow(boot.Config{
			Handshake: handshake.Config{
				KeyChallenge: defaultKeyChallenge(),
				KeyComplete:  defaultKeyComplete(),
			},
			Login: loginflow.Config{
				Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
					ticket, err := store.Consume(packet.Login, packet.LoginKey)
					if err != nil {
						return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
					}

					sessionTicket = ticket
					hasTicket = true
					return loginflow.Result{
						Accepted:      true,
						Empire:        ticketEmpire(ticket),
						LoginSuccess4: ticketLoginSuccessPacket(ticket, advertisedAddr, advertisedPort),
					}
				},
			},
			WorldEntry: worldentry.Config{
				SelectCharacter: func(index uint8) worldentry.Result {
					if !hasTicket || int(index) >= len(sessionTicket.Characters) {
						return worldentry.Result{Accepted: false}
					}

					selected := sessionTicket.Characters[index]
					return worldentry.Result{
						Accepted:      true,
						MainCharacter: ticketMainCharacterPacket(selected),
						PlayerPoints:  ticketPlayerPointsPacket(selected),
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

func issueLoginTicket(store loginticket.Store, login string, generateLoginKey loginKeyGenerator) (uint32, bool) {
	for range 8 {
		loginKey, err := generateLoginKey()
		if err != nil || loginKey == 0 {
			continue
		}

		err = store.Issue(loginticket.Ticket{
			Login:      login,
			LoginKey:   loginKey,
			Characters: stubCharacters(),
		})
		if err == nil {
			return loginKey, true
		}
		if errors.Is(err, loginticket.ErrTicketExists) {
			continue
		}
		return 0, false
	}

	return 0, false
}

func randomLoginKey() (uint32, error) {
	var raw [4]byte
	for range 8 {
		if _, err := rand.Read(raw[:]); err != nil {
			return 0, err
		}
		loginKey := binary.LittleEndian.Uint32(raw[:])
		if loginKey != 0 {
			return loginKey, nil
		}
	}

	return 0, errors.New("failed to generate non-zero login key")
}

func ticketEmpire(ticket loginticket.Ticket) uint8 {
	if len(ticket.Characters) == 0 {
		return 2
	}
	if ticket.Characters[0].Empire == 0 {
		return 2
	}
	return ticket.Characters[0].Empire
}

func ticketLoginSuccessPacket(ticket loginticket.Ticket, addr uint32, port uint16) loginproto.LoginSuccess4Packet {
	packet := loginproto.LoginSuccess4Packet{
		Handle:    0x11223344,
		RandomKey: 0x55667788,
	}

	for i, character := range ticket.Characters {
		if i >= loginproto.PlayerCount {
			break
		}

		packet.Players[i] = loginproto.SimplePlayer{
			ID:          character.ID,
			Name:        character.Name,
			Job:         character.Job,
			Level:       character.Level,
			PlayMinutes: character.PlayMinutes,
			ST:          character.ST,
			HT:          character.HT,
			DX:          character.DX,
			IQ:          character.IQ,
			MainPart:    character.MainPart,
			ChangeName:  character.ChangeName,
			HairPart:    character.HairPart,
			Dummy:       character.Dummy,
			X:           character.X,
			Y:           character.Y,
			Addr:        addr,
			Port:        port,
			SkillGroup:  character.SkillGroup,
		}
		packet.GuildIDs[i] = character.GuildID
		packet.GuildNames[i] = character.GuildName
	}

	return packet
}

func ticketMainCharacterPacket(character loginticket.Character) worldproto.MainCharacterPacket {
	return worldproto.MainCharacterPacket{
		VID:        character.VID,
		RaceNum:    character.RaceNum,
		Name:       character.Name,
		BGMName:    "",
		BGMVolume:  math.Float32frombits(0),
		X:          character.X,
		Y:          character.Y,
		Z:          character.Z,
		Empire:     character.Empire,
		SkillGroup: character.SkillGroup,
	}
}

func ticketPlayerPointsPacket(character loginticket.Character) worldproto.PlayerPointsPacket {
	return worldproto.PlayerPointsPacket{Points: character.Points}
}

func stubCharacters() []loginticket.Character {
	first := loginticket.Character{
		ID:          1,
		VID:         0x01020304,
		Name:        "MkmkWar",
		Job:         0,
		RaceNum:     0,
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
		Z:           0,
		Empire:      2,
		SkillGroup:  1,
		GuildID:     10,
		GuildName:   "Alpha",
	}
	first.Points[0] = 15
	first.Points[1] = 1234
	first.Points[2] = 5678
	first.Points[3] = 900
	first.Points[4] = 1000
	first.Points[5] = 200
	first.Points[6] = 300
	first.Points[7] = 999999
	first.Points[8] = 50

	second := loginticket.Character{
		ID:          2,
		VID:         0x01020305,
		Name:        "MkmkSura",
		Job:         3,
		RaceNum:     3,
		Level:       12,
		PlayMinutes: 2100,
		ST:          4,
		HT:          5,
		DX:          3,
		IQ:          8,
		MainPart:    102,
		ChangeName:  0,
		HairPart:    202,
		Dummy:       [4]byte{5, 6, 7, 8},
		X:           1200,
		Y:           2100,
		Z:           0,
		Empire:      2,
		SkillGroup:  2,
	}
	second.Points[0] = 12
	second.Points[1] = 900
	second.Points[2] = 1800
	second.Points[3] = 700
	second.Points[4] = 800
	second.Points[5] = 150
	second.Points[6] = 120
	second.Points[7] = 500000
	second.Points[8] = 20

	return []loginticket.Character{first, second}
}

func defaultTicketStoreDir() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-login-tickets")
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

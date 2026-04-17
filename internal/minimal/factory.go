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
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	authflow "github.com/MikelCalvo/go-metin2-server/internal/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/authboot"
	"github.com/MikelCalvo/go-metin2-server/internal/boot"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	gameflow "github.com/MikelCalvo/go-metin2-server/internal/game"
	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	authproto "github.com/MikelCalvo/go-metin2-server/internal/proto/auth"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
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
	return newAuthSessionFactoryWithAccountStore(
		loginticket.NewFileStore(defaultTicketStoreDir()),
		accountstore.NewFileStore(defaultAccountStoreDir()),
		randomLoginKey,
	)
}

func newAuthSessionFactory(store loginticket.Store, generateLoginKey loginKeyGenerator) service.SessionFactory {
	return newAuthSessionFactoryWithAccountStore(store, nil, generateLoginKey)
}

func newAuthSessionFactoryWithAccountStore(store loginticket.Store, accounts accountstore.Store, generateLoginKey loginKeyGenerator) service.SessionFactory {
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

					characters, ok := loadOrCreateAccountCharacters(accounts, packet.Login)
					if !ok {
						return authflow.Result{Accepted: false, FailureStatus: "FAILED"}
					}
					loginKey, ok := issueLoginTicket(store, packet.Login, characters, generateLoginKey)
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
	return newGameSessionFactoryWithAccountStore(
		cfg,
		loginticket.NewFileStore(defaultTicketStoreDir()),
		accountstore.NewFileStore(defaultAccountStoreDir()),
	)
}

func newGameSessionFactory(cfg config.Service, store loginticket.Store) (service.SessionFactory, error) {
	return newGameSessionFactoryWithAccountStore(cfg, store, nil)
}

func newGameSessionFactoryWithAccountStore(cfg config.Service, store loginticket.Store, accounts accountstore.Store) (service.SessionFactory, error) {
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
		var selectedIndex uint8
		var hasSelected bool

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
					hasSelected = false
					selectedIndex = 0
					return loginflow.Result{
						Accepted:      true,
						Empire:        ticketEmpire(ticket),
						LoginSuccess4: ticketLoginSuccessPacket(ticket, advertisedAddr, advertisedPort),
					}
				},
			},
			WorldEntry: worldentry.Config{
				CreateCharacter: func(packet worldproto.CharacterCreatePacket) worldentry.CreateResult {
					if !hasTicket {
						return worldentry.CreateResult{Accepted: false, FailureType: 0}
					}
					created, failureType, ok := createCharacterInTicket(&sessionTicket, packet, ticketEmpire(sessionTicket))
					if !ok {
						return worldentry.CreateResult{Accepted: false, FailureType: failureType}
					}
					if !saveAccountCharacters(accounts, sessionTicket.Login, sessionTicket.Characters) {
						return worldentry.CreateResult{Accepted: false, FailureType: 0}
					}
					return worldentry.CreateResult{
						Accepted: true,
						Player:   ticketPlayerCreateSuccessPacket(created, packet.Index, advertisedAddr, advertisedPort),
					}
				},
				SelectCharacter: func(index uint8) worldentry.Result {
					if !hasTicket || int(index) >= len(sessionTicket.Characters) {
						return worldentry.Result{Accepted: false}
					}

					selected := sessionTicket.Characters[index]
					if selected.ID == 0 {
						return worldentry.Result{Accepted: false}
					}
					selectedIndex = index
					hasSelected = true
					return worldentry.Result{
						Accepted:      true,
						MainCharacter: ticketMainCharacterPacket(selected),
						PlayerPoints:  ticketPlayerPointsPacket(selected),
					}
				},
			},
			Game: gameflow.Config{
				HandleMove: func(packet movep.MovePacket) gameflow.Result {
					if !hasTicket || !hasSelected || int(selectedIndex) >= len(sessionTicket.Characters) {
						return gameflow.Result{Accepted: false}
					}
					selected := sessionTicket.Characters[selectedIndex]
					if selected.ID == 0 {
						return gameflow.Result{Accepted: false}
					}
					selected.X = packet.X
					selected.Y = packet.Y
					sessionTicket.Characters[selectedIndex] = selected
					return gameflow.Result{Accepted: true, Replication: ticketMoveAckPacket(selected, packet)}
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

func issueLoginTicket(store loginticket.Store, login string, characters []loginticket.Character, generateLoginKey loginKeyGenerator) (uint32, bool) {
	if len(characters) == 0 {
		characters = stubCharacters()
	}
	for range 8 {
		loginKey, err := generateLoginKey()
		if err != nil || loginKey == 0 {
			continue
		}

		err = store.Issue(loginticket.Ticket{
			Login:      login,
			LoginKey:   loginKey,
			Characters: cloneCharacters(characters),
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

func loadOrCreateAccountCharacters(store accountstore.Store, login string) ([]loginticket.Character, bool) {
	if store == nil {
		return cloneCharacters(stubCharacters()), true
	}
	account, err := store.Load(login)
	if err == nil {
		return cloneCharacters(account.Characters), true
	}
	if !errors.Is(err, accountstore.ErrAccountNotFound) {
		return nil, false
	}
	characters := cloneCharacters(stubCharacters())
	if err := store.Save(accountstore.Account{Login: login, Characters: characters}); err != nil {
		return nil, false
	}
	return cloneCharacters(characters), true
}

func saveAccountCharacters(store accountstore.Store, login string, characters []loginticket.Character) bool {
	if store == nil {
		return true
	}
	return store.Save(accountstore.Account{Login: login, Characters: cloneCharacters(characters)}) == nil
}

func cloneCharacters(characters []loginticket.Character) []loginticket.Character {
	if len(characters) == 0 {
		return nil
	}
	cloned := make([]loginticket.Character, len(characters))
	copy(cloned, characters)
	return cloned
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

func ticketMoveAckPacket(character loginticket.Character, packet movep.MovePacket) movep.MoveAckPacket {
	return movep.MoveAckPacket{
		Func:     packet.Func,
		Arg:      packet.Arg,
		Rot:      packet.Rot,
		VID:      character.VID,
		X:        packet.X,
		Y:        packet.Y,
		Time:     packet.Time,
		Duration: 250,
	}
}

func ticketPlayerCreateSuccessPacket(character loginticket.Character, index uint8, addr uint32, port uint16) worldproto.PlayerCreateSuccessPacket {
	return worldproto.PlayerCreateSuccessPacket{
		Index: index,
		Player: loginproto.SimplePlayer{
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
		},
	}
}

func createCharacterInTicket(ticket *loginticket.Ticket, packet worldproto.CharacterCreatePacket, empire uint8) (loginticket.Character, uint8, bool) {
	if ticket == nil || packet.Index >= loginproto.PlayerCount {
		return loginticket.Character{}, 0, false
	}
	if !isValidCharacterName(packet.Name) || !isValidCreateRace(packet.RaceNum) || packet.Shape > 1 {
		return loginticket.Character{}, 0, false
	}
	if hasDuplicateCharacterName(ticket.Characters, packet.Name) {
		return loginticket.Character{}, 1, false
	}

	index := int(packet.Index)
	if index < len(ticket.Characters) && ticket.Characters[index].ID != 0 {
		return loginticket.Character{}, 0, false
	}
	if len(ticket.Characters) <= index {
		extended := make([]loginticket.Character, index+1)
		copy(extended, ticket.Characters)
		ticket.Characters = extended
	}

	character := buildCreatedCharacter(nextCharacterID(ticket.Characters), nextCharacterVID(ticket.Characters), packet, empire)
	ticket.Characters[index] = character
	return character, 0, true
}

type initialCharacterStats struct {
	ST    uint8
	HT    uint8
	DX    uint8
	IQ    uint8
	MaxHP int32
	MaxSP int32
}

func buildCreatedCharacter(id uint32, vid uint32, packet worldproto.CharacterCreatePacket, empire uint8) loginticket.Character {
	stats := initialStatsForRace(packet.RaceNum)
	x, y := spawnPositionForSlot(packet.Index)
	points := initialPointsForRace(packet.RaceNum)
	return loginticket.Character{
		ID:          id,
		VID:         vid,
		Name:        packet.Name,
		Job:         uint8(packet.RaceNum),
		RaceNum:     packet.RaceNum,
		Level:       1,
		PlayMinutes: 0,
		ST:          stats.ST,
		HT:          stats.HT,
		DX:          stats.DX,
		IQ:          stats.IQ,
		MainPart:    uint16(packet.Shape),
		ChangeName:  0,
		HairPart:    0,
		Dummy:       [4]byte{},
		X:           x,
		Y:           y,
		Z:           0,
		Empire:      empire,
		SkillGroup:  0,
		Points:      points,
	}
}

func initialStatsForRace(race uint16) initialCharacterStats {
	switch race {
	case 0, 4:
		return initialCharacterStats{ST: 6, HT: 4, DX: 3, IQ: 3, MaxHP: 600, MaxSP: 200}
	case 1, 5:
		return initialCharacterStats{ST: 4, HT: 3, DX: 6, IQ: 3, MaxHP: 650, MaxSP: 200}
	case 2, 6:
		return initialCharacterStats{ST: 5, HT: 3, DX: 3, IQ: 5, MaxHP: 650, MaxSP: 200}
	case 3, 7:
		return initialCharacterStats{ST: 3, HT: 4, DX: 3, IQ: 6, MaxHP: 700, MaxSP: 200}
	default:
		return initialCharacterStats{}
	}
}

func initialPointsForRace(race uint16) [worldproto.PointCount]int32 {
	stats := initialStatsForRace(race)
	var points [worldproto.PointCount]int32
	points[0] = 1
	points[1] = stats.MaxHP
	points[2] = stats.MaxSP
	return points
}

func spawnPositionForSlot(index uint8) (int32, int32) {
	return 1000 + int32(index)*100, 2000 + int32(index)*100
}

func nextCharacterID(characters []loginticket.Character) uint32 {
	var maxID uint32
	for _, character := range characters {
		if character.ID > maxID {
			maxID = character.ID
		}
	}
	if maxID == 0 {
		return 1
	}
	return maxID + 1
}

func nextCharacterVID(characters []loginticket.Character) uint32 {
	var maxVID uint32
	for _, character := range characters {
		if character.VID > maxVID {
			maxVID = character.VID
		}
	}
	if maxVID == 0 {
		return 0x01020304
	}
	return maxVID + 1
}

func isValidCreateRace(race uint16) bool {
	switch race {
	case 0, 1, 2, 3, 4, 5, 6, 7:
		return true
	default:
		return false
	}
}

func isValidCharacterName(name string) bool {
	if name == "" || len(name) >= worldproto.CharacterNameFieldSize {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

func hasDuplicateCharacterName(characters []loginticket.Character, name string) bool {
	for _, character := range characters {
		if character.ID != 0 && strings.EqualFold(character.Name, name) {
			return true
		}
	}
	return false
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

func defaultAccountStoreDir() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-accounts")
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

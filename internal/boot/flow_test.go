package boot

import (
	"bytes"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/handshake"
	loginflow "github.com/MikelCalvo/go-metin2-server/internal/login"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	loginproto "github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	"github.com/MikelCalvo/go-metin2-server/internal/session"
)

func TestStartBeginsWithTheHandshakeChallenge(t *testing.T) {
	flow := NewFlow(testConfig())

	out, err := flow.Start()
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	want := control.EncodeKeyChallenge(testConfig().Handshake.KeyChallenge)
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing frame, got %d", len(out))
	}

	if !bytes.Equal(out[0], want) {
		t.Fatalf("unexpected key challenge bytes: got %x want %x", out[0], want)
	}

	if flow.CurrentPhase() != session.PhaseHandshake {
		t.Fatalf("expected phase %q, got %q", session.PhaseHandshake, flow.CurrentPhase())
	}
}

func TestHandleClientFrameCompletesHandshakeThenProcessesLogin2(t *testing.T) {
	flow := NewFlow(testConfig())

	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	handshakeOut, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}

	if len(handshakeOut) != 2 {
		t.Fatalf("expected 2 handshake frames, got %d", len(handshakeOut))
	}

	if flow.CurrentPhase() != session.PhaseLogin {
		t.Fatalf("expected phase %q after handshake, got %q", session.PhaseLogin, flow.CurrentPhase())
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "mkmk", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}

	loginOut, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	if len(loginOut) != 3 {
		t.Fatalf("expected 3 login frames, got %d", len(loginOut))
	}

	wantEmpire := loginproto.EncodeEmpire(loginproto.EmpirePacket{Empire: 2})
	wantPhase, err := control.EncodePhase(session.PhaseSelect)
	if err != nil {
		t.Fatalf("unexpected phase encode error: %v", err)
	}
	wantSuccess, err := loginproto.EncodeLoginSuccess4(sampleLoginSuccessPacket())
	if err != nil {
		t.Fatalf("unexpected login success encode error: %v", err)
	}

	want := [][]byte{wantEmpire, wantPhase, wantSuccess}
	for i := range want {
		if !bytes.Equal(loginOut[i], want[i]) {
			t.Fatalf("unexpected login frame %d: got %x want %x", i, loginOut[i], want[i])
		}
	}

	if flow.CurrentPhase() != session.PhaseSelect {
		t.Fatalf("expected phase %q after login, got %q", session.PhaseSelect, flow.CurrentPhase())
	}
}

func TestHandleClientFrameReturnsLoginFailureWithoutLeavingLogin(t *testing.T) {
	cfg := testConfig()
	cfg.Login.Authenticate = func(loginproto.Login2Packet) loginflow.Result {
		return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
	}

	flow := NewFlow(cfg)
	if _, err := flow.Start(); err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	_, err := flow.HandleClientFrame(decodeSingleFrame(t, control.EncodeKeyResponse(control.KeyResponsePacket{
		ClientPublicKey:   sequentialBytes32(0x40),
		ChallengeResponse: sequentialBytes32(0x60),
	})))
	if err != nil {
		t.Fatalf("unexpected handshake error: %v", err)
	}

	login2Raw, err := loginproto.EncodeLogin2(loginproto.Login2Packet{Login: "ghost", LoginKey: 0x01020304})
	if err != nil {
		t.Fatalf("unexpected login2 encode error: %v", err)
	}

	out, err := flow.HandleClientFrame(decodeSingleFrame(t, login2Raw))
	if err != nil {
		t.Fatalf("unexpected login error: %v", err)
	}

	wantFailure, err := loginproto.EncodeLoginFailure(loginproto.LoginFailurePacket{Status: "NOID"})
	if err != nil {
		t.Fatalf("unexpected login failure encode error: %v", err)
	}

	if len(out) != 1 || !bytes.Equal(out[0], wantFailure) {
		t.Fatalf("unexpected login failure output: got %x want %x", out, wantFailure)
	}

	if flow.CurrentPhase() != session.PhaseLogin {
		t.Fatalf("expected phase %q, got %q", session.PhaseLogin, flow.CurrentPhase())
	}
}

func decodeSingleFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()

	decoder := frame.NewDecoder(4096)
	frames, err := decoder.Feed(raw)
	if err != nil {
		t.Fatalf("unexpected frame decode error: %v", err)
	}

	if len(frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(frames))
	}

	return frames[0]
}

func testConfig() Config {
	return Config{
		Handshake: handshake.Config{
			KeyChallenge: control.KeyChallengePacket{
				ServerPublicKey: sequentialBytes32(0x00),
				Challenge:       sequentialBytes32(0x20),
				ServerTime:      0x01020304,
			},
			KeyComplete: control.KeyCompletePacket{
				EncryptedToken: sequentialBytes48(0x80),
				Nonce:          sequentialBytes24(0xb0),
			},
		},
		Login: loginflow.Config{
			Authenticate: func(packet loginproto.Login2Packet) loginflow.Result {
				if packet.Login == "mkmk" && packet.LoginKey == 0x01020304 {
					return loginflow.Result{
						Accepted:      true,
						Empire:        2,
						LoginSuccess4: sampleLoginSuccessPacket(),
					}
				}

				return loginflow.Result{Accepted: false, FailureStatus: "NOID"}
			},
		},
	}
}

func sampleLoginSuccessPacket() loginproto.LoginSuccess4Packet {
	packet := loginproto.LoginSuccess4Packet{
		GuildIDs: [loginproto.PlayerCount]uint32{10, 20, 0, 0},
		GuildNames: [loginproto.PlayerCount]string{
			"Alpha",
			"Beta",
			"",
			"",
		},
		Handle:    0x11223344,
		RandomKey: 0x55667788,
	}

	packet.Players[0] = loginproto.SimplePlayer{
		ID:          1,
		Name:        "Chris",
		Job:         2,
		Level:       30,
		PlayMinutes: 1234,
		ST:          3,
		HT:          4,
		DX:          5,
		IQ:          6,
		MainPart:    100,
		ChangeName:  0,
		HairPart:    200,
		Dummy:       [4]byte{9, 8, 7, 6},
		X:           1000,
		Y:           2000,
		Addr:        0x0100007f,
		Port:        13000,
		SkillGroup:  1,
	}

	packet.Players[1] = loginproto.SimplePlayer{
		ID:          2,
		Name:        "Mkmk",
		Job:         1,
		Level:       15,
		PlayMinutes: 4321,
		ST:          6,
		HT:          5,
		DX:          4,
		IQ:          3,
		MainPart:    101,
		ChangeName:  1,
		HairPart:    201,
		Dummy:       [4]byte{1, 2, 3, 4},
		X:           3000,
		Y:           4000,
		Addr:        0x0200007f,
		Port:        13001,
		SkillGroup:  2,
	}

	return packet
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

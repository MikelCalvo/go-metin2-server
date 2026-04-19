package securecipher

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
)

const (
	publicKeySize    = 32
	secretSize       = 32
	sessionKeySize   = 32
	challengeSize    = 32
	sessionTokenSize = 32
	nonceSize        = chacha20poly1305.NonceSizeX
	tagSize          = 16
	keystreamBlock   = 64
)

var (
	ErrInvalidServerPublicKey  = errors.New("invalid server public key")
	ErrInvalidClientPublicKey  = errors.New("invalid client public key")
	ErrChallengeNotStarted     = errors.New("challenge not started")
	ErrChallengeResponseFailed = errors.New("challenge response verification failed")
	ErrCipherNotReady          = errors.New("secure cipher is not ready")
)

type ServerConfig struct {
	Random     io.Reader
	ServerTime func() uint32
}

type ClientConfig struct {
	Random io.Reader
}

type ServerSession struct {
	random     io.Reader
	serverTime func() uint32
	curve      ecdh.Curve
	privateKey *ecdh.PrivateKey
	publicKey  [publicKeySize]byte
	challenge  [challengeSize]byte
	txKey      [sessionKeySize]byte
	rxKey      [sessionKeySize]byte
	txNonce    [nonceSize]byte
	rxNonce    [nonceSize]byte
	txOffset   uint64
	rxOffset   uint64
	token      [sessionTokenSize]byte
	started    bool
	active     bool
	pending    bool
}

type ClientSession struct {
	random     io.Reader
	curve      ecdh.Curve
	privateKey *ecdh.PrivateKey
	publicKey  [publicKeySize]byte
	txKey      [sessionKeySize]byte
	rxKey      [sessionKeySize]byte
	txNonce    [nonceSize]byte
	rxNonce    [nonceSize]byte
	txOffset   uint64
	rxOffset   uint64
	token      [sessionTokenSize]byte
	active     bool
}

func NewServerSession(cfg ServerConfig) *ServerSession {
	randomReader := cfg.Random
	if randomReader == nil {
		randomReader = rand.Reader
	}
	serverTime := cfg.ServerTime
	if serverTime == nil {
		serverTime = func() uint32 { return 0 }
	}
	return &ServerSession{
		random:     randomReader,
		serverTime: serverTime,
		curve:      ecdh.X25519(),
	}
}

func NewClientSession(cfg ClientConfig) *ClientSession {
	randomReader := cfg.Random
	if randomReader == nil {
		randomReader = rand.Reader
	}
	return &ClientSession{
		random: randomReader,
		curve:  ecdh.X25519(),
	}
}

func (s *ServerSession) Start() (control.KeyChallengePacket, error) {
	if err := s.generateKeyPair(); err != nil {
		return control.KeyChallengePacket{}, err
	}
	if _, err := io.ReadFull(s.random, s.challenge[:]); err != nil {
		return control.KeyChallengePacket{}, fmt.Errorf("generate challenge: %w", err)
	}
	clearBytes(s.txKey[:])
	clearBytes(s.rxKey[:])
	clearBytes(s.txNonce[:])
	clearBytes(s.rxNonce[:])
	clearBytes(s.token[:])
	s.txOffset = 0
	s.rxOffset = 0
	s.active = false
	s.pending = false
	s.started = true

	return control.KeyChallengePacket{
		ServerPublicKey: s.publicKey,
		Challenge:       s.challenge,
		ServerTime:      s.serverTime(),
	}, nil
}

func (s *ServerSession) HandleKeyResponse(packet control.KeyResponsePacket) (control.KeyCompletePacket, error) {
	if !s.started || s.privateKey == nil {
		return control.KeyCompletePacket{}, ErrChallengeNotStarted
	}
	clientPublic, err := s.curve.NewPublicKey(packet.ClientPublicKey[:])
	if err != nil {
		return control.KeyCompletePacket{}, fmt.Errorf("%w: %v", ErrInvalidClientPublicKey, err)
	}
	if err := deriveServerSessionKeys(s.privateKey, s.publicKey, packet.ClientPublicKey, &s.rxKey, &s.txKey); err != nil {
		return control.KeyCompletePacket{}, err
	}
	clearBytes(s.txNonce[:])
	clearBytes(s.rxNonce[:])
	s.txNonce[0] = 0x01
	s.rxNonce[0] = 0x02
	s.txOffset = 0
	s.rxOffset = 0
	if !verifyChallengeResponse(s.challenge[:], s.rxKey[:], packet.ChallengeResponse[:]) {
		return control.KeyCompletePacket{}, ErrChallengeResponseFailed
	}
	if _, err := io.ReadFull(s.random, s.token[:]); err != nil {
		return control.KeyCompletePacket{}, fmt.Errorf("generate session token: %w", err)
	}
	ciphertext, nonce, err := encryptToken(s.random, s.txKey[:], s.token[:])
	if err != nil {
		return control.KeyCompletePacket{}, err
	}
	var encryptedToken [48]byte
	copy(encryptedToken[:], ciphertext)
	var completeNonce [24]byte
	copy(completeNonce[:], nonce)
	s.pending = true
	_ = clientPublic
	return control.KeyCompletePacket{EncryptedToken: encryptedToken, Nonce: completeNonce}, nil
}

func (s *ServerSession) EncryptOutgoing(raw []byte) ([]byte, error) {
	out := cloneBytes(raw)
	if len(out) == 0 {
		return out, nil
	}
	if s.pending {
		s.pending = false
		s.active = true
		return out, nil
	}
	if !s.active {
		return out, nil
	}
	if err := applyStreamCipher(out, s.txKey[:], &s.txOffset, s.txNonce[:]); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ServerSession) DecryptIncoming(raw []byte) ([]byte, error) {
	out := cloneBytes(raw)
	if len(out) == 0 || !s.active {
		return out, nil
	}
	if err := applyStreamCipher(out, s.rxKey[:], &s.rxOffset, s.rxNonce[:]); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ClientSession) HandleKeyChallenge(packet control.KeyChallengePacket) (control.KeyResponsePacket, error) {
	if err := c.generateKeyPair(); err != nil {
		return control.KeyResponsePacket{}, err
	}
	serverPublic, err := c.curve.NewPublicKey(packet.ServerPublicKey[:])
	if err != nil {
		return control.KeyResponsePacket{}, fmt.Errorf("%w: %v", ErrInvalidServerPublicKey, err)
	}
	if err := deriveClientSessionKeys(c.privateKey, c.publicKey, packet.ServerPublicKey, &c.rxKey, &c.txKey); err != nil {
		return control.KeyResponsePacket{}, err
	}
	clearBytes(c.txNonce[:])
	clearBytes(c.rxNonce[:])
	c.txNonce[0] = 0x02
	c.rxNonce[0] = 0x01
	c.txOffset = 0
	c.rxOffset = 0
	c.active = false
	response := computeChallengeResponse(packet.Challenge[:], c.txKey[:])
	_ = serverPublic
	return control.KeyResponsePacket{ClientPublicKey: c.publicKey, ChallengeResponse: response}, nil
}

func (c *ClientSession) HandleKeyComplete(packet control.KeyCompletePacket) error {
	if c.privateKey == nil {
		return ErrCipherNotReady
	}
	plaintext, err := decryptToken(c.rxKey[:], packet.Nonce[:], packet.EncryptedToken[:])
	if err != nil {
		return err
	}
	copy(c.token[:], plaintext)
	c.active = true
	return nil
}

func (c *ClientSession) EncryptOutgoing(raw []byte) ([]byte, error) {
	out := cloneBytes(raw)
	if len(out) == 0 || !c.active {
		return out, nil
	}
	if err := applyStreamCipher(out, c.txKey[:], &c.txOffset, c.txNonce[:]); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ClientSession) DecryptIncoming(raw []byte) ([]byte, error) {
	out := cloneBytes(raw)
	if len(out) == 0 || !c.active {
		return out, nil
	}
	if err := applyStreamCipher(out, c.rxKey[:], &c.rxOffset, c.rxNonce[:]); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ServerSession) generateKeyPair() error {
	privateKey, err := s.curve.GenerateKey(s.random)
	if err != nil {
		return fmt.Errorf("generate server keypair: %w", err)
	}
	s.privateKey = privateKey
	copy(s.publicKey[:], privateKey.PublicKey().Bytes())
	return nil
}

func (c *ClientSession) generateKeyPair() error {
	privateKey, err := c.curve.GenerateKey(c.random)
	if err != nil {
		return fmt.Errorf("generate client keypair: %w", err)
	}
	c.privateKey = privateKey
	copy(c.publicKey[:], privateKey.PublicKey().Bytes())
	return nil
}

func deriveClientSessionKeys(privateKey *ecdh.PrivateKey, clientPublic [32]byte, serverPublic [32]byte, rxKey *[32]byte, txKey *[32]byte) error {
	keys, err := deriveSharedKeys(privateKey, serverPublic[:], clientPublic[:], serverPublic[:])
	if err != nil {
		return err
	}
	copy(rxKey[:], keys[:32])
	copy(txKey[:], keys[32:])
	clearBytes(keys)
	return nil
}

func deriveServerSessionKeys(privateKey *ecdh.PrivateKey, serverPublic [32]byte, clientPublic [32]byte, rxKey *[32]byte, txKey *[32]byte) error {
	keys, err := deriveSharedKeys(privateKey, clientPublic[:], clientPublic[:], serverPublic[:])
	if err != nil {
		return err
	}
	copy(txKey[:], keys[:32])
	copy(rxKey[:], keys[32:])
	clearBytes(keys)
	return nil
}

func deriveSharedKeys(privateKey *ecdh.PrivateKey, peerPublicBytes []byte, clientPublic []byte, serverPublic []byte) ([]byte, error) {
	curve := ecdh.X25519()
	peerPublic, err := curve.NewPublicKey(peerPublicBytes)
	if err != nil {
		return nil, fmt.Errorf("create peer public key: %w", err)
	}
	shared, err := privateKey.ECDH(peerPublic)
	if err != nil {
		return nil, fmt.Errorf("derive shared secret: %w", err)
	}
	hash, err := blake2b.New(64, nil)
	if err != nil {
		return nil, fmt.Errorf("create blake2b hasher: %w", err)
	}
	if _, err := hash.Write(shared); err != nil {
		return nil, fmt.Errorf("hash shared secret: %w", err)
	}
	if _, err := hash.Write(clientPublic); err != nil {
		return nil, fmt.Errorf("hash client public key: %w", err)
	}
	if _, err := hash.Write(serverPublic); err != nil {
		return nil, fmt.Errorf("hash server public key: %w", err)
	}
	keys := hash.Sum(nil)
	clearBytes(shared)
	return keys, nil
}

func computeChallengeResponse(challenge []byte, key []byte) [32]byte {
	mac := hmac.New(sha512.New512_256, key)
	_, _ = mac.Write(challenge)
	var response [32]byte
	copy(response[:], mac.Sum(nil))
	return response
}

func verifyChallengeResponse(challenge []byte, key []byte, response []byte) bool {
	expected := computeChallengeResponse(challenge, key)
	return hmac.Equal(expected[:], response)
}

func encryptToken(random io.Reader, key []byte, plaintext []byte) ([]byte, []byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create xchacha20poly1305: %w", err)
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate token nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func decryptToken(key []byte, nonce []byte, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create xchacha20poly1305: %w", err)
	}
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt token: %w", err)
	}
	if len(plaintext) != sessionTokenSize {
		return nil, fmt.Errorf("unexpected session token size: %d", len(plaintext))
	}
	return plaintext, nil
}

func applyStreamCipher(buffer []byte, key []byte, byteOffset *uint64, nonce []byte) error {
	if len(buffer) == 0 {
		return nil
	}
	if len(key) != sessionKeySize {
		return ErrCipherNotReady
	}
	if len(nonce) != nonceSize {
		return ErrCipherNotReady
	}
	remaining := buffer
	if offset := int(*byteOffset % keystreamBlock); offset != 0 {
		keystream := make([]byte, keystreamBlock)
		stream, err := chacha20.NewUnauthenticatedCipher(key, nonce)
		if err != nil {
			return fmt.Errorf("create xchacha20 stream: %w", err)
		}
		stream.SetCounter(uint32(*byteOffset / keystreamBlock))
		stream.XORKeyStream(keystream, keystream)
		use := len(remaining)
		if use > keystreamBlock-offset {
			use = keystreamBlock - offset
		}
		for i := 0; i < use; i++ {
			remaining[i] ^= keystream[offset+i]
		}
		remaining = remaining[use:]
		*byteOffset += uint64(use)
	}
	if len(remaining) == 0 {
		return nil
	}
	stream, err := chacha20.NewUnauthenticatedCipher(key, nonce)
	if err != nil {
		return fmt.Errorf("create xchacha20 stream: %w", err)
	}
	stream.SetCounter(uint32(*byteOffset / keystreamBlock))
	stream.XORKeyStream(remaining, remaining)
	*byteOffset += uint64(len(remaining))
	return nil
}

func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func cloneBytes(b []byte) []byte {
	return append([]byte(nil), b...)
}

func bytesEqual(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

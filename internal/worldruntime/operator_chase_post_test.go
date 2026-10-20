package worldruntime_test

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/minimal"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
	combatproto "github.com/MikelCalvo/go-metin2-server/internal/proto/combat"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/control"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/frame"
	"github.com/MikelCalvo/go-metin2-server/internal/proto/login"
	movep "github.com/MikelCalvo/go-metin2-server/internal/proto/move"
	worldproto "github.com/MikelCalvo/go-metin2-server/internal/proto/world"
	"github.com/MikelCalvo/go-metin2-server/internal/securecipher"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
)

func chaseFrame(t *testing.T, raw []byte) frame.Frame {
	t.Helper()
	frames, err := frame.NewDecoder(4096).Feed(raw)
	if err != nil || len(frames) != 1 {
		t.Fatalf("decode single frame: count=%d err=%v", len(frames), err)
	}
	return frames[0]
}

// This is an external runtime test so it exercises the public gamed-facing
// method instead of calling the private pending-frame executor directly.
func TestOperatorChasePostMovesEngagedSpawnAndRejectsReleasedOrDead(t *testing.T) {
	root := t.TempDir()
	cfg := config.Service{
		PprofAddr: "127.0.0.1:6060", LegacyAddr: ":13000", PublicAddr: "127.0.0.1",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       filepath.Join(root, "accounts"),
		StaticActorStorePath:  filepath.Join(root, "static", "actors.json"),
		InteractionStorePath:  filepath.Join(root, "interactions", "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "items", "items.json"),
		QuestStateStorePath:   filepath.Join(root, "quests", "quests.json"),
		GroundItemStorePath:   filepath.Join(root, "ground", "ground.json"),
		SafeboxStorePath:      filepath.Join(root, "safebox", "safebox.json"),
		CubeRecipeStorePath:   filepath.Join(root, "cube", "cube.json"),
	}
	owner := loginticket.Character{ID: 11, VID: 22, Name: "ChaseOpOwner", Level: 20, MainPart: 101, HairPart: 201, X: 1900, Y: 2800, MapIndex: 42, Empire: 2, SkillGroup: 1}
	owner.Points[0] = 20
	owner.Points[1] = 50
	store := loginticket.NewFileStore(cfg.LoginTicketStoreDir)
	if err := store.Issue(loginticket.Ticket{Login: "chase-op-owner", LoginKey: 1234, Empire: 2, Characters: []loginticket.Character{owner}}); err != nil {
		t.Fatalf("issue owner ticket: %v", err)
	}
	if err := accountstore.NewFileStore(cfg.AccountStoreDir).Save(accountstore.Account{Login: "chase-op-owner", Empire: 2, Characters: []loginticket.Character{owner}}); err != nil {
		t.Fatalf("seed owner account: %v", err)
	}
	runtime, err := minimal.NewGameRuntime(cfg)
	if err != nil {
		t.Fatalf("construct gamed runtime: %v", err)
	}
	if _, err := runtime.ImportContentBundle(contentbundle.Bundle{SpawnGroups: []contentbundle.SpawnGroup{{
		Ref: "practice.operator_chase", Name: "ChaseOpMob", MapIndex: 42, X: 1700, Y: 2800,
		RaceNum: 20350, CombatProfile: "practice_mob",
	}}}); err != nil {
		t.Fatalf("import spawn group: %v", err)
	}
	actor, ok := runtime.SpawnGroupByRef("practice.operator_chase")
	if !ok {
		t.Fatal("spawn group missing")
	}
	flow := runtime.SessionFactory()()
	defer func() {
		if c, ok := flow.(interface{ Close() error }); ok {
			if err := c.Close(); err != nil {
				t.Errorf("close session: %v", err)
			}
		}
	}()
	started, err := flow.Start()
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	var response []byte
	for _, raw := range started {
		f := chaseFrame(t, raw)
		if f.Header == control.HeaderKeyChallenge {
			challenge, err := control.DecodeKeyChallenge(f)
			if err != nil {
				t.Fatal(err)
			}
			client := securecipher.NewClientSession(securecipher.ClientConfig{Random: rand.Reader})
			key, err := client.HandleKeyChallenge(challenge)
			if err != nil {
				t.Fatal(err)
			}
			response = control.EncodeKeyResponse(key)
		}
	}
	if len(response) == 0 {
		t.Fatal("missing secure handshake challenge")
	}
	if _, err := flow.HandleClientFrame(chaseFrame(t, response)); err != nil {
		t.Fatalf("secure handshake: %v", err)
	}
	loginRaw, err := login.EncodeLogin2(login.Login2Packet{Login: "chase-op-owner", LoginKey: 1234})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{loginRaw, worldproto.EncodeCharacterSelect(worldproto.CharacterSelectPacket{Index: 0}), worldproto.EncodeEnterGame()} {
		if _, err := flow.HandleClientFrame(chaseFrame(t, raw)); err != nil {
			t.Fatalf("enter game: %v", err)
		}
	}
	source, ok := flow.(service.ServerFrameSource)
	if !ok {
		t.Fatal("session lacks pending frame source")
	}
	if _, err := source.FlushServerFrames(); err != nil {
		t.Fatalf("flush initial frames: %v", err)
	}
	target, err := flow.HandleClientFrame(chaseFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil || len(target) != 1 {
		t.Fatalf("select target: frames=%d err=%v actor=%+v connected=%+v", len(target), err, actor, runtime.ConnectedCharacters())
	}
	if _, err := source.FlushServerFrames(); err != nil {
		t.Fatalf("flush selection: %v", err)
	}
	hit, err := flow.HandleClientFrame(chaseFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
	if err != nil || len(hit) == 0 {
		t.Fatalf("engage mob: frames=%d err=%v", len(hit), err)
	}
	if _, ok := runtime.SpawnGroupChaseStep(actor.EntityID); !ok {
		t.Fatal("accepted hit did not arm chase")
	}
	before, _ := runtime.SpawnGroupChaseStep(actor.EntityID)

	mux := ops.RegisterLocalSpawnGroupChaseStepEndpoint(ops.NewPprofMux("gamed"), func(entityID uint64, maxStep int32) (any, bool) {
		return runtime.StepSpawnGroupChase(entityID, maxStep)
	})
	post := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	path := "/local/spawn-groups/" + strconv.FormatUint(actor.EntityID, 10) + "/chase-step"
	first := post(path)
	if first.Code != http.StatusOK {
		t.Fatalf("engaged chase POST: status=%d body=%s", first.Code, first.Body.String())
	}
	var result struct {
		Actor struct {
			EntityID uint64 `json:"entity_id"`
			X        int32  `json:"x"`
			Y        int32  `json:"y"`
		} `json:"actor"`
		Step struct {
			Current struct {
				X int32 `json:"x"`
			} `json:"current"`
			Next struct {
				X int32 `json:"x"`
			} `json:"next"`
			Complete bool `json:"complete"`
		} `json:"step"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &result); err != nil || result.Actor.EntityID != actor.EntityID || result.Actor.X != 1800 || result.Step.Current.X != 1700 || result.Step.Next.X != 1800 || result.Step.Complete {
		t.Fatalf("wrong one-step operator response: result=%+v err=%v", result, err)
	}
	if got, ok := runtime.SpawnGroup(actor.EntityID); !ok || got.X != 1800 || got.Y != 2800 || got.Dead {
		t.Fatalf("expected exactly one live +100 chase step, ok=%v actor=%+v", ok, got)
	}
	stored, err := staticstore.NewFileStore(cfg.StaticActorStorePath).Load()
	if err != nil || len(stored.StaticActors) != 1 || stored.StaticActors[0].X != 1800 {
		t.Fatalf("expected persisted one-step movement, snapshot=%+v err=%v", stored, err)
	}
	queued, err := source.FlushServerFrames()
	if err != nil || len(queued) == 0 {
		t.Fatalf("retained owner MOVE missing: frames=%d err=%v", len(queued), err)
	}
	move, err := movep.DecodeMoveAck(chaseFrame(t, queued[0]))
	if err != nil || move.X != 1800 || move.Y != 2800 {
		t.Fatalf("expected retained MOVE at +100: move=%+v err=%v", move, err)
	}
	changeSpeed := false
	for _, raw := range queued[1:] {
		if speed, err := worldproto.DecodeChangeSpeed(chaseFrame(t, raw)); err == nil && speed.VID == uint32(actor.EntityID) && speed.MovingSpeed == worldproto.BootstrapCharacterMovingSpeed {
			changeSpeed = true
		}
	}
	if !changeSpeed {
		t.Fatalf("expected owned chase speed companion beside MOVE, queued=%d", len(queued))
	}
	after, ok := runtime.SpawnGroupChaseStep(actor.EntityID)
	if !ok || !after.ReadyAt.After(before.ReadyAt) || !after.ReadyAt.After(time.Now().Add(4*time.Second)) {
		t.Fatalf("manual chase should refresh automatic deadline: before=%+v after=%+v ok=%v", before, after, ok)
	}
	completed := post(path + "?max_step=100")
	if completed.Code != http.StatusOK {
		t.Fatalf("operator completion: status=%d body=%s", completed.Code, completed.Body.String())
	}
	if got, ok := runtime.SpawnGroup(actor.EntityID); !ok || got.X != 1900 {
		t.Fatalf("expected second manual step to reach owner, ok=%v actor=%+v", ok, got)
	}
	if pending, ok := runtime.SpawnGroupChaseStep(actor.EntityID); ok {
		t.Fatalf("completed operator chase should clear automatic deadline, pending=%+v", pending)
	}
	if rec := post(path); rec.Code != http.StatusOK {
		t.Fatalf("same-cell no-op should be accepted, status=%d", rec.Code)
	}
	if got, ok := runtime.SpawnGroup(actor.EntityID); !ok || got.X != 1900 {
		t.Fatalf("same-cell no-op moved actor, ok=%v actor=%+v", ok, got)
	}
	if _, err := source.FlushServerFrames(); err != nil {
		t.Fatalf("flush manual completion frames: %v", err)
	}
	// Once engagement is explicitly released the same operator POST must not
	// re-acquire a target or move an unengaged actor.
	if _, err := flow.HandleClientFrame(chaseFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: 0}))); err != nil {
		t.Fatalf("release target: %v", err)
	}
	if rec := post(path); rec.Code != http.StatusNotFound {
		t.Fatalf("unengaged chase should be 404, got %d", rec.Code)
	}
	if got, ok := runtime.SpawnGroup(actor.EntityID); !ok || got.X != 1900 {
		t.Fatalf("unengaged actor moved: ok=%v actor=%+v", ok, got)
	}
	// Return-required ownership belongs to return-step, never chase POST.
	if _, ok := runtime.UpdateStaticActor(actor.EntityID, "ChaseOpMob", 42, 2301, 2800, 20350); !ok {
		t.Fatal("cannot displace for return-required gate")
	}
	if rec := post(path); rec.Code != http.StatusNotFound {
		t.Fatalf("return-required chase should be 404, got %d", rec.Code)
	}
	if got, ok := runtime.SpawnGroup(actor.EntityID); !ok || got.X != 2301 {
		t.Fatalf("return-required actor moved: ok=%v actor=%+v", ok, got)
	}
	if rec := post("/local/spawn-groups/999999/chase-step"); rec.Code != http.StatusNotFound {
		t.Fatalf("missing actor chase should be 404, got %d", rec.Code)
	}
	if _, ok := runtime.ReturnSpawnGroupHome(actor.EntityID); !ok {
		t.Fatal("cannot restore home for dead gate")
	}
	if _, err := source.FlushServerFrames(); err != nil {
		t.Fatalf("flush return-home frames: %v", err)
	}
	target, err = flow.HandleClientFrame(chaseFrame(t, combatproto.EncodeClientTarget(combatproto.ClientTargetPacket{TargetVID: uint32(actor.EntityID)})))
	if err != nil || len(target) != 1 {
		t.Fatalf("fresh target for death gate: frames=%d err=%v", len(target), err)
	}
	if _, err := source.FlushServerFrames(); err != nil {
		t.Fatalf("flush fresh target frames: %v", err)
	}
	for hitIndex := 0; hitIndex < 9; hitIndex++ {
		if hitIndex != 0 {
			time.Sleep(260 * time.Millisecond) // owned normal-attack cadence
		}
		frames, err := flow.HandleClientFrame(chaseFrame(t, combatproto.EncodeClientAttack(combatproto.ClientAttackPacket{AttackType: combatproto.ClientAttackTypeNormal, TargetVID: uint32(actor.EntityID)})))
		if err != nil || len(frames) == 0 {
			t.Fatalf("accepted killing sequence hit %d: frames=%d err=%v", hitIndex+1, len(frames), err)
		}
	}
	if dead, ok := runtime.SpawnGroup(actor.EntityID); !ok || !dead.Dead {
		t.Fatalf("expected dead actor after ten accepted hits, ok=%v actor=%+v", ok, dead)
	}
	if rec := post(path); rec.Code != http.StatusNotFound {
		t.Fatalf("dead chase should be 404, got %d", rec.Code)
	}
}

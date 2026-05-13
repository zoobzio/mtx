package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // HMAC-SHA1 required by Synapse admin registration API
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/zoobzio/mtx"
)

const sharedSecret = "integration_test_secret"

var testHomeserver string

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "matrixdotorg/synapse:latest",
		ExposedPorts: []string{"8008/tcp"},
		Env: map[string]string{
			"SYNAPSE_SERVER_NAME":  "localhost",
			"SYNAPSE_REPORT_STATS": "no",
		},
		Entrypoint: []string{"sh", "-c"},
		Cmd: []string{
			`python -m synapse.app.homeserver --server-name localhost --config-path /data/homeserver.yaml --generate-config --report-stats=no && ` +
				`sed -i 's/- ::1/- 0.0.0.0/' /data/homeserver.yaml && ` +
				`sed -i '/- 127.0.0.1/d' /data/homeserver.yaml && ` +
				`printf '\nenable_registration: true\nenable_registration_without_verification: true\nregistration_shared_secret: "` + sharedSecret + `"\nsuppress_key_server_warning: true\n' >> /data/homeserver.yaml && ` +
				`exec python -m synapse.app.homeserver --config-path /data/homeserver.yaml`,
		},
		WaitingFor: wait.ForHTTP("/_matrix/client/versions").WithPort("8008").WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting synapse: %v\n", err)
		os.Exit(1)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "8008")
	testHomeserver = fmt.Sprintf("http://%s:%s", host, port.Port())

	code := m.Run()

	_ = container.Terminate(ctx)
	os.Exit(code)
}

// adminRegister creates a user via Synapse's admin registration API.
func adminRegister(t *testing.T, username, password string) *mtx.Registration {
	t.Helper()

	// Step 1: Get nonce.
	resp, err := http.Get(testHomeserver + "/_synapse/admin/v1/register")
	if err != nil {
		t.Fatalf("getting nonce: %v", err)
	}
	defer resp.Body.Close()

	var nonceResp struct {
		Nonce string `json:"nonce"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nonceResp); err != nil {
		t.Fatalf("decoding nonce: %v", err)
	}

	// Step 2: Compute HMAC-SHA1 per Synapse's admin registration protocol.
	mac := hmac.New(sha1.New, []byte(sharedSecret)) //nolint:gosec // required by Synapse
	mac.Write([]byte(nonceResp.Nonce))
	mac.Write([]byte{0})
	mac.Write([]byte(username))
	mac.Write([]byte{0})
	mac.Write([]byte(password))
	mac.Write([]byte{0})
	mac.Write([]byte("notadmin"))
	digest := hex.EncodeToString(mac.Sum(nil))

	// Step 3: Register.
	body, _ := json.Marshal(map[string]interface{}{
		"nonce":    nonceResp.Nonce,
		"username": username,
		"password": password,
		"mac":      digest,
		"admin":    false,
	})
	resp2, err := http.Post(testHomeserver+"/_synapse/admin/v1/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("registering %s: %v", username, err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("register %s: status %d: %s", username, resp2.StatusCode, b)
	}

	var reg mtx.Registration
	if err := json.NewDecoder(resp2.Body).Decode(&reg); err != nil {
		t.Fatalf("decoding registration for %s: %v", username, err)
	}
	return &reg
}

func TestLoginAndWhoAmI(t *testing.T) {
	reg := adminRegister(t, "alice", "password123")

	// Login.
	client := mtx.NewClient(testHomeserver, "")
	login, err := client.Login("alice", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.UserID != reg.UserID {
		t.Errorf("login user_id = %q, want %q", login.UserID, reg.UserID)
	}

	// WhoAmI.
	authed := mtx.NewClient(testHomeserver, login.AccessToken)
	whoami, err := authed.WhoAmI()
	if err != nil {
		t.Fatalf("whoami: %v", err)
	}
	if whoami.UserID != reg.UserID {
		t.Errorf("whoami = %q, want %q", whoami.UserID, reg.UserID)
	}
}

func TestRoomMessaging(t *testing.T) {
	reg := adminRegister(t, "bob", "password123")
	client := mtx.NewClient(testHomeserver, reg.AccessToken)

	// Create room.
	room, err := client.CreateRoom("test-room", "integration test")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if room.RoomID == "" {
		t.Fatal("room ID is empty")
	}

	// Send message.
	eventID, err := client.Send(room.RoomID, "hello world")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if eventID == "" {
		t.Fatal("event ID is empty")
	}

	// Read messages back.
	msgs, err := client.Messages(room.RoomID, 10)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	found := false
	for _, m := range msgs.Chunk {
		if body, ok := m.Content["body"].(string); ok && body == "hello world" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sent message not found in room messages")
	}

	// Joined rooms should include this room.
	rooms, err := client.JoinedRooms()
	if err != nil {
		t.Fatalf("joined rooms: %v", err)
	}
	roomFound := false
	for _, r := range rooms.Rooms {
		if r == room.RoomID {
			roomFound = true
			break
		}
	}
	if !roomFound {
		t.Error("created room not in joined rooms list")
	}

	// Room info.
	info, err := client.GetRoomInfo(room.RoomID)
	if err != nil {
		t.Fatalf("get room info: %v", err)
	}
	if info.Name != "test-room" {
		t.Errorf("room name = %q, want %q", info.Name, "test-room")
	}
	if info.Topic != "integration test" {
		t.Errorf("room topic = %q, want %q", info.Topic, "integration test")
	}

	// Members.
	members, err := client.Members(room.RoomID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("members count = %d, want 1", len(members))
	}
}

func TestInviteJoinLeave(t *testing.T) {
	regA := adminRegister(t, "charlie", "password123")
	regB := adminRegister(t, "diana", "password123")
	clientA := mtx.NewClient(testHomeserver, regA.AccessToken)
	clientB := mtx.NewClient(testHomeserver, regB.AccessToken)

	// Charlie creates a room and invites Diana.
	room, err := clientA.CreateRoom("invite-test", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := clientA.Invite(room.RoomID, regB.UserID); err != nil {
		t.Fatalf("invite: %v", err)
	}

	// Diana joins.
	joinedID, err := clientB.Join(room.RoomID)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if joinedID != room.RoomID {
		t.Errorf("joined room = %q, want %q", joinedID, room.RoomID)
	}

	// Both should be members.
	members, err := clientA.Members(room.RoomID)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("members count = %d, want 2", len(members))
	}

	// Diana sends a message, Charlie reads it.
	if _, err := clientB.Send(room.RoomID, "hi charlie"); err != nil {
		t.Fatalf("send: %v", err)
	}
	msgs, err := clientA.Messages(room.RoomID, 10)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	found := false
	for _, m := range msgs.Chunk {
		if body, ok := m.Content["body"].(string); ok && body == "hi charlie" {
			found = true
			break
		}
	}
	if !found {
		t.Error("diana's message not found")
	}

	// Diana leaves.
	if err := clientB.Leave(room.RoomID); err != nil {
		t.Fatalf("leave: %v", err)
	}
	rooms, err := clientB.JoinedRooms()
	if err != nil {
		t.Fatalf("joined rooms: %v", err)
	}
	for _, r := range rooms.Rooms {
		if r == room.RoomID {
			t.Error("diana still in room after leaving")
		}
	}
}

func TestDMFlow(t *testing.T) {
	regA := adminRegister(t, "eve", "password123")
	regB := adminRegister(t, "frank", "password123")
	clientA := mtx.NewClient(testHomeserver, regA.AccessToken)

	// Eve creates a DM room with Frank.
	room, err := clientA.CreateDMRoom(regB.UserID)
	if err != nil {
		t.Fatalf("create DM room: %v", err)
	}
	if room.RoomID == "" {
		t.Fatal("DM room ID is empty")
	}

	// Track it in m.direct.
	if err := clientA.SetDirectRooms(regA.UserID, map[string][]string{
		regB.UserID: {room.RoomID},
	}); err != nil {
		t.Fatalf("set direct rooms: %v", err)
	}

	// Read it back.
	directs, err := clientA.GetDirectRooms(regA.UserID)
	if err != nil {
		t.Fatalf("get direct rooms: %v", err)
	}
	rooms, ok := directs[regB.UserID]
	if !ok || len(rooms) == 0 {
		t.Fatal("DM room not found in m.direct")
	}
	if rooms[0] != room.RoomID {
		t.Errorf("m.direct room = %q, want %q", rooms[0], room.RoomID)
	}

	// Send a message in the DM.
	eventID, err := clientA.Send(room.RoomID, "hey frank")
	if err != nil {
		t.Fatalf("send DM: %v", err)
	}
	if eventID == "" {
		t.Fatal("DM event ID is empty")
	}
}

func TestRoomAlias(t *testing.T) {
	reg := adminRegister(t, "grace", "password123")
	client := mtx.NewClient(testHomeserver, reg.AccessToken)

	// Create room with alias.
	room, err := client.CreateRoomWithAlias("aliased-room", "room with alias", "test-alias")
	if err != nil {
		t.Fatalf("create room with alias: %v", err)
	}

	// Resolve alias.
	resolved, err := client.ResolveAlias("#test-alias:localhost")
	if err != nil {
		t.Fatalf("resolve alias: %v", err)
	}
	if resolved.RoomID != room.RoomID {
		t.Errorf("resolved room = %q, want %q", resolved.RoomID, room.RoomID)
	}
}

func TestPublicRooms(t *testing.T) {
	reg := adminRegister(t, "heidi", "password123")
	client := mtx.NewClient(testHomeserver, reg.AccessToken)

	// PublicRooms endpoint should respond without error.
	resp, err := client.PublicRooms()
	if err != nil {
		t.Fatalf("public rooms: %v", err)
	}
	// Response should be valid (may be empty on a fresh server).
	if resp == nil {
		t.Fatal("public rooms response is nil")
	}
}

func TestMessagesPagination(t *testing.T) {
	reg := adminRegister(t, "ivan", "password123")
	client := mtx.NewClient(testHomeserver, reg.AccessToken)

	room, err := client.CreateRoom("pagination-test", "")
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	// Send several messages.
	var firstEventID string
	for i := range 5 {
		eid, err := client.Send(room.RoomID, fmt.Sprintf("msg-%d", i))
		if err != nil {
			t.Fatalf("send msg-%d: %v", i, err)
		}
		if i == 0 {
			firstEventID = eid
		}
	}

	// Read messages since the first event.
	token, err := client.EventContext(room.RoomID, firstEventID)
	if err != nil {
		t.Fatalf("event context: %v", err)
	}
	msgs, err := client.MessagesFrom(room.RoomID, token, 10, "f")
	if err != nil {
		t.Fatalf("messages from: %v", err)
	}

	// Should have messages after the first one.
	if len(msgs.Chunk) == 0 {
		t.Error("no messages returned from pagination")
	}
}

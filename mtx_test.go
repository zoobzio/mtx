//go:build testing

package mtx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	mtesting "github.com/zoobzio/mtx/testing"
)

func TestClientRegister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/register")
		mtesting.AssertEqual(t, r.Method, http.MethodPost)
		json.NewEncoder(w).Encode(Registration{
			UserID:      "@agent:localhost",
			AccessToken: "tok_123",
			DeviceID:    "DEV1",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	reg, err := client.Register("agent", "pass", "mtx")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, reg.UserID, "@agent:localhost")
	mtesting.AssertEqual(t, reg.AccessToken, "tok_123")
}

func TestClientLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/login")
		json.NewEncoder(w).Encode(Registration{
			UserID:      "@operator:localhost",
			AccessToken: "tok_456",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	reg, err := client.Login("operator", "pass")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, reg.UserID, "@operator:localhost")
	mtesting.AssertEqual(t, reg.AccessToken, "tok_456")
}

func TestClientCreateRoom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/createRoom")
		mtesting.AssertEqual(t, r.Header.Get("Authorization"), "Bearer tok_abc")
		json.NewEncoder(w).Encode(Room{RoomID: "!room123:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	room, err := client.CreateRoom("general", "dev discussion")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, room.RoomID, "!room123:localhost")
}

func TestClientSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodPut)
		json.NewEncoder(w).Encode(map[string]string{"event_id": "$evt1"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	eventID, err := client.Send("!room:localhost", "hello")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, eventID, "$evt1")
}

func TestClientMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodGet)
		json.NewEncoder(w).Encode(Messages{
			Chunk: []Message{
				{Sender: "@bob:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "hi"}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	msgs, err := client.Messages("!room:localhost", 10)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, len(msgs.Chunk), 1)
	mtesting.AssertEqual(t, msgs.Chunk[0].Sender, "@bob:localhost")
}

func TestClientJoinedRooms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(JoinedRooms{Rooms: []string{"!a:localhost", "!b:localhost"}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	rooms, err := client.JoinedRooms()
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, len(rooms.Rooms), 2)
}

func TestClientErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(matrixError{ErrCode: "M_FORBIDDEN", Error: "not allowed"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.Login("x", "y")
	mtesting.AssertError(t, err)
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("MTX_TOKEN", "tok_env")
	token, err := TokenFromEnv()
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, token, "tok_env")
}

func TestTokenFromEnvMissing(t *testing.T) {
	t.Setenv("MTX_TOKEN", "")
	_, err := TokenFromEnv()
	mtesting.AssertError(t, err)
}

func TestTokenFromEnvFile(t *testing.T) {
	t.Setenv("MTX_TOKEN", "")
	dir := t.TempDir()
	mtxDir := filepath.Join(dir, ".mtx")
	_ = os.MkdirAll(mtxDir, 0o750)
	_ = os.WriteFile(filepath.Join(mtxDir, "env"), []byte("MTX_TOKEN=tok_file\n"), 0o600)
	t.Chdir(dir)
	token, err := TokenFromEnv()
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, token, "tok_file")
}

func TestServerName(t *testing.T) {
	mtesting.AssertEqual(t, ServerName("http://localhost:8008"), "localhost")
	mtesting.AssertEqual(t, ServerName("https://matrix.example.com"), "matrix.example.com")
	mtesting.AssertEqual(t, ServerName("https://matrix.example.com:8448"), "matrix.example.com")
}

func TestResolveRoomIDWithRoomID(t *testing.T) {
	resolver := func(_ string) (*AliasResponse, error) {
		t.Fatal("resolver should not be called for room IDs")
		return nil, nil
	}
	roomID, err := ResolveRoomID("!room:localhost", resolver)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, roomID, "!room:localhost")
}

func TestResolveRoomIDWithAlias(t *testing.T) {
	Homeserver = "http://localhost:8008"
	resolver := func(alias string) (*AliasResponse, error) {
		mtesting.AssertEqual(t, alias, "#general:localhost")
		return &AliasResponse{RoomID: "!abc:localhost"}, nil
	}
	roomID, err := ResolveRoomID("general", resolver)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, roomID, "!abc:localhost")
}

func TestResolveRoomIDWithFullAlias(t *testing.T) {
	resolver := func(alias string) (*AliasResponse, error) {
		mtesting.AssertEqual(t, alias, "#general:example.com")
		return &AliasResponse{RoomID: "!abc:example.com"}, nil
	}
	roomID, err := ResolveRoomID("#general:example.com", resolver)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, roomID, "!abc:example.com")
}

func TestResolveRoomIDWithUserID(t *testing.T) {
	resolver := func(_ string) (*AliasResponse, error) {
		t.Fatal("resolver should not be called for user IDs")
		return nil, nil
	}
	roomID, err := ResolveRoomID("@wintermute:localhost", resolver)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, roomID, "@wintermute:localhost")
}

func TestClientLeave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodPost)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.Leave("!room:localhost")
	mtesting.AssertNoError(t, err)
}

func TestClientGetProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodGet)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"displayname":"Alice"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.GetProfile("@alice:localhost")
	mtesting.AssertNoError(t, err)
}

func TestClientGetProfileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"User not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.GetProfile("@ghost:localhost")
	mtesting.AssertError(t, err)
}

func TestClientSetRoomAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodPut)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.SetRoomAlias("#dm-alice-bob:localhost", "!room:localhost")
	mtesting.AssertNoError(t, err)
}

func TestClientGetPresence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodGet)
		json.NewEncoder(w).Encode(PresenceResponse{
			Presence:        "online",
			CurrentlyActive: true,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	resp, err := client.GetPresence("@alice:localhost")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, resp.Presence, "online")
	mtesting.AssertEqual(t, resp.CurrentlyActive, true)
}

func TestClientEventContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"end": "tok_end"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	token, err := client.EventContext("!room:localhost", "$evt1")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, token, "tok_end")
}

func TestClientMessagesFrom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Messages{
			Chunk: []Message{
				{Sender: "@alice:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "new"}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	msgs, err := client.MessagesFrom("!room:localhost", "tok_start", 10, "f")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, len(msgs.Chunk), 1)
}

func TestClientWhoAmI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/account/whoami")
		json.NewEncoder(w).Encode(WhoAmIResponse{UserID: "@blue-vicky:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	resp, err := client.WhoAmI()
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, resp.UserID, "@blue-vicky:localhost")
}

func TestClientPublicRooms(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/publicRooms")
		json.NewEncoder(w).Encode(PublicRoomsResponse{
			Chunk: []PublicRoom{{RoomID: "!a:localhost", Name: "general"}},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	resp, err := client.PublicRooms()
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, len(resp.Chunk), 1)
}

func TestClientResolveAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(AliasResponse{RoomID: "!abc:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	resp, err := client.ResolveAlias("#general:localhost")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, resp.RoomID, "!abc:localhost")
}

func TestClientJoin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mtesting.AssertEqual(t, r.Method, http.MethodPost)
		json.NewEncoder(w).Encode(map[string]string{"room_id": "!abc:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	roomID, err := client.Join("!abc:localhost")
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, roomID, "!abc:localhost")
}

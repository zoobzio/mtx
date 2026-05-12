//go:build testing

package mtx

import (
	"fmt"
	"testing"

	mtesting "github.com/zoobzio/mtx/testing"
)

func TestRunDMSendExistingRoom(t *testing.T) {
	Homeserver = "http://localhost:8008"
	whoami := func() (*WhoAmIResponse, error) {
		return &WhoAmIResponse{UserID: "@blue-vicky:localhost"}, nil
	}
	getDirect := func(_ string) (map[string][]string, error) {
		return map[string][]string{
			"@blue-flux:localhost": {"!dm:localhost"},
		}, nil
	}
	setDirect := func(_ string, _ map[string][]string) error { return nil }

	var sentRoom, sentMsg string
	sender := func(roomID, message string) (string, error) {
		sentRoom = roomID
		sentMsg = message
		return "$evt1", nil
	}
	creator := func(_ string) (*Room, error) { return nil, fmt.Errorf("should not create") }
	checker := func(_ string) error { return nil }

	err := runDMSend("blue-flux", "hello", whoami, getDirect, setDirect, sender, creator, checker)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, sentRoom, "!dm:localhost")
	mtesting.AssertEqual(t, sentMsg, "hello")
}

func TestRunDMSendCreatesRoom(t *testing.T) {
	Homeserver = "http://localhost:8008"
	whoami := func() (*WhoAmIResponse, error) {
		return &WhoAmIResponse{UserID: "@blue-vicky:localhost"}, nil
	}
	getDirect := func(_ string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	var directsSet bool
	setDirect := func(_ string, rooms map[string][]string) error {
		directsSet = true
		return nil
	}
	sender := func(_, _ string) (string, error) { return "$evt1", nil }
	var createdInvite string
	creator := func(invite string) (*Room, error) {
		createdInvite = invite
		return &Room{RoomID: "!newdm:localhost"}, nil
	}
	checker := func(_ string) error { return nil }

	err := runDMSend("blue-flux", "hey", whoami, getDirect, setDirect, sender, creator, checker)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, createdInvite, "@blue-flux:localhost")
	mtesting.AssertEqual(t, directsSet, true)
}

func TestRunDMSendWithFullUserID(t *testing.T) {
	Homeserver = "http://localhost:8008"
	whoami := func() (*WhoAmIResponse, error) {
		return &WhoAmIResponse{UserID: "@blue-vicky:localhost"}, nil
	}
	getDirect := func(_ string) (map[string][]string, error) {
		return map[string][]string{
			"@red-flux:example.com": {"!dm:example.com"},
		}, nil
	}
	setDirect := func(_ string, _ map[string][]string) error { return nil }
	var sentRoom string
	sender := func(roomID, _ string) (string, error) {
		sentRoom = roomID
		return "$evt1", nil
	}
	creator := func(_ string) (*Room, error) { return nil, fmt.Errorf("should not create") }
	checker := func(_ string) error { return nil }

	err := runDMSend("@red-flux:example.com", "hi", whoami, getDirect, setDirect, sender, creator, checker)
	mtesting.AssertNoError(t, err)
	mtesting.AssertEqual(t, sentRoom, "!dm:example.com")
}

func TestRunDMSendWhoAmIError(t *testing.T) {
	whoami := func() (*WhoAmIResponse, error) {
		return nil, fmt.Errorf("unauthorized")
	}
	err := runDMSend("someone", "hi", whoami, nil, nil, nil, nil, nil)
	mtesting.AssertError(t, err)
}

func TestRunDMSendNonexistentUser(t *testing.T) {
	Homeserver = "http://localhost:8008"
	whoami := func() (*WhoAmIResponse, error) {
		return &WhoAmIResponse{UserID: "@blue-vicky:localhost"}, nil
	}
	getDirect := func(_ string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}
	setDirect := func(_ string, _ map[string][]string) error { return nil }
	sender := func(_, _ string) (string, error) { return "$evt1", nil }
	creator := func(_ string) (*Room, error) { return nil, fmt.Errorf("should not create") }
	checker := func(_ string) error { return fmt.Errorf("not found") }

	err := runDMSend("ghost", "hello", whoami, getDirect, setDirect, sender, creator, checker)
	mtesting.AssertError(t, err)
}

func TestRunDMReadSuccess(t *testing.T) {
	Homeserver = "http://localhost:8008"
	whoami := func() (*WhoAmIResponse, error) {
		return &WhoAmIResponse{UserID: "@blue-vicky:localhost"}, nil
	}
	getDirect := func(_ string) (map[string][]string, error) {
		return map[string][]string{
			"@blue-flux:localhost": {"!dm:localhost"},
		}, nil
	}
	reader := func(roomID string, limit int) (*Messages, error) {
		mtesting.AssertEqual(t, roomID, "!dm:localhost")
		return &Messages{
			Chunk: []Message{
				{Sender: "@blue-flux:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "hey"}},
			},
		}, nil
	}

	err := runDMRead("blue-flux", 20, false, whoami, getDirect, reader)
	mtesting.AssertNoError(t, err)
}

func TestRunDMReadNoDMRoom(t *testing.T) {
	Homeserver = "http://localhost:8008"
	whoami := func() (*WhoAmIResponse, error) {
		return &WhoAmIResponse{UserID: "@blue-vicky:localhost"}, nil
	}
	getDirect := func(_ string) (map[string][]string, error) {
		return map[string][]string{}, nil
	}

	err := runDMRead("ghost", 20, false, whoami, getDirect, nil)
	mtesting.AssertError(t, err)
}

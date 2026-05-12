package mtx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	watchCmd.Flags().Int("timeout", 30, "seconds to wait before giving up")
	watchCmd.Flags().BoolP("follow", "f", false, "stream messages continuously")
	watchCmd.Flags().Bool("json", false, "output messages as JSON")
	rootCmd.AddCommand(watchCmd)
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch all joined rooms for new messages",
	Long:  "Block until a new message arrives in any joined room, print it, and exit.\nUse --follow to stream continuously across all rooms.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		follow, _ := cmd.Flags().GetBool("follow")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		return runWatch(timeout, follow, jsonFlag, client.Sync, client.GetRoomInfo)
	},
}

type watchMessage struct {
	Type     string `json:"type"`
	RoomID   string `json:"room_id"`
	RoomName string `json:"room_name,omitempty"`
	Sender   string `json:"sender"`
	Body     string `json:"body"`
	EventID  string `json:"event_id,omitempty"`
}

// parseInvites extracts invite info from a sync response's invite map.
func parseInvites(invites map[string]SyncInvitedRoom) []inviteInfo {
	out := make([]inviteInfo, 0, len(invites))
	for roomID, room := range invites {
		inv := inviteInfo{RoomID: roomID}
		for _, ev := range room.InviteState.Events {
			switch ev.Type {
			case "m.room.name":
				if name, ok := ev.Content["name"].(string); ok {
					inv.Name = name
				}
			case "m.room.member":
				if membership, ok := ev.Content["membership"].(string); ok && membership == "invite" {
					inv.Sender = ev.Sender
				}
			}
		}
		out = append(out, inv)
	}
	return out
}

// pollInterval is the maximum duration for a single sync request.
// The overall deadline is controlled by the context; this just caps
// how long one HTTP round-trip can block so that incoming messages
// are detected promptly even if the server ignores early returns.
const pollInterval = 5

type syncFunc func(ctx context.Context, since string, timeout int, roomID string) (*SyncResponse, error)

func runWatch(timeout int, follow, jsonOut bool, sync syncFunc, getInfo RoomInfoGetter) error {
	ctx := context.Background()
	if !follow && timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second+5*time.Second)
		defer cancel()
	}

	// Initial sync (no room filter = all rooms).
	resp, err := sync(ctx, "", 0, "")
	if err != nil {
		return fmt.Errorf("initial sync: %w", err)
	}

	// Cache room names.
	roomNames := map[string]string{}
	lookupName := func(roomID string) string {
		if name, ok := roomNames[roomID]; ok {
			return name
		}
		if getInfo != nil {
			if info, getErr := getInfo(roomID); getErr == nil && info.Name != "" {
				roomNames[roomID] = info.Name
				return info.Name
			}
		}
		roomNames[roomID] = roomID
		return roomID
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	for {
		resp, err = sync(ctx, resp.NextBatch, pollInterval, "")
		if err != nil {
			if ctx.Err() != nil && !follow {
				return fmt.Errorf("no new messages within timeout")
			}
			return fmt.Errorf("sync: %w", err)
		}

		found := false
		for roomID, room := range resp.Rooms.Join {
			for _, m := range room.Timeline.Events {
				if m.Type != msgTypeRoomMessage {
					continue
				}
				found = true
				body, _ := m.Content["body"].(string)
				if jsonOut {
					if err := enc.Encode(watchMessage{
						Type:     "message",
						RoomID:   roomID,
						RoomName: lookupName(roomID),
						Sender:   m.Sender,
						Body:     body,
						EventID:  m.EventID,
					}); err != nil {
						return fmt.Errorf("encoding message: %w", err)
					}
				} else {
					fmt.Printf("[%s] %s: %s\n", lookupName(roomID), m.Sender, body)
				}
			}
		}

		for _, inv := range parseInvites(resp.Rooms.Invite) {
			found = true
			name := inv.Name
			if name == "" {
				name = inv.RoomID
			}
			sender := inv.Sender
			if sender == "" {
				sender = unknownPlaceholder
			}
			if jsonOut {
				if err := enc.Encode(watchMessage{
					Type:     "invite",
					RoomID:   inv.RoomID,
					RoomName: name,
					Sender:   sender,
					Body:     fmt.Sprintf("invited you to %s", name),
				}); err != nil {
					return fmt.Errorf("encoding invite: %w", err)
				}
			} else {
				fmt.Printf("[invite] %s invited you to %s\n", sender, name)
			}
		}

		if !follow {
			if found {
				return nil
			}
			if ctx.Err() != nil {
				return fmt.Errorf("no new messages within timeout")
			}
		}
	}
}

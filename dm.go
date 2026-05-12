package mtx

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var dmCmd = &cobra.Command{
	Use:   "dm",
	Short: "Direct message commands",
	Long:  "Send and read direct messages by username.",
}

var dmSendCmd = &cobra.Command{
	Use:   "send <user> <message...>",
	Short: "Send a direct message to a user",
	Long:  "Send a direct message by username. Creates a DM room if one doesn't exist.",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		message := strings.Join(args[1:], " ")
		if message == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			message = strings.TrimRight(string(data), "\n")
		}
		return runDMSend(args[0], message, client.WhoAmI, client.GetDirectRooms, client.SetDirectRooms, client.Send, client.CreateDMRoom, client.GetProfile)
	},
}

var dmReadCmd = &cobra.Command{
	Use:   "read <user>",
	Short: "Read DM history with a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")
		token, err := TokenFromEnv()
		if err != nil {
			return err
		}
		client := NewClient(Homeserver, token)
		return runDMRead(args[0], limit, jsonFlag, client.WhoAmI, client.GetDirectRooms, client.Messages)
	},
}

func init() {
	dmReadCmd.Flags().IntP("limit", "n", 20, "number of messages to retrieve")
	dmReadCmd.Flags().Bool("json", false, "output messages as JSON")
	dmCmd.AddCommand(dmSendCmd)
	dmCmd.AddCommand(dmReadCmd)
	rootCmd.AddCommand(dmCmd)
}

type directRoomGetter func(userID string) (map[string][]string, error)
type directRoomSetter func(userID string, rooms map[string][]string) error
type profileChecker func(userID string) error

func resolveUserID(target string) string {
	if strings.HasPrefix(target, "@") {
		return target
	}
	return "@" + target + ":" + ServerName(Homeserver)
}

func runDMSend(target, message string, whoami WhoAmIGetter, getDirect directRoomGetter, setDirect directRoomSetter, send MessageSender, createDM DMRoomCreator, checkProfile profileChecker) error {
	targetID := resolveUserID(target)

	me, err := whoami()
	if err != nil {
		return fmt.Errorf("getting identity: %w", err)
	}

	directs, err := getDirect(me.UserID)
	if err != nil {
		return fmt.Errorf("getting direct rooms: %w", err)
	}

	var roomID string
	if rooms, ok := directs[targetID]; ok && len(rooms) > 0 {
		roomID = rooms[0]
	}

	if roomID == "" {
		if profileErr := checkProfile(targetID); profileErr != nil {
			return fmt.Errorf("user %s does not exist", targetID)
		}
		room, createErr := createDM(targetID)
		if createErr != nil {
			return fmt.Errorf("creating DM room: %w", createErr)
		}
		roomID = room.RoomID

		directs[targetID] = []string{roomID}
		if setErr := setDirect(me.UserID, directs); setErr != nil {
			return fmt.Errorf("updating direct rooms: %w", setErr)
		}
	}

	eventID, err := send(roomID, message)
	if err != nil {
		return err
	}
	fmt.Println(eventID)
	return nil
}

func runDMRead(target string, limit int, jsonOut bool, whoami WhoAmIGetter, getDirect directRoomGetter, read MessageReader) error {
	targetID := resolveUserID(target)

	me, err := whoami()
	if err != nil {
		return fmt.Errorf("getting identity: %w", err)
	}

	directs, err := getDirect(me.UserID)
	if err != nil {
		return fmt.Errorf("getting direct rooms: %w", err)
	}

	rooms, ok := directs[targetID]
	if !ok || len(rooms) == 0 {
		return fmt.Errorf("no DM room with %s", targetID)
	}

	roomID := rooms[0]
	if jsonOut {
		return runReadJSON(roomID, limit, read)
	}
	return runRead(roomID, limit, read)
}

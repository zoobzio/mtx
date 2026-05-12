// Package mtx provides a standalone Matrix messaging CLI for agentic coordination.
package mtx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// msgTypeRoomMessage is the Matrix event type for room messages.
const msgTypeRoomMessage = "m.room.message"

const unknownPlaceholder = "unknown"

// Package-level config set via PersistentPreRunE from the loaded Config.
var (
	Homeserver        string
	RegistrationToken string
	DataDir           string
)

// Config holds mtx configuration.
type Config struct {
	Homeserver        string `yaml:"homeserver"`
	RegistrationToken string `yaml:"registration_token"`
	DataDir           string `yaml:"data_dir"`
}

// configDir returns the directory to load config from.
func configDir() string {
	if dir := os.Getenv("MTX_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "mtx")
}

// loadConfig reads the mtx configuration file.
func loadConfig() (*Config, error) {
	dir := configDir()
	if dir == "" {
		return &Config{}, nil
	}
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(filepath.Clean(path))
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

var rootCmd = &cobra.Command{
	Use:   "mtx",
	Short: "Matrix messaging CLI for agentic coordination",
	Long:  "Standalone Matrix messaging CLI for agent-to-agent messaging over the Matrix protocol.",
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if cfg.Homeserver != "" {
			Homeserver = cfg.Homeserver
		}
		if cfg.RegistrationToken != "" {
			RegistrationToken = cfg.RegistrationToken
		}
		if cfg.DataDir != "" {
			DataDir = cfg.DataDir
		}
		return nil
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// Client performs HTTP requests against a Matrix homeserver.
type Client struct {
	HTTP        *http.Client
	Homeserver  string
	AccessToken string
}

// NewClient creates a Matrix client for the given homeserver and access token.
func NewClient(homeserver, accessToken string) *Client {
	return &Client{
		Homeserver:  strings.TrimRight(homeserver, "/"),
		AccessToken: accessToken,
		HTTP:        http.DefaultClient,
	}
}

// --- Response types ---

// Registration is returned by register and login endpoints.
type Registration struct {
	UserID      string `json:"user_id"`
	AccessToken string `json:"access_token"`
	DeviceID    string `json:"device_id"`
}

// Room is returned when creating a room.
type Room struct {
	RoomID string `json:"room_id"`
}

// JoinedRooms is returned when listing joined rooms.
type JoinedRooms struct {
	Rooms []string `json:"joined_rooms"`
}

// RoomInfo holds a room's name and topic.
type RoomInfo struct {
	Name  string `json:"name"`
	Topic string `json:"topic"`
}

// Member represents a room member with their display name.
type Member struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
}

// Message represents a single Matrix timeline event.
type Message struct {
	Sender  string                 `json:"sender"`
	Type    string                 `json:"type"`
	Content map[string]interface{} `json:"content"`
	EventID string                 `json:"event_id"`
}

// Messages is the response from the room messages endpoint.
type Messages struct {
	End   string    `json:"end"`
	Chunk []Message `json:"chunk"`
}

// --- Dependency types for testability ---

// Registerer registers a Matrix user account.
type Registerer func(username, password, token string) (*Registration, error)

// Authenticator logs into a Matrix account.
type Authenticator func(username, password string) (*Registration, error)

// RoomCreator creates a Matrix room.
type RoomCreator func(name, topic string) (*Room, error)

// DMRoomCreator creates a direct-message room with an invited user.
type DMRoomCreator func(invite string) (*Room, error)

// Inviter invites a user to a Matrix room.
type Inviter func(roomID, userID string) error

// MessageSender sends a message to a Matrix room.
type MessageSender func(roomID, message string) (string, error)

// MessageReader reads messages from a Matrix room.
type MessageReader func(roomID string, limit int) (*Messages, error)

// RoomLister lists joined Matrix rooms.
type RoomLister func() (*JoinedRooms, error)

// RoomInfoGetter retrieves room name and topic.
type RoomInfoGetter func(roomID string) (*RoomInfo, error)

// MemberLister lists the members of a room.
type MemberLister func(roomID string) ([]Member, error)

// WhoAmIGetter retrieves the current user's identity.
type WhoAmIGetter func() (*WhoAmIResponse, error)

// PublicRoomLister lists public rooms on the server.
type PublicRoomLister func() (*PublicRoomsResponse, error)

// AliasResolver resolves a room alias to a room ID.
type AliasResolver func(alias string) (*AliasResponse, error)

// RoomJoiner joins a room by ID or alias.
type RoomJoiner func(roomIDOrAlias string) (string, error)

// RoomLeaver leaves a room.
type RoomLeaver func(roomID string) error

// ProfileChecker checks if a user profile exists.
type ProfileChecker func(userID string) error

// RoomAliasCreator registers a room alias.
type RoomAliasCreator func(alias, roomID string) error

// PresenceGetter retrieves user presence status.
type PresenceGetter func(userID string) (*PresenceResponse, error)

// EventContextGetter returns a pagination token after a given event.
type EventContextGetter func(roomID, eventID string) (string, error)

// MessageFromReader reads messages starting from a pagination token.
type MessageFromReader func(roomID, from string, limit int, dir string) (*Messages, error)

// --- Additional response types ---

// WhoAmIResponse is returned by the whoami endpoint.
type WhoAmIResponse struct {
	UserID string `json:"user_id"`
}

// PublicRoom represents a room in the public room listing.
type PublicRoom struct {
	RoomID         string `json:"room_id"`
	Name           string `json:"name"`
	Topic          string `json:"topic"`
	CanonicalAlias string `json:"canonical_alias"`
	NumJoined      int    `json:"num_joined_members"`
}

// PublicRoomsResponse is the response from the public rooms endpoint.
type PublicRoomsResponse struct {
	Chunk []PublicRoom `json:"chunk"`
}

// AliasResponse is returned when resolving a room alias.
type AliasResponse struct {
	RoomID  string   `json:"room_id"`
	Servers []string `json:"servers"`
}

// PresenceResponse is returned by the presence endpoint.
type PresenceResponse struct {
	Presence        string `json:"presence"`
	LastActiveAgo   int    `json:"last_active_ago"`
	CurrentlyActive bool   `json:"currently_active"`
}

// SyncResponse is a simplified Matrix sync response.
type SyncResponse struct {
	Rooms     SyncRooms `json:"rooms"`
	NextBatch string    `json:"next_batch"`
}

// SyncRooms contains room data from a sync response.
type SyncRooms struct {
	Join   map[string]SyncJoinedRoom  `json:"join"`
	Invite map[string]SyncInvitedRoom `json:"invite"`
}

// SyncJoinedRoom contains timeline data for a joined room.
type SyncJoinedRoom struct {
	Timeline SyncTimeline `json:"timeline"`
}

// SyncInvitedRoom contains invite state from a sync response.
type SyncInvitedRoom struct {
	InviteState SyncInviteState `json:"invite_state"`
}

// SyncInviteState contains stripped state events for an invited room.
type SyncInviteState struct {
	Events []Message `json:"events"`
}

// SyncTimeline contains timeline events from a sync.
type SyncTimeline struct {
	Events []Message `json:"events"`
}

// --- Client methods ---

// Register creates a new Matrix user account.
func (c *Client) Register(username, password, token string) (*Registration, error) {
	body := map[string]interface{}{
		"auth": map[string]interface{}{
			"type":  "m.login.registration_token",
			"token": token,
		},
		"username": username,
		"password": password,
	}
	var reg Registration
	if err := c.post("/_matrix/client/v3/register", body, &reg); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	return &reg, nil
}

// Login authenticates and returns an access token.
func (c *Client) Login(username, password string) (*Registration, error) {
	body := map[string]interface{}{
		"type":     "m.login.password",
		"user":     username,
		"password": password,
	}
	var reg Registration
	if err := c.post("/_matrix/client/v3/login", body, &reg); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	return &reg, nil
}

// CreateRoom creates a new Matrix room with an optional topic.
func (c *Client) CreateRoom(name, topic string) (*Room, error) {
	body := map[string]interface{}{
		"name":   name,
		"preset": "private_chat",
	}
	if topic != "" {
		body["topic"] = topic
	}
	var room Room
	if err := c.post("/_matrix/client/v3/createRoom", body, &room); err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return &room, nil
}

// CreateDMRoom creates a direct-message room and invites the target user.
// Uses trusted_private_chat preset so the invited user can join immediately.
func (c *Client) CreateDMRoom(invite string) (*Room, error) {
	body := map[string]interface{}{
		"preset":    "trusted_private_chat",
		"is_direct": true,
		"invite":    []string{invite},
	}
	var room Room
	if err := c.post("/_matrix/client/v3/createRoom", body, &room); err != nil {
		return nil, fmt.Errorf("create DM room: %w", err)
	}
	return &room, nil
}

// Invite invites a user to a room.
func (c *Client) Invite(roomID, userID string) error {
	body := map[string]interface{}{
		"user_id": userID,
	}
	if err := c.post(fmt.Sprintf("/_matrix/client/v3/rooms/%s/invite", escapePathParam(roomID)), body, nil); err != nil {
		return fmt.Errorf("invite: %w", err)
	}
	return nil
}

// Send sends a text message to a room and returns the event ID.
func (c *Client) Send(roomID, message string) (string, error) {
	txnID := fmt.Sprintf("%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"msgtype": "m.text",
		"body":    message,
	}
	var resp struct {
		EventID string `json:"event_id"`
	}
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", escapePathParam(roomID), txnID)
	if err := c.put(path, body, &resp); err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	return resp.EventID, nil
}

// Messages retrieves recent messages from a room.
func (c *Client) Messages(roomID string, limit int) (*Messages, error) {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/messages?dir=b&limit=%d", escapePathParam(roomID), limit)
	var msgs Messages
	if err := c.get(path, &msgs); err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}
	return &msgs, nil
}

// JoinedRooms returns the list of rooms the user has joined.
func (c *Client) JoinedRooms() (*JoinedRooms, error) {
	var rooms JoinedRooms
	if err := c.get("/_matrix/client/v3/joined_rooms", &rooms); err != nil {
		return nil, fmt.Errorf("joined rooms: %w", err)
	}
	return &rooms, nil
}

// GetRoomInfo retrieves the name and topic for a room.
func (c *Client) GetRoomInfo(roomID string) (*RoomInfo, error) {
	var info RoomInfo

	// Fetch room name (ignore errors — name may not be set).
	var nameEvent struct {
		Name string `json:"name"`
	}
	namePath := fmt.Sprintf("/_matrix/client/v3/rooms/%s/state/m.room.name", escapePathParam(roomID))
	if err := c.get(namePath, &nameEvent); err == nil {
		info.Name = nameEvent.Name
	}

	// Fetch room topic (ignore errors — topic may not be set).
	var topicEvent struct {
		Topic string `json:"topic"`
	}
	topicPath := fmt.Sprintf("/_matrix/client/v3/rooms/%s/state/m.room.topic", escapePathParam(roomID))
	if err := c.get(topicPath, &topicEvent); err == nil {
		info.Topic = topicEvent.Topic
	}

	return &info, nil
}

// Members returns the joined members of a room with display names.
func (c *Client) Members(roomID string) ([]Member, error) {
	var resp struct {
		Joined map[string]struct {
			DisplayName string `json:"display_name"`
		} `json:"joined"`
	}
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/joined_members", escapePathParam(roomID))
	if err := c.get(path, &resp); err != nil {
		return nil, fmt.Errorf("members: %w", err)
	}

	members := make([]Member, 0, len(resp.Joined))
	for userID, info := range resp.Joined {
		members = append(members, Member{
			UserID:      userID,
			DisplayName: info.DisplayName,
		})
	}
	return members, nil
}

// WhoAmI returns the user ID for the current access token.
func (c *Client) WhoAmI() (*WhoAmIResponse, error) {
	var resp WhoAmIResponse
	if err := c.get("/_matrix/client/v3/account/whoami", &resp); err != nil {
		return nil, fmt.Errorf("whoami: %w", err)
	}
	return &resp, nil
}

// PublicRooms returns public rooms on the server.
func (c *Client) PublicRooms() (*PublicRoomsResponse, error) {
	var resp PublicRoomsResponse
	if err := c.get("/_matrix/client/v3/publicRooms", &resp); err != nil {
		return nil, fmt.Errorf("public rooms: %w", err)
	}
	return &resp, nil
}

// ResolveAlias resolves a room alias to a room ID.
func (c *Client) ResolveAlias(alias string) (*AliasResponse, error) {
	path := fmt.Sprintf("/_matrix/client/v3/directory/room/%s", escapePathParam(alias))
	var resp AliasResponse
	if err := c.get(path, &resp); err != nil {
		return nil, fmt.Errorf("resolve alias: %w", err)
	}
	return &resp, nil
}

// Join joins a room by ID or alias.
func (c *Client) Join(roomIDOrAlias string) (string, error) {
	var resp struct {
		RoomID string `json:"room_id"`
	}
	path := fmt.Sprintf("/_matrix/client/v3/join/%s", escapePathParam(roomIDOrAlias))
	if err := c.post(path, map[string]interface{}{}, &resp); err != nil {
		return "", fmt.Errorf("join: %w", err)
	}
	return resp.RoomID, nil
}

// Leave leaves a room.
func (c *Client) Leave(roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/leave", escapePathParam(roomID))
	if err := c.post(path, map[string]interface{}{}, nil); err != nil {
		return fmt.Errorf("leave: %w", err)
	}
	return nil
}

// GetProfile retrieves a user's profile. Returns an error if the user does not exist.
func (c *Client) GetProfile(userID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/profile/%s", escapePathParam(userID))
	return c.get(path, nil)
}

// SetRoomAlias registers an alias for a room.
func (c *Client) SetRoomAlias(alias, roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/directory/room/%s", escapePathParam(alias))
	body := map[string]interface{}{
		"room_id": roomID,
	}
	return c.put(path, body, nil)
}

// GetPresence retrieves a user's presence status.
func (c *Client) GetPresence(userID string) (*PresenceResponse, error) {
	path := fmt.Sprintf("/_matrix/client/v3/presence/%s/status", escapePathParam(userID))
	var resp PresenceResponse
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EventContext returns the pagination token after a given event.
func (c *Client) EventContext(roomID, eventID string) (string, error) {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/context/%s?limit=0",
		escapePathParam(roomID), escapePathParam(eventID))
	var resp struct {
		End string `json:"end"`
	}
	if err := c.get(path, &resp); err != nil {
		return "", fmt.Errorf("event context: %w", err)
	}
	return resp.End, nil
}

// MessagesFrom retrieves messages starting from a pagination token.
func (c *Client) MessagesFrom(roomID, from string, limit int, dir string) (*Messages, error) {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/messages?dir=%s&from=%s&limit=%d",
		escapePathParam(roomID), dir, url.QueryEscape(from), limit)
	var msgs Messages
	if err := c.get(path, &msgs); err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}
	return &msgs, nil
}

// CreateRoomWithAlias creates a room with a canonical alias.
func (c *Client) CreateRoomWithAlias(name, topic, aliasName string) (*Room, error) {
	body := map[string]interface{}{
		"name":            name,
		"preset":          "public_chat",
		"room_alias_name": aliasName,
	}
	if topic != "" {
		body["topic"] = topic
	}
	var room Room
	if err := c.post("/_matrix/client/v3/createRoom", body, &room); err != nil {
		return nil, fmt.Errorf("create room: %w", err)
	}
	return &room, nil
}

// Sync performs a Matrix sync request. timeout is in seconds.
func (c *Client) Sync(ctx context.Context, since string, timeout int, roomID string) (*SyncResponse, error) {
	q := url.Values{}
	q.Set("timeout", fmt.Sprintf("%d", timeout*1000))
	if since != "" {
		q.Set("since", since)
	}
	if roomID != "" {
		filter := fmt.Sprintf(`{"room":{"rooms":[%q],"timeline":{"limit":10}}}`, roomID)
		q.Set("filter", filter)
	}
	path := "/_matrix/client/v3/sync?" + q.Encode()
	var resp SyncResponse
	if err := c.doCtx(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("sync: %w", err)
	}
	return &resp, nil
}

// GetDirectRooms retrieves the m.direct account data for the user.
func (c *Client) GetDirectRooms(userID string) (map[string][]string, error) {
	path := fmt.Sprintf("/_matrix/client/v3/user/%s/account_data/m.direct", escapePathParam(userID))
	var resp map[string][]string
	if err := c.get(path, &resp); err != nil {
		return map[string][]string{}, nil
	}
	return resp, nil
}

// SetDirectRooms updates the m.direct account data for the user.
func (c *Client) SetDirectRooms(userID string, rooms map[string][]string) error {
	path := fmt.Sprintf("/_matrix/client/v3/user/%s/account_data/m.direct", escapePathParam(userID))
	return c.put(path, rooms, nil)
}

// escapePathParam percent-encodes a value for use in a URL path segment.
// Go's url.PathEscape leaves ':' unencoded (valid per RFC 3986) but some
// Matrix homeservers require it encoded for room IDs like !foo:localhost.
func escapePathParam(s string) string {
	return strings.ReplaceAll(url.PathEscape(s), ":", "%3A")
}

// --- HTTP helpers ---

func (c *Client) post(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPost, path, body, result)
}

func (c *Client) put(path string, body interface{}, result interface{}) error {
	return c.do(http.MethodPut, path, body, result)
}

func (c *Client) get(path string, result interface{}) error {
	return c.do(http.MethodGet, path, nil, result)
}

type matrixError struct {
	ErrCode string `json:"errcode"`
	Error   string `json:"error"`
}

func (c *Client) do(method, path string, body interface{}, result interface{}) error {
	return c.doCtx(context.Background(), method, path, body, result)
}

func (c *Client) doCtx(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshalling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.Homeserver+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var mErr matrixError
		if json.Unmarshal(respBody, &mErr) == nil && mErr.Error != "" {
			return fmt.Errorf("%s (%s)", mErr.Error, mErr.ErrCode)
		}
		return fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// TokenFromEnv reads the Matrix access token. It checks MTX_TOKEN first,
// then walks up from CWD looking for .mtx/env files.
func TokenFromEnv() (string, error) {
	if token := os.Getenv("MTX_TOKEN"); token != "" {
		return token, nil
	}
	if token := envFromFile("MTX_TOKEN"); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("MTX_TOKEN not set and no .mtx/env found")
}

// TeamFromEnv reads the team name from MTX_TEAM or .mtx/env files.
func TeamFromEnv() (string, error) {
	if team := os.Getenv("MTX_TEAM"); team != "" {
		return team, nil
	}
	if team := envFromFile("MTX_TEAM"); team != "" {
		return team, nil
	}
	return "", fmt.Errorf("MTX_TEAM not set and no .mtx/env found")
}

// envFromFile walks up from CWD looking for .mtx/env and reads
// a KEY=VALUE entry matching the given key.
func envFromFile(key string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, ".mtx", "env")
		data, err := os.ReadFile(filepath.Clean(path))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				if k, v, ok := strings.Cut(line, "="); ok && k == key {
					return v
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ResolveRoomID resolves a room argument that may be a room ID, full alias, or
// short alias name into a room ID.
func ResolveRoomID(arg string, resolve AliasResolver) (string, error) {
	arg = strings.TrimPrefix(arg, "\\")
	if strings.HasPrefix(arg, "!") || strings.HasPrefix(arg, "@") {
		return arg, nil
	}
	alias := arg
	if !strings.HasPrefix(alias, "#") {
		alias = "#" + alias + ":" + ServerName(Homeserver)
	}
	resp, err := resolve(alias)
	if err != nil {
		return "", fmt.Errorf("resolving %q: %w", alias, err)
	}
	return resp.RoomID, nil
}

// ServerName extracts the hostname from a homeserver URL.
func ServerName(homeserver string) string {
	u, err := url.Parse(homeserver)
	if err != nil {
		return homeserver
	}
	host := u.Hostname()
	if host == "" {
		return homeserver
	}
	return host
}

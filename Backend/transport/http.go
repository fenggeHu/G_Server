package transport

import (
	"aigame/server/backend/domain"
	"aigame/server/backend/usecase"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Handler struct {
	Service            *usecase.Service
	AllowHTTP          bool
	AllowDevDockerHTTP bool
	mu                 sync.Mutex
	windows            map[string][]time.Time
}

func New(s *usecase.Service, allow bool, options ...bool) *Handler {
	devDocker := len(options) > 0 && options[0]
	return &Handler{Service: s, AllowHTTP: allow, AllowDevDockerHTTP: devDocker, windows: make(map[string][]time.Time)}
}
func (h *Handler) slot(key string, limit int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	for k, v := range h.windows {
		if now.Sub(v[len(v)-1]) >= time.Minute {
			delete(h.windows, k)
		}
	}
	v, ok := h.windows[key]
	if !ok && len(h.windows) >= 2048 {
		return false
	}
	for len(v) > 0 && now.Sub(v[0]) >= time.Minute {
		v = v[1:]
	}
	if len(v) >= limit {
		return false
	}
	h.windows[key] = append(v, now)
	return true
}
func response(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func failure(w http.ResponseWriter, status int, s string) {
	response(w, status, map[string]string{"detail": s})
}

func serviceAuthorized(header, expected string) bool {
	const scheme = "Bearer "
	if len(header) != len(scheme)+len(expected) || !strings.HasPrefix(header, scheme) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header[len(scheme):]), []byte(expected)) == 1
}
func dbError(w http.ResponseWriter, e error) {
	if errors.Is(e, pgx.ErrNoRows) || errors.Is(e, usecase.ErrUnauthorized) {
		failure(w, 401, "unauthorized")
	} else {
		failure(w, 503, "database unavailable")
	}
}
func text(x string, max int) bool {
	return utf8.ValidString(x) && utf8.RuneCountInString(x) > 0 && utf8.RuneCountInString(x) <= max
}
func integer(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0) && x >= 0 && x <= math.MaxInt64 && math.Trunc(x) == x
}

type login struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type ticketReq struct {
	Preset  string `json:"preset_id"`
	Proto   int    `json:"protocol_version"`
	Content string `json:"gameplay_content_hash"`
}
type redeemReq struct {
	Ticket  string `json:"ticket"`
	Room    string `json:"room_id"`
	Preset  string `json:"preset_id"`
	Proto   int    `json:"protocol_version"`
	Content string `json:"gameplay_content_hash"`
	Config  string `json:"resolved_config_hash"`
}
type leaseReq struct {
	PlayerID     string  `json:"player_id"`
	RoomID       string  `json:"room_id"`
	FencingToken float64 `json:"fencing_token"`
}
type commitReq struct {
	PlayerID         string          `json:"player_id"`
	RoomID           string          `json:"room_id"`
	FencingToken     float64         `json:"fencing_token"`
	ExpectedRevision float64         `json:"expected_revision"`
	OperationID      string          `json:"operation_id"`
	Payload          json.RawMessage `json:"payload"`
}
type equipmentReq struct {
	PlayerID string `json:"player_id"`
	RoomID string `json:"room_id"`
	FencingToken float64 `json:"fencing_token"`
	ExpectedRevision float64 `json:"expected_revision"`
	OperationID string `json:"operation_id"`
	Slot string `json:"slot"`
	ItemID string `json:"item_id"`
	Equipped bool `json:"equipped"`
}

func decode(b []byte, v any) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return false
	}
	return d.Decode(new(any)) == io.EOF
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	private := net.ParseIP(ip) != nil && (net.ParseIP(ip).IsLoopback() || net.ParseIP(ip).IsPrivate())
	if r.TLS == nil && !(h.AllowHTTP && net.ParseIP(ip) != nil && net.ParseIP(ip).IsLoopback()) && !(h.AllowDevDockerHTTP && private) {
		failure(w, 400, "https required")
		return
	}
	limits := map[string]int{"/v1/auth/login": 5, "/v1/auth/logout": 30, "/v1/tickets": 30, "/v1/rooms/allocate": 30, "/v1/internal/tickets/redeem": 60, "/v1/internal/rooms/register": 60, "/v1/internal/rooms/heartbeat": 120, "/v1/internal/rooms/release": 60, "/v1/internal/progress/acquire": 60, "/v1/internal/progress/renew": 120, "/v1/internal/progress/release": 60, "/v1/internal/progress/commit": 120, "/v1/internal/progress/query": 120, "/v1/internal/progress/snapshot": 60, "/v1/internal/progress/equipment": 60, "/v1/internal/progress/quests": 60}
	if n := limits[r.URL.Path]; n > 0 && !h.slot(r.URL.Path+"|"+ip, n) {
		w.Header().Set("Retry-After", "60")
		failure(w, 429, "rate limited")
		return
	}
	if v := r.Header.Values("Content-Length"); len(v) > 0 {
		n, e := strconv.ParseUint(v[0], 10, 64)
		if e != nil || len(v) != 1 {
			failure(w, 400, "invalid content length")
			return
		}
		if n > 8192 {
			failure(w, 413, "request too large")
			return
		}
	}
	if r.ContentLength > 8192 {
		failure(w, 413, "request too large")
		return
	}
	b, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
	if e != nil {
		var size *http.MaxBytesError
		if errors.As(e, &size) {
			failure(w, 413, "request too large")
		} else {
			failure(w, 400, "invalid request")
		}
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	path := r.URL.Path
	if path == "/health/live" || path == "/health/ready" {
		if r.Method != "GET" {
			failure(w, 405, "Method Not Allowed")
			return
		}
		if path == "/health/ready" {
			if h.Service.Store.Ping(ctx) != nil {
				failure(w, 503, "not ready")
				return
			}
			response(w, 200, map[string]string{"status": "ready"})
		} else {
			response(w, 200, map[string]string{"status": "ok"})
		}
		return
	}
	if limits[path] == 0 {
		failure(w, 404, "Not Found")
		return
	}
	if r.Method != "POST" {
		failure(w, 405, "Method Not Allowed")
		return
	}
	if path == "/v1/auth/login" {
		var x login
		if !decode(b, &x) || !text(x.Username, 64) || !text(x.Password, 256) {
			failure(w, 422, "invalid request")
			return
		}
		t, id, e := h.Service.Login(ctx, x.Username, x.Password)
		if e != nil {
			dbError(w, e)
			return
		}
		response(w, 200, map[string]string{"access_token": t, "player_id": id})
		return
	}
	if path == "/v1/internal/tickets/redeem" {
		if !serviceAuthorized(r.Header.Get("Authorization"), h.Service.ServiceToken) {
			if os.Getenv("ALLOW_DEV_DOCKER_HTTP") == "1" {
				fmt.Fprintf(os.Stderr, "dev service auth mismatch header_len=%d expected_len=%d\\n", len(r.Header.Get("Authorization")), len("Bearer "+h.Service.ServiceToken))
			}
			failure(w, 401, "unauthorized")
			return
		}
		var x redeemReq
		if !decode(b, &x) || !text(x.Ticket, 256) || !text(x.Room, 128) || !text(x.Preset, 32) || !text(x.Content, 128) || !text(x.Config, 128) || x.Proto < 1 || x.Proto > 100 {
			fmt.Printf("REDEEM_INVALID ticket=%d room=%d preset=%d content=%d config=%d proto=%d\n", len(x.Ticket), len(x.Room), len(x.Preset), len(x.Content), len(x.Config), x.Proto)
			failure(w, 422, "invalid request")
			return
		}
		out, e := h.Service.Redeem(ctx, domain.Ticket{Token: x.Ticket, RoomID: x.Room, PresetID: x.Preset, Protocol: x.Proto, Content: x.Content, ConfigHash: x.Config})
		if e != nil {
			fmt.Printf("REDEEM_ERR token=%s room=%s preset=%s err=%v\n", x.Ticket, x.Room, x.Preset, e)
			dbError(w, e)
			return
		}
		response(w, 200, map[string]string{"player_id": out.PlayerID, "room_id": out.RoomID, "preset_id": out.PresetID, "resolved_config_hash": out.ConfigHash})
		fmt.Printf("REDEEM_OK player=%s room=%s preset=%s\n", out.PlayerID, out.RoomID, out.PresetID)
		return
	}
	if strings.HasPrefix(path, "/v1/internal/progress/") {
		if !serviceAuthorized(r.Header.Get("Authorization"), h.Service.ServiceToken) {
			failure(w, 401, "unauthorized")
			return
		}
		if path == "/v1/internal/progress/acquire" {
			var x leaseReq
			if !decode(b, &x) || !text(x.PlayerID, 64) || !text(x.RoomID, 128) {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.AcquireSession(ctx, x.PlayerID, x.RoomID)
			if e != nil {
				fmt.Printf("ACQUIRE_ERR player=%s room=%s err=%v\n", x.PlayerID, x.RoomID, e)
				dbError(w, e)
				return
			}
			fmt.Printf("ACQUIRE_OK player=%s room=%s token=%d\n", x.PlayerID, x.RoomID, out.FencingToken)
			response(w, 200, out)
			return
		}
		if path == "/v1/internal/progress/renew" {
			var x leaseReq
			if !decode(b, &x) || !text(x.PlayerID, 64) || !text(x.RoomID, 128) || !integer(x.FencingToken) || x.FencingToken < 1 {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.RenewSession(ctx, domain.SessionLease{PlayerID: x.PlayerID, RoomID: x.RoomID, FencingToken: int64(x.FencingToken)})
			if e != nil {
				dbError(w, e)
				return
			}
			response(w, 200, out)
			return
		}
		if path == "/v1/internal/progress/release" {
			var x leaseReq
			if !decode(b, &x) || !text(x.PlayerID, 64) || !text(x.RoomID, 128) || !integer(x.FencingToken) || x.FencingToken < 1 {
				failure(w, 422, "invalid request")
				return
			}
			if e := h.Service.ReleaseSession(ctx, domain.SessionLease{PlayerID: x.PlayerID, RoomID: x.RoomID, FencingToken: int64(x.FencingToken)}); e != nil {
				dbError(w, e)
				return
			}
			response(w, 200, map[string]string{"status": "ok"})
			return
		}
		if path == "/v1/internal/progress/commit" {
			var x commitReq
			if !decode(b, &x) || !text(x.PlayerID, 64) || !text(x.RoomID, 128) || !text(x.OperationID, 128) || !integer(x.FencingToken) || !integer(x.ExpectedRevision) || x.FencingToken < 1 || len(x.Payload) == 0 || len(x.Payload) > 4096 {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.CommitProgress(ctx, domain.ProgressCommit{PlayerID: x.PlayerID, RoomID: x.RoomID, FencingToken: int64(x.FencingToken), ExpectedRevision: int64(x.ExpectedRevision), OperationID: x.OperationID, Payload: x.Payload})
			if e != nil {
				dbError(w, e)
				return
			}
			response(w, 200, out)
			return
		}
		if path == "/v1/internal/progress/query" {
			var x domain.ProgressQuery
			if !decode(b, &x) || !text(x.PlayerID, 64) || !text(x.OperationID, 128) {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.QueryProgress(ctx, x)
			if e != nil {
				dbError(w, e)
				return
			}
			response(w, 200, out)
			return
		}
		if path == "/v1/internal/progress/snapshot" {
			var x struct{ PlayerID string `json:"player_id"` }
			if !decode(b, &x) || !text(x.PlayerID, 64) {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.GetProgressSnapshot(ctx, x.PlayerID)
			if e != nil {
				dbError(w, e)
				return
			}
			response(w, 200, out)
			return
		}
		if path == "/v1/internal/progress/equipment" {
			var x equipmentReq
			if !decode(b, &x) || !text(x.PlayerID, 64) || !text(x.RoomID, 128) || !text(x.OperationID, 128) || !text(x.Slot, 32) || !text(x.ItemID, 128) || !integer(x.FencingToken) || !integer(x.ExpectedRevision) || x.FencingToken < 1 || x.ExpectedRevision < 0 {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.CommitEquipment(ctx, domain.EquipmentCommit{PlayerID: x.PlayerID, RoomID: x.RoomID, FencingToken: int64(x.FencingToken), ExpectedRevision: int64(x.ExpectedRevision), OperationID: x.OperationID, Slot: x.Slot, ItemID: x.ItemID, Equipped: x.Equipped})
			if e != nil { dbError(w, e); return }
			response(w, 200, out)
			return
		}
		if path == "/v1/internal/progress/quests" {
			var x struct{ PlayerID string `json:"player_id"` }
			if !decode(b, &x) || !text(x.PlayerID, 64) { failure(w, 422, "invalid request"); return }
			out, e := h.Service.GetQuestSnapshot(ctx, x.PlayerID)
			if e != nil { dbError(w, e); return }
			response(w, 200, out)
			return
		}
	}
	if strings.HasPrefix(path, "/v1/internal/rooms/") {
		if !serviceAuthorized(r.Header.Get("Authorization"), h.Service.ServiceToken) {
			if os.Getenv("ALLOW_DEV_DOCKER_HTTP") == "1" {
				fmt.Fprintf(os.Stderr, "dev room auth mismatch header_len=%d expected_len=%d\\n", len(r.Header.Get("Authorization")), len("Bearer "+h.Service.ServiceToken))
			}
			failure(w, 401, "unauthorized")
			return
		}
		if path == "/v1/internal/rooms/register" {
			var x domain.RoomRecord
			if !decode(b, &x) || !text(x.ID, 128) || !text(x.Host, 256) || x.Port < 1 || x.Port > 65535 || x.Generation < 1 || x.Capacity < 1 || x.Capacity > 8 {
				failure(w, 422, "invalid request")
				return
			}
			out, e := h.Service.RegisterRoom(ctx, x)
			if e != nil {
				dbError(w, e)
				return
			}
			response(w, 200, out)
			return
		}
		var x struct {
			RoomID      string `json:"room_id"`
			Generation  int    `json:"generation"`
			Capacity    int    `json:"capacity"`
			UsedPlayers int    `json:"used_players"`
			Status      string `json:"status"`
			PlayerID    string `json:"player_id"`
		}
		if !decode(b, &x) || !text(x.RoomID, 128) {
			failure(w, 422, "invalid request")
			return
		}
		var e error
		if path == "/v1/internal/rooms/heartbeat" {
			if x.Generation < 1 || x.Capacity < 1 || x.Capacity > 8 || x.UsedPlayers < 0 || x.UsedPlayers > x.Capacity || (x.Status != "ready" && x.Status != "draining") {
				failure(w, 422, "invalid heartbeat")
				return
			}
			e = h.Service.Store.HeartbeatRoom(ctx, x.RoomID, x.Generation, x.Capacity, x.UsedPlayers, x.Status)
		} else {
			e = h.Service.Store.ReleaseReservation(ctx, x.RoomID, x.PlayerID)
		}
		if e != nil {
			dbError(w, e)
			return
		}
		response(w, 200, map[string]string{"status": "ok"})
		return
	}
	auth := strings.Fields(r.Header.Get("Authorization"))
	if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") {
		failure(w, 401, "unauthorized")
		return
	}
	s, e := h.Service.Auth(ctx, auth[1])
	if e != nil {
		dbError(w, e)
		return
	}
	if path == "/v1/auth/logout" {
		if e = h.Service.Store.RevokeSession(ctx, s.TokenHash); e != nil {
			dbError(w, e)
			return
		}
		response(w, 200, map[string]string{"status": "ok"})
		return
	}
	var x ticketReq
	if !decode(b, &x) || !text(x.Preset, 32) || !text(x.Content, 128) || x.Proto < 1 || x.Proto > 100 {
		failure(w, 422, "invalid request")
		return
	}
	if !usecase.ValidPreset(x.Preset, x.Content, x.Proto) {
		fmt.Printf("ALLOCATE_ERR validPreset player=%s preset=%s content=%s proto=%d\n", s.PlayerID, x.Preset, x.Content, x.Proto)
		failure(w, 409, "version mismatch")
		return
	}
	if path == "/v1/rooms/allocate" {
		out, r, e := h.Service.Allocate(ctx, s.PlayerID, s.TokenHash, x.Preset, x.Content, x.Proto)
		if e != nil {
			fmt.Printf("ALLOCATE_ERR player=%s preset=%s content=%s proto=%d err=%v\n", s.PlayerID, x.Preset, x.Content, x.Proto, e)
			if errors.Is(e, pgx.ErrNoRows) {
				failure(w, 409, "capacity unavailable")
				return
			}
			dbError(w, e)
			return
		}
		response(w, 200, map[string]any{"ticket": out.Token, "room_id": r.ID, "host": r.Host, "port": r.Port, "preset_id": out.PresetID, "resolved_config_hash": out.ConfigHash, "protocol_version": out.Protocol, "gameplay_content_hash": out.Content})
		return
	}
	out, e := h.Service.Ticket(ctx, s.PlayerID, s.TokenHash, x.Preset, x.Content, x.Proto)
	if e != nil {
		dbError(w, e)
		return
	}
	response(w, 200, map[string]any{"ticket": out.Token, "room_id": out.RoomID, "host": h.Service.Host, "port": h.Service.Port, "preset_id": out.PresetID, "resolved_config_hash": out.ConfigHash, "protocol_version": out.Protocol, "gameplay_content_hash": out.Content})
}

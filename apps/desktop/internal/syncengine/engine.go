package syncengine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/domain"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/storage"
)

type Engine struct {
	store   *storage.Store
	baseURL string
	client  *http.Client
	mu      sync.RWMutex
	token   string
}

type pushRequest struct {
	Operations []domain.SyncOperation `json:"operations"`
}

type pushResponse struct {
	Accepted  []string `json:"accepted"`
	Conflicts []struct {
		OperationID string `json:"operation_id"`
		EntityID    string `json:"entity_id"`
	} `json:"conflicts"`
}

type pullResponse struct {
	Employees []domain.Employee `json:"employees"`
	Cursor    string            `json:"cursor"`
}

func New(store *storage.Store, baseURL, token string) *Engine {
	return &Engine{
		store:   store,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 12 * time.Second},
		token:   token,
	}
}

func (e *Engine) SetAccessToken(token string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.token = token
}

func (e *Engine) accessToken() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.token
}

func (e *Engine) authorize(req *http.Request) error {
	token := strings.TrimSpace(e.accessToken())
	if token == "" {
		return fmt.Errorf("sign in is required before cloud sync")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

func (e *Engine) Sync(ctx context.Context) (domain.SyncResult, error) {
	result := domain.SyncResult{}
	operations, err := e.store.PendingOperations(ctx, 100)
	if err != nil {
		return result, err
	}

	if len(operations) > 0 {
		body, err := json.Marshal(pushRequest{Operations: operations})
		if err != nil {
			return result, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/v1/sync/push", bytes.NewReader(body))
		if err != nil {
			return result, err
		}
		if err := e.authorize(req); err != nil {
			return result, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := e.client.Do(req)
		if err != nil {
			result.Offline = true
			result.Message = "Cloud unavailable. Changes remain safely queued on this device."
			result.Pending, _ = e.store.PendingCount(ctx)
			return result, nil
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return result, fmt.Errorf("desktop session is no longer authorized; sign in again")
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return result, fmt.Errorf("sync push failed: %s", resp.Status)
		}
		var pushed pushResponse
		if err := json.NewDecoder(resp.Body).Decode(&pushed); err != nil {
			return result, err
		}
		if err := e.store.DeleteOperations(ctx, pushed.Accepted); err != nil {
			return result, err
		}
		result.Pushed = len(pushed.Accepted)
		result.Conflicts = len(pushed.Conflicts)
	}

	cursor, _ := e.store.State(ctx, "cursor")
	pullURL, err := url.Parse(e.baseURL + "/api/v1/sync/pull")
	if err != nil {
		return result, err
	}
	query := pullURL.Query()
	if cursor != "" {
		query.Set("since", cursor)
	}
	pullURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pullURL.String(), nil)
	if err != nil {
		return result, err
	}
	if err := e.authorize(req); err != nil {
		return result, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		result.Offline = true
		result.Message = "Working offline. Cloud sync will resume when connectivity returns."
		result.Pending, _ = e.store.PendingCount(ctx)
		return result, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return result, fmt.Errorf("desktop session is no longer authorized; sign in again")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, fmt.Errorf("sync pull failed: %s", resp.Status)
	}

	var pulled pullResponse
	if err := json.NewDecoder(resp.Body).Decode(&pulled); err != nil {
		return result, err
	}
	for _, employee := range pulled.Employees {
		pending, err := e.store.HasPendingEntity(ctx, "employee", employee.ID)
		if err != nil {
			return result, err
		}
		if pending {
			continue
		}
		if err := e.store.ApplyRemoteEmployee(ctx, employee); err != nil {
			return result, err
		}
		result.Pulled++
	}
	if pulled.Cursor != "" {
		if err := e.store.SetState(ctx, "cursor", pulled.Cursor); err != nil {
			return result, err
		}
	}
	now := time.Now().UTC()
	_ = e.store.SetState(ctx, "last_sync_at", now.Format(time.RFC3339))
	result.Cursor = pulled.Cursor
	result.LastSyncAt = now
	result.Pending, _ = e.store.PendingCount(ctx)
	if result.Conflicts > 0 {
		result.Message = fmt.Sprintf("%d change(s) need conflict resolution before they can sync.", result.Conflicts)
	} else {
		result.Message = "Desktop data is synchronized with Advance HRIS Cloud."
	}
	return result, nil
}

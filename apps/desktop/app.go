package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/domain"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/secure"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/storage"
	"github.com/savisaluwadana/Advance-HIRS-System/apps/desktop/internal/syncengine"
)

type App struct {
	ctx           context.Context
	store         *storage.Store
	syncer        *syncengine.Engine
	baseURL       string
	initErr       error
	offline       bool
	authenticated bool
	user          domain.AuthUser
	client        *http.Client
}

func NewApp() *App {
	baseURL := os.Getenv("ADVANCE_HRIS_API_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:8080"
	}
	return &App{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 12 * time.Second},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cipher, err := secure.NewKeyringCipher()
	if err != nil {
		a.initErr = err
		return
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		a.initErr = err
		return
	}
	dataDir := filepath.Join(configDir, "AdvanceHRIS")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		a.initErr = err
		return
	}

	store, err := storage.Open(filepath.Join(dataDir, "advance-hris.db"), cipher)
	if err != nil {
		a.initErr = err
		return
	}
	a.store = store

	token, err := secure.LoadAccessToken()
	if err != nil {
		a.initErr = fmt.Errorf("load desktop session: %w", err)
		return
	}
	a.syncer = syncengine.New(store, a.baseURL, token)
	if token != "" {
		_ = a.restoreSession(ctx, token)
	}

	count, err := store.EmployeeCount(ctx)
	if err != nil {
		a.initErr = err
		return
	}
	if count == 0 {
		for _, employee := range domain.DemoEmployees() {
			if err := store.ApplyRemoteEmployee(ctx, employee); err != nil {
				a.initErr = err
				return
			}
		}
	}
}

func (a *App) shutdown(context.Context) {
	if a.store != nil {
		_ = a.store.Close()
	}
}

func (a *App) GetDesktopState() (domain.DesktopState, error) {
	if a.initErr != nil {
		return domain.DesktopState{}, a.initErr
	}
	if a.store == nil {
		return domain.DesktopState{}, fmt.Errorf("desktop storage is not initialized")
	}
	if !a.authenticated {
		return domain.DesktopState{
			APIBaseURL: a.baseURL, StorageReady: true, Authenticated: false,
		}, nil
	}
	employees, err := a.store.Employees(a.ctx)
	if err != nil {
		return domain.DesktopState{}, err
	}
	pending, err := a.store.PendingCount(a.ctx)
	if err != nil {
		return domain.DesktopState{}, err
	}
	lastSync, _ := a.store.State(a.ctx, "last_sync_at")
	return domain.DesktopState{
		Employees: employees, PendingSync: pending, LastSyncAt: lastSync,
		Offline: a.offline, APIBaseURL: a.baseURL, StorageReady: true,
		Authenticated: true, User: a.user,
	}, nil
}

func (a *App) Login(email, password, organizationSlug string) (domain.AuthSession, error) {
	payload, err := json.Marshal(map[string]string{
		"email": email, "password": password, "organization_slug": organizationSlug,
	})
	if err != nil {
		return domain.AuthSession{}, err
	}
	req, err := http.NewRequestWithContext(a.ctx, http.MethodPost, a.baseURL+"/api/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		return domain.AuthSession{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return domain.AuthSession{}, fmt.Errorf("could not reach Advance HRIS Cloud: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return domain.AuthSession{}, fmt.Errorf("sign in failed: %s", resp.Status)
	}
	var response struct {
		AccessToken string          `json:"access_token"`
		ExpiresAt   time.Time       `json:"expires_at"`
		User        domain.AuthUser `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return domain.AuthSession{}, err
	}
	if response.AccessToken == "" {
		return domain.AuthSession{}, fmt.Errorf("cloud returned an empty access token")
	}
	if !desktopRoleAllowed(response.User.Role) {
		return domain.AuthSession{}, fmt.Errorf("the desktop console is restricted to HR and administrator accounts; use the web portal for %s access", response.User.Role)
	}
	if err := secure.SaveAccessToken(response.AccessToken); err != nil {
		return domain.AuthSession{}, fmt.Errorf("save desktop session: %w", err)
	}
	a.syncer.SetAccessToken(response.AccessToken)
	a.authenticated = true
	a.user = response.User
	return domain.AuthSession{Authenticated: true, ExpiresAt: response.ExpiresAt, User: response.User}, nil
}

func (a *App) Logout() error {
	if err := secure.ClearAccessToken(); err != nil {
		return err
	}
	a.syncer.SetAccessToken("")
	a.authenticated = false
	a.user = domain.AuthUser{}
	return nil
}

func (a *App) SaveEmployee(employee domain.Employee) error {
	if a.initErr != nil {
		return a.initErr
	}
	if !a.authenticated || !desktopRoleAllowed(a.user.Role) {
		return fmt.Errorf("HR or administrator access is required before editing HR records")
	}
	if employee.ID == "" {
		employee.ID = fmt.Sprintf("emp_%d", time.Now().UnixNano())
	}
	employee.UpdatedAt = time.Now().UTC()
	return a.store.SaveLocalEmployee(a.ctx, employee)
}

func (a *App) SyncNow() (domain.SyncResult, error) {
	if a.initErr != nil {
		return domain.SyncResult{}, a.initErr
	}
	if !a.authenticated || !desktopRoleAllowed(a.user.Role) {
		return domain.SyncResult{}, fmt.Errorf("HR or administrator access is required before cloud sync")
	}
	result, err := a.syncer.Sync(a.ctx)
	if err != nil {
		return result, err
	}
	a.offline = result.Offline
	return result, nil
}

func (a *App) restoreSession(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/v1/auth/me", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_ = secure.ClearAccessToken()
		a.syncer.SetAccessToken("")
		return fmt.Errorf("stored session expired")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("restore session failed: %s", resp.Status)
	}
	var response struct {
		User domain.AuthUser `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}
	if !desktopRoleAllowed(response.User.Role) {
		_ = secure.ClearAccessToken()
		a.syncer.SetAccessToken("")
		return fmt.Errorf("stored account is not permitted to use the HR desktop console")
	}
	a.authenticated = true
	a.user = response.User
	return nil
}

func desktopRoleAllowed(role string) bool {
	return role == "admin" || role == "hr"
}

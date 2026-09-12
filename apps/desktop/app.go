package main

import (
	"context"
	"fmt"
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
	ctx     context.Context
	store   *storage.Store
	syncer  *syncengine.Engine
	baseURL string
	initErr error
	offline bool
}

func NewApp() *App {
	baseURL := os.Getenv("ADVANCE_HRIS_API_URL")
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "http://localhost:8080"
	}
	return &App{baseURL: baseURL}
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
	a.syncer = syncengine.New(store, a.baseURL)

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
	}, nil
}

func (a *App) SaveEmployee(employee domain.Employee) error {
	if a.initErr != nil {
		return a.initErr
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
	result, err := a.syncer.Sync(a.ctx)
	if err != nil {
		return result, err
	}
	a.offline = result.Offline
	return result, nil
}

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

func mustURLStore(t *testing.T) *URLStore {
	t.Helper()
	s, err := NewURLStore(&config.Config{})
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	if err := s.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

// TestSaveWrappedTypeChain verifies the typed store errors survive wrapping
// so the service layer can match them with errors.As.
func TestSaveWrappedTypeChain(t *testing.T) {
	s := mustURLStore(t)
	existing := &model.ShortURL{Code: "dupCode", RawURL: "https://example.com", CreatedAt: time.Now()}
	if err := s.Save(existing, false); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	// second save with overwrite=false -> ErrURLCodeAlreadyExists
	err := s.Save(existing, false)

	var dupErr *model.ErrURLCodeAlreadyExists
	if !errors.As(err, &dupErr) {
		t.Fatalf("errors.As(ErrURLCodeAlreadyExists) = false; err=%v (type %T)", err, err)
	}
	if dupErr.Code != "dupCode" {
		t.Fatalf("matched error Code = %q, want %q", dupErr.Code, "dupCode")
	}
	t.Logf("OK: errors.As matched ErrURLCodeAlreadyExists via wrap (msg=%q)", err.Error())
}

// TestGetWrappedTypeChains covers the three typed errors produced by Get.
func TestGetWrappedTypeChains(t *testing.T) {
	s := mustURLStore(t)

	t.Run("NotFound", func(t *testing.T) {
		_, err := s.Get("missing")
		var notFound *model.ErrURLCodeNotFound
		if !errors.As(err, &notFound) {
			t.Fatalf("errors.As(ErrURLCodeNotFound) = false; err=%v (%T)", err, err)
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		u := &model.ShortURL{Code: "disCode", RawURL: "https://example.com", CreatedAt: time.Now(), Disabled: true}
		if err := s.Save(u, false); err != nil {
			t.Fatalf("seed: %v", err)
		}
		_, err := s.Get("disCode")
		var disabled *model.ErrRedirectDisabled
		if !errors.As(err, &disabled) {
			t.Fatalf("errors.As(ErrRedirectDisabled) = false; err=%v (%T)", err, err)
		}
	})

	t.Run("Expired", func(t *testing.T) {
		u := &model.ShortURL{Code: "expCode", RawURL: "https://example.com", CreatedAt: time.Now(), Visits: 10001}
		if err := s.Save(u, false); err != nil {
			t.Fatalf("seed: %v", err)
		}
		_, err := s.Get("expCode")
		var expired *model.ErrRedirectExpired
		if !errors.As(err, &expired) {
			t.Fatalf("errors.As(ErrRedirectExpired) = false; err=%v (%T)", err, err)
		}
	})
}

// TestClosedStoreWrappedTypeChain verifies ErrURLStoreUnavailable survives
// wrapping across Save, Get and IncrementVisits when the store is closed.
func TestClosedStoreWrappedTypeChain(t *testing.T) {
	s := mustURLStore(t)
	_ = s.Close()

	var storeErr *model.ErrURLStoreUnavailable

	if err := s.Save(&model.ShortURL{Code: "x", RawURL: "https://example.com"}, false); !errors.As(err, &storeErr) {
		t.Errorf("Save closed: errors.As(ErrURLStoreUnavailable) = false; err=%v (%T)", err, err)
	}
	if _, err := s.Get("x"); !errors.As(err, &storeErr) {
		t.Errorf("Get closed: errors.As(ErrURLStoreUnavailable) = false; err=%v (%T)", err, err)
	}
	if err := s.IncrementVisits("x"); !errors.As(err, &storeErr) {
		t.Errorf("IncrementVisits closed: errors.As(ErrURLStoreUnavailable) = false; err=%v (%T)", err, err)
	}
}

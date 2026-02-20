package vault

import (
	"sync"
	"testing"
)

func TestStoreSetGet(t *testing.T) {
	s := NewStore()
	info := Info{
		ECDSAPublicKey: "02abc",
		EdDSAPublicKey: "ed123",
		ChainCode:      "cc456",
	}

	s.Set("session1", info)

	got, ok := s.Get("session1")
	if !ok {
		t.Fatal("expected to find session1")
	}
	if got != info {
		t.Fatalf("got %+v, want %+v", got, info)
	}
}

func TestStoreGetMissing(t *testing.T) {
	s := NewStore()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Fatal("expected not to find nonexistent session")
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	s.Set("session1", Info{ECDSAPublicKey: "key"})
	s.Delete("session1")

	_, ok := s.Get("session1")
	if ok {
		t.Fatal("expected session1 to be deleted")
	}
}

func TestStoreOverwrite(t *testing.T) {
	s := NewStore()
	s.Set("session1", Info{ECDSAPublicKey: "old"})
	s.Set("session1", Info{ECDSAPublicKey: "new"})

	got, ok := s.Get("session1")
	if !ok {
		t.Fatal("expected to find session1")
	}
	if got.ECDSAPublicKey != "new" {
		t.Fatalf("got ECDSAPublicKey=%q, want %q", got.ECDSAPublicKey, "new")
	}
}

func TestStoreConcurrency(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "session"
			s.Set(id, Info{ECDSAPublicKey: "key"})
			s.Get(id)
			if n%2 == 0 {
				s.Delete(id)
			}
		}(i)
	}

	wg.Wait()
}

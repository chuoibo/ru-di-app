package janitor

import "testing"

type fakeStore map[string]bool

func (f fakeStore) Delete(key string) (bool, error) {
	had := f[key]
	delete(f, key)
	return had, nil
}

// The reaper's database half is covered on real PostgreSQL; this pins the
// interface a PhotoStorage satisfies.
func TestFakeStoreSatisfiesObjectStore(t *testing.T) {
	var _ ObjectStore = fakeStore{}
}

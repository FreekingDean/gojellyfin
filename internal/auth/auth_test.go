package auth

import "testing"

func TestVerify(t *testing.T) {
	hash, err := Hash("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("matches the password it was made from", func(t *testing.T) {
		ok, err := Verify("hunter2", hash)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("want match for correct password")
		}
	})

	t.Run("refuses another password", func(t *testing.T) {
		ok, err := Verify("hunter3", hash)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Error("want no match for wrong password")
		}
	})

	t.Run("errors on a malformed hash", func(t *testing.T) {
		for _, malformed := range []string{"", "hunter2", "$1$AA$BB"} {
			if _, err := Verify("hunter2", malformed); err == nil {
				t.Errorf("want error for %q", malformed)
			}
		}
	})
}

func TestHash(t *testing.T) {
	a, err := Hash("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	b, err := Hash("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	if a == b {
		t.Error("want distinct hashes for the same password")
	}
}

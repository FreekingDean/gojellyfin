package auth

import "testing"

func TestVerifyAcceptsHashedPassword(t *testing.T) {
	hash, err := Hash("hunter2")
	if err != nil {
		t.Fatal(err)
	}

	ok, err := Verify("hunter2", hash)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("want match for correct password")
	}

	ok, err = Verify("hunter3", hash)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("want no match for wrong password")
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
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

func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, hash := range []string{"", "hunter2", "$MD5$1$AA$BB", "$PBKDF2-SHA512$notanumber$AA$BB"} {
		if _, err := Verify("hunter2", hash); err == nil {
			t.Errorf("want error for %q", hash)
		}
	}
}

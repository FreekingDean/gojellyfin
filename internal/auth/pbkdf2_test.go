package auth

import "testing"

const (
	sha512Hash = "$PBKDF2-SHA512$iterations=210000$0102030405060708090A0B0C0D0E0F10$" +
		"8A5A90C00EB2155E81D3C6B82ABEE6B9875A0E7CE286688E796A32F9675D1724" +
		"5E71EE6B008F47256E6EC66BBD8040417DF06E50007501FEDF44A2588B949C1F"
	sha1Hash = "$PBKDF2$iterations=1000$0102030405060708090A0B0C0D0E0F10$" +
		"4198375FF7DC0FE4DD95C924CF0B2296F4F551102C3B93F4DD08D24F158F49AE"
)

func TestVerify_Jellyfin(t *testing.T) {
	t.Run("accepts the password behind a PBKDF2-SHA512 hash", func(t *testing.T) {
		ok, err := Verify("hunter2", sha512Hash)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("want match for the correct password")
		}
	})

	t.Run("accepts the password behind a legacy PBKDF2 hash", func(t *testing.T) {
		ok, err := Verify("hunter2", sha1Hash)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("want match for the correct password")
		}
	})

	t.Run("refuses another password", func(t *testing.T) {
		for _, encoded := range []string{sha512Hash, sha1Hash} {
			ok, err := Verify("hunter3", encoded)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Errorf("want no match for %q", encoded)
			}
		}
	})

	t.Run("errors on a malformed hash", func(t *testing.T) {
		malformed := []string{
			"$PBKDF2-SHA512$iterations=210000$0102$",
			"$PBKDF2-SHA512$iterations=210000$0102030405060708090A0B0C0D0E0F10$AABB",
			"$PBKDF2-SHA512$0102030405060708090A0B0C0D0E0F10$AABB",
			"$PBKDF2-SHA512$iterations=0$0102030405060708090A0B0C0D0E0F10$AABB",
			"$PBKDF2-SHA256$iterations=210000$0102030405060708090A0B0C0D0E0F10$AABB",
			"$PBKDF2-SHA512$iterations=210000$ZZ$AABB",
		}
		for _, encoded := range malformed {
			if _, err := Verify("hunter2", encoded); err == nil {
				t.Errorf("want error for %q", encoded)
			}
		}
	})

	t.Run("leaves a bcrypt hash to bcrypt", func(t *testing.T) {
		encoded, err := Hash("hunter2")
		if err != nil {
			t.Fatal(err)
		}
		if isJellyfinHash(encoded) {
			t.Fatalf("bcrypt hash %q reads as a jellyfin one", encoded)
		}

		ok, err := Verify("hunter2", encoded)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Error("want match for the correct password")
		}
	})
}

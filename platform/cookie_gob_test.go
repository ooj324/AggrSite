package platform

import (
	"testing"
)

func TestExtractLikelyUserIDsFromGobToken(t *testing.T) {
	// The exact token provided by the user containing Gob-encoded ID 3103 inside parts[1].
	token := "MTc4Mjg3NTgwOXxEWDhFQVFMX2dBQUJFQUVRQUFEX3ZfLUFBQVlHYzNSeWFXNW5EQTBBQzI5aGRYUm9YM04wWVhSbEJuTjBjbWx1Wnd3T0FBdzBZbUpQY0ZOMVZsVkZiVklHYzNSeWFXNW5EQVFBQW1sa0EybHVkQVFFQVA0WVBnWnpkSEpwYm1jTUNnQUlkWE5sY201aGJXVUdjM1J5YVc1bkRBa0FCMnBoWTJsc2Iza0djM1J5YVc1bkRBWUFCSEp2YkdVRGFXNTBCQUlBQWdaemRISnBibWNNQ0FBR2MzUmhkSFZ6QTJsdWRBUUNBQUlHYzNSeWFXNW5EQWNBQldkeWIzVndCbk4wY21sdVp3d0pBQWRrWldaaGRXeDB8kqONAfF7zzqtXxs1X8BMPwDil4AEwUKbN5TN05mXHH8="
	ids := ExtractLikelyUserIDs(token)
	found := false
	for _, id := range ids {
		if id == 3103 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to extract user ID 3103 from Gob token, got %v", ids)
	}
}

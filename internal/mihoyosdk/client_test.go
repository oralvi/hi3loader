package mihoyosdk

import "testing"

func TestExtractSessionInfoAllowsMissingUID(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"open_id":     "open-id",
			"combo_id":    "combo-id",
			"combo_token": "combo-token",
		},
	}

	session, err := ExtractSessionInfo(resp)
	if err != nil {
		t.Fatalf("extract session info: %v", err)
	}
	if session == nil {
		t.Fatal("expected session info")
	}
	if session.UID != "" {
		t.Fatalf("expected uid to stay empty, got %q", session.UID)
	}
	if session.OpenID != "open-id" {
		t.Fatalf("unexpected open_id: %q", session.OpenID)
	}
	if session.ComboToken != "combo-token" {
		t.Fatalf("unexpected combo token: %q", session.ComboToken)
	}
}

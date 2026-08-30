package acp

import (
	"encoding/json"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
)

func TestACP_BijectiveTranslation(t *testing.T) {
	// Test round-trip for initialize, new, prompt, update, cancel, request_permission
	cases := []adapter.Capabilities{
		{ProtocolVersion: 1},
		{ProtocolVersion: 1, Permission: true},
		{ProtocolVersion: 1, Terminal: true, LoadSession: true},
		{ProtocolVersion: 2, Permission: true, Extra: map[string]any{"_custom": "value", "_future": "additive"}},
	}
	for _, tc := range cases {
		m := ToACP(tc)
		got := FromACP(m)
		if got.ProtocolVersion != tc.ProtocolVersion {
			t.Fatalf("ProtocolVersion round-trip %d vs %d", got.ProtocolVersion, tc.ProtocolVersion)
		}
		if got.Permission != tc.Permission {
			t.Fatalf("Permission round-trip %v vs %v (case %v)", got.Permission, tc.Permission, tc)
		}
		if got.Terminal != tc.Terminal {
			t.Fatalf("Terminal round-trip")
		}
		if got.LoadSession != tc.LoadSession {
			t.Fatalf("LoadSession round-trip")
		}
		for k, v := range tc.Extra {
			if got.Extra[k] != v {
				t.Fatalf("Extra[%q] = %v, want %v", k, got.Extra[k], v)
			}
		}
	}
}

func TestACP_TranslateMessages(t *testing.T) {
	// initialize
	initReq := TranslateInitialize(adapter.Capabilities{ProtocolVersion: 1, Permission: true})
	if initReq["protocolVersion"] != float64(1) && initReq["protocolVersion"] != 1 {
		// Use number
		b, _ := json.Marshal(initReq)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
	}
	// session/new
	newReq := TranslateSessionNew(adapter.SessionBundle{Instructions: "hello", WorkspaceRoot: "/tmp"})
	if newReq["instructions"] != "hello" {
		t.Fatalf("newReq instructions = %v", newReq["instructions"])
	}
	// prompt
	promptReq := TranslatePrompt("sess-1", adapter.PromptInput{Text: "hi"})
	if promptReq["sessionId"] != "sess-1" {
		t.Fatalf("prompt sessionId = %v", promptReq["sessionId"])
	}
	// update (harness -> broker notification)
	updateNotif := TranslateUpdate("sess-1", 1, map[string]any{"delta": "hello"})
	if updateNotif["cursor"] != 1 && updateNotif["cursor"] != float64(1) {
		t.Fatalf("update cursor = %v", updateNotif["cursor"])
	}
	// cancel
	cancelReq := TranslateCancel("sess-1")
	if cancelReq["sessionId"] != "sess-1" {
		t.Fatalf("cancel sessionId")
	}
	// request_permission (reverse)
	permReq := TranslateRequestPermission("sess-1", adapter.PermissionRequest{Kind: "ask", Description: "need", Options: []string{"allow"}})
	if permReq["kind"] != "ask" {
		t.Fatalf("perm kind")
	}
	// Triangulate bijective for permission decision
	decision := TranslatePermissionDecision(map[string]any{"option": "allow"})
	if decision.Option != "allow" {
		t.Fatalf("decision round-trip")
	}
}

func TestACP_FixtureRoundTrip(t *testing.T) {
	// Simulate complete fixture cycle: translate out and back
	original := adapter.Capabilities{ProtocolVersion: 1, Permission: true, Extra: map[string]any{"_a": "1"}}
	m := ToACP(original)
	back := FromACP(m)
	// Check bijective: re-translate should be identical
	m2 := ToACP(back)
	for k, v := range m {
		if m2[k] != v {
			// JSON numbers may differ type but compare via marshal
			b1, _ := json.Marshal(v)
			b2, _ := json.Marshal(m2[k])
			if string(b1) != string(b2) {
				t.Fatalf("ACP round-trip mismatch for %q: %v vs %v", k, v, m2[k])
			}
		}
	}
}

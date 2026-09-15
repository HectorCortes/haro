package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func TestCLIStepRunAndEventsUseBroker(t *testing.T) {
	previous := brokerCall
	t.Cleanup(func() { brokerCall = previous })
	var methods []string
	brokerCall = func(_ context.Context, _ string, method string, _ any) (json.RawMessage, error) {
		methods = append(methods, method)
		switch method {
		case "step.run":
			return json.RawMessage(`{"attempt_id":"attempt-1","next_cursor":0}`), nil
		case "step.events":
			return json.RawMessage(`{"events":[{"cursor":0,"event_type":"output_delta","payload":"done"}],"next_cursor":1}`), nil
		default:
			t.Fatalf("unexpected broker method %q", method)
			return nil, nil
		}
	}

	var runOut bytes.Buffer
	if code := Execute(context.Background(), []string{"step", "run", "exec-1", "one", "--json"}, t.TempDir(), &runOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("step run exit=%d output=%q", code, runOut.String())
	}
	var run map[string]any
	if err := json.Unmarshal(runOut.Bytes(), &run); err != nil || run["attempt_id"] != "attempt-1" {
		t.Fatalf("step run output=%q err=%v", runOut.String(), err)
	}

	var eventsOut bytes.Buffer
	if code := Execute(context.Background(), []string{"step", "events", "exec-1", "one", "--json"}, t.TempDir(), &eventsOut, &bytes.Buffer{}); code != 0 {
		t.Fatalf("step events exit=%d output=%q", code, eventsOut.String())
	}
	var events map[string]any
	if err := json.Unmarshal(eventsOut.Bytes(), &events); err != nil || events["next_cursor"] != float64(1) {
		t.Fatalf("step events output=%q err=%v", eventsOut.String(), err)
	}
	if len(methods) != 2 || methods[0] != "step.run" || methods[1] != "step.events" {
		t.Fatalf("broker methods=%v", methods)
	}
}

func TestCLIStepReopenUsesBrokerAndReturnsInvalidated(t *testing.T) {
	previous := brokerCall
	t.Cleanup(func() { brokerCall = previous })
	var method string
	var params map[string]any
	brokerCall = func(_ context.Context, _ string, gotMethod string, gotParams any) (json.RawMessage, error) {
		method = gotMethod
		encoded, err := json.Marshal(gotParams)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &params); err != nil {
			t.Fatal(err)
		}
		return json.RawMessage(`{"invalidated":["s1","s2"]}`), nil
	}

	var out bytes.Buffer
	if code := Execute(context.Background(), []string{"step", "reopen", "exec-1", "s1", "--cascade", "--feedback", "repair", "--json"}, t.TempDir(), &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("step reopen exit=%d output=%q", code, out.String())
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["invalidated"] == nil {
		t.Fatalf("step reopen output=%q err=%v", out.String(), err)
	}
	if method != "step.reopen" || params["execution_id"] != "exec-1" || params["step_id"] != "s1" || params["feedback"] != "repair" {
		t.Fatalf("method=%q params=%v", method, params)
	}
}

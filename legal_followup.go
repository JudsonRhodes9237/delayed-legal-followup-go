package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Matter struct {
	MatterID       string        `json:"matter_id"`
	ClientName     string        `json:"client_name"`
	SignedDocument string        `json:"signed_document"`
	ReceivedAt     time.Time     `json:"received_at"`
	FollowUpAfter  time.Duration `json:"follow_up_after"`
}

type FollowUpDecision struct {
	MatterID string    `json:"matter_id"`
	DueAt    time.Time `json:"due_at"`
	Action   string    `json:"action"`
}

func decideFollowUp(m Matter) FollowUpDecision {
	return FollowUpDecision{MatterID: m.MatterID, DueAt: m.ReceivedAt.Add(m.FollowUpAfter), Action: "request deadline status"}
}

func scheduleMatter(c *InfraiClient, m Matter, taskURL string) (FollowUpDecision, error) {
	decision := decideFollowUp(m)
	if _, err := c.CreateCron(taskURL); err != nil {
		return FollowUpDecision{}, err
	}
	if err := c.Publish("legal-followups", decision); err != nil {
		return FollowUpDecision{}, err
	}
	return decision, nil
}

func followUpHandler(c *InfraiClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var decision FollowUpDecision
		if err := json.NewDecoder(r.Body).Decode(&decision); err != nil {
			http.Error(w, "invalid follow-up payload", http.StatusBadRequest)
			return
		}
		if decision.Action == "" {
			err := fmt.Errorf("follow-up action is empty for matter %s", decision.MatterID)
			if captureErr := c.CaptureFailure(map[string]any{
				"exception": err.Error(),
				"extra":     map[string]string{"matter_id": decision.MatterID},
			}); captureErr != nil {
				http.Error(w, captureErr.Error(), http.StatusBadGateway)
				return
			}
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "follow-up sent", "matter_id": decision.MatterID})
	}
}

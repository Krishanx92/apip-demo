package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type leaveType struct {
	Code          string `json:"code"`
	Name          string `json:"name"`
	AnnualQuota   int    `json:"annualQuota"`
	CarryForward  bool   `json:"carryForward"`
	RequiresProof bool   `json:"requiresProof"`
}

type leaveBalance struct {
	LeaveTypeCode string  `json:"leaveTypeCode"`
	LeaveTypeName string  `json:"leaveTypeName"`
	TotalDays     float64 `json:"totalDays"`
	UsedDays      float64 `json:"usedDays"`
	RemainingDays float64 `json:"remainingDays"`
}

type leaveRequest struct {
	ID             string `json:"id"`
	EmployeeID     string `json:"employeeId"`
	LeaveTypeCode  string `json:"leaveTypeCode"`
	StartDate      string `json:"startDate"`
	EndDate        string `json:"endDate"`
	Days           int    `json:"days"`
	Reason         string `json:"reason,omitempty"`
	HalfDay        bool   `json:"halfDay"`
	Status         string `json:"status"`
	ManagerComment string `json:"managerComment,omitempty"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type createLeaveRequest struct {
	LeaveTypeCode string `json:"leaveTypeCode"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	Reason        string `json:"reason"`
	HalfDay       bool   `json:"halfDay"`
}

type cancelLeaveRequest struct {
	Reason string `json:"reason"`
}

type approvalDecision struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment"`
}

type store struct {
	mu       sync.Mutex
	requests map[string]leaveRequest
}

func main() {
	s := &store{requests: sampleRequests()}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", home)
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /leave-types", listLeaveTypes)
	mux.HandleFunc("GET /employees/{employeeId}/leave-balances", listLeaveBalances)
	mux.HandleFunc("GET /employees/{employeeId}/leave-requests", s.listLeaveRequests)
	mux.HandleFunc("POST /employees/{employeeId}/leave-requests", s.createLeaveRequest)
	mux.HandleFunc("GET /employees/{employeeId}/leave-requests/{requestId}", s.getLeaveRequest)
	mux.HandleFunc("POST /employees/{employeeId}/leave-requests/{requestId}/cancellation", s.cancelLeaveRequest)
	mux.HandleFunc("POST /leave-requests/{requestId}/approval", s.decideLeaveRequest)
	mux.HandleFunc("GET /managers/{managerId}/team-leave-requests", s.listTeamLeaveRequests)
	mux.HandleFunc("GET /reports/leave-utilization", leaveUtilizationReport)

	handler := withOptionalContext("/leave-management/v1.0", withJSONHeaders(mux))
	addr := ":8080"

	log.Printf("leave-management backend listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

func home(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"name":   "leave-management-backend",
		"status": "running",
	})
}

func withJSONHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func withOptionalContext(prefix string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, prefix+"/") {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		}
		next.ServeHTTP(w, r)
	})
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func listLeaveTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"data": []leaveType{
			{Code: "annual", Name: "Annual Leave", AnnualQuota: 20, CarryForward: true},
			{Code: "sick", Name: "Sick Leave", AnnualQuota: 10, RequiresProof: true},
			{Code: "casual", Name: "Casual Leave", AnnualQuota: 7},
		},
	})
}

func listLeaveBalances(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employeeId")
	writeJSON(w, http.StatusOK, map[string]any{
		"employeeId": employeeID,
		"data": []leaveBalance{
			{LeaveTypeCode: "annual", LeaveTypeName: "Annual Leave", TotalDays: 20, UsedDays: 5, RemainingDays: 15},
			{LeaveTypeCode: "sick", LeaveTypeName: "Sick Leave", TotalDays: 10, UsedDays: 1, RemainingDays: 9},
			{LeaveTypeCode: "casual", LeaveTypeName: "Casual Leave", TotalDays: 7, UsedDays: 2, RemainingDays: 5},
		},
	})
}

func (s *store) listLeaveRequests(w http.ResponseWriter, r *http.Request) {
	employeeID := r.PathValue("employeeId")
	status := r.URL.Query().Get("status")

	s.mu.Lock()
	defer s.mu.Unlock()

	requests := make([]leaveRequest, 0)
	for _, request := range s.requests {
		if request.EmployeeID != employeeID {
			continue
		}
		if status != "" && request.Status != status {
			continue
		}
		requests = append(requests, request)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(requests),
		"data":  requests,
	})
}

func (s *store) createLeaveRequest(w http.ResponseWriter, r *http.Request) {
	var payload createLeaveRequest
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.LeaveTypeCode == "" || payload.StartDate == "" || payload.EndDate == "" {
		writeError(w, http.StatusBadRequest, "leaveTypeCode, startDate and endDate are required")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	request := leaveRequest{
		ID:            "req-" + time.Now().UTC().Format("20060102150405"),
		EmployeeID:    r.PathValue("employeeId"),
		LeaveTypeCode: payload.LeaveTypeCode,
		StartDate:     payload.StartDate,
		EndDate:       payload.EndDate,
		Days:          1,
		Reason:        payload.Reason,
		HalfDay:       payload.HalfDay,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.mu.Lock()
	s.requests[request.ID] = request
	s.mu.Unlock()

	w.Header().Set("Location", "/employees/"+request.EmployeeID+"/leave-requests/"+request.ID)
	writeJSON(w, http.StatusCreated, request)
}

func (s *store) getLeaveRequest(w http.ResponseWriter, r *http.Request) {
	request, ok := s.findRequest(r.PathValue("requestId"))
	if !ok || request.EmployeeID != r.PathValue("employeeId") {
		writeError(w, http.StatusNotFound, "leave request not found")
		return
	}
	writeJSON(w, http.StatusOK, request)
}

func (s *store) cancelLeaveRequest(w http.ResponseWriter, r *http.Request) {
	var payload cancelLeaveRequest
	_ = decodeJSON(r, &payload)

	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.requests[r.PathValue("requestId")]
	if !ok || request.EmployeeID != r.PathValue("employeeId") {
		writeError(w, http.StatusNotFound, "leave request not found")
		return
	}
	if request.Status != "pending" {
		writeError(w, http.StatusConflict, "only pending leave requests can be cancelled")
		return
	}

	request.Status = "cancelled"
	request.ManagerComment = payload.Reason
	request.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.requests[request.ID] = request
	writeJSON(w, http.StatusOK, request)
}

func (s *store) decideLeaveRequest(w http.ResponseWriter, r *http.Request) {
	var payload approvalDecision
	if err := decodeJSON(r, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if payload.Decision != "approved" && payload.Decision != "rejected" {
		writeError(w, http.StatusBadRequest, "decision must be approved or rejected")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.requests[r.PathValue("requestId")]
	if !ok {
		writeError(w, http.StatusNotFound, "leave request not found")
		return
	}
	if request.Status != "pending" {
		writeError(w, http.StatusConflict, "only pending leave requests can be approved or rejected")
		return
	}

	request.Status = payload.Decision
	request.ManagerComment = payload.Comment
	request.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.requests[request.ID] = request
	writeJSON(w, http.StatusOK, request)
}

func (s *store) listTeamLeaveRequests(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	requests := make([]leaveRequest, 0, len(s.requests))
	for _, request := range s.requests {
		requests = append(requests, request)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"managerId": r.PathValue("managerId"),
		"count":     len(requests),
		"data":      requests,
	})
}

func leaveUtilizationReport(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"period": "2026",
		"summary": map[string]any{
			"employees":          24,
			"pendingRequests":    1,
			"approvedRequests":   1,
			"averageUtilization": 0.42,
		},
	})
}

func (s *store) findRequest(id string) (leaveRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	request, ok := s.requests[id]
	return request, ok
}

func decodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid JSON request body")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"code":    status,
		"message": message,
	})
}

func sampleRequests() map[string]leaveRequest {
	return map[string]leaveRequest{
		"req-1001": {
			ID:            "req-1001",
			EmployeeID:    "emp-100",
			LeaveTypeCode: "annual",
			StartDate:     "2026-06-10",
			EndDate:       "2026-06-14",
			Days:          5,
			Reason:        "Family event",
			Status:        "pending",
			CreatedAt:     "2026-05-04T08:00:00Z",
			UpdatedAt:     "2026-05-04T08:00:00Z",
		},
		"req-1002": {
			ID:             "req-1002",
			EmployeeID:     "emp-100",
			LeaveTypeCode:  "sick",
			StartDate:      "2026-04-12",
			EndDate:        "2026-04-12",
			Days:           1,
			Reason:         "Medical appointment",
			Status:         "approved",
			ManagerComment: "Approved",
			CreatedAt:      "2026-04-10T08:00:00Z",
			UpdatedAt:      "2026-04-10T10:00:00Z",
		},
	}
}

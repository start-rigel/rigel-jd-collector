package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rigel-labs/rigel-jd-collector/internal/config"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
	collectorservice "github.com/rigel-labs/rigel-jd-collector/internal/service/collector"
)

// App wires the phase 3 HTTP surface for the JD collector.
type App struct {
	cfg       config.Config
	collector *collectorservice.Service
}

func New(cfg config.Config, collector *collectorservice.Service) *App {
	return &App{cfg: cfg, collector: collector}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealth)
	mux.HandleFunc("/api/v1/collect/search", a.handleCollectSearch)
	mux.HandleFunc("/api/v1/collect/batch", a.handleCollectBatch)
	mux.HandleFunc("/api/v1/products", a.handleListProducts)
	mux.HandleFunc("/api/v1/jobs", a.handleListJobs)
	mux.HandleFunc("/api/v1/jobs/", a.handleJobRoutes)
	mux.HandleFunc("/", a.handleIndex)
	return mux
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": a.cfg.ServiceName,
		"mode":    a.cfg.JDCollectorMode,
	})
}

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service": a.cfg.ServiceName,
		"mode":    a.cfg.JDCollectorMode,
		"routes": []string{
			"GET /healthz",
			"POST /api/v1/collect/search",
			"POST /api/v1/collect/batch",
			"GET /api/v1/products",
			"GET /api/v1/jobs",
			"GET /api/v1/jobs/{id}",
			"POST /api/v1/jobs/{id}/retry",
		},
	})
}

func (a *App) handleCollectSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Keyword  string `json:"keyword"`
		Category string `json:"category"`
		Brand    string `json:"brand"`
		Limit    int    `json:"limit"`
		Persist  *bool  `json:"persist,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	persist := true
	if req.Persist != nil {
		persist = *req.Persist
	}

	response, err := a.collector.SearchAndStore(r.Context(), collectorservice.SearchRequest{
		Keyword:  strings.TrimSpace(req.Keyword),
		Category: strings.TrimSpace(req.Category),
		Brand:    strings.TrimSpace(req.Brand),
		Limit:    req.Limit,
		Persist:  persist,
	}, a.cfg.JDCollectorMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *App) handleCollectBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Preset          string `json:"preset,omitempty"`
		Persist         *bool  `json:"persist,omitempty"`
		ContinueOnError *bool  `json:"continue_on_error,omitempty"`
		Requests        []struct {
			Keyword  string `json:"keyword"`
			Category string `json:"category"`
			Brand    string `json:"brand,omitempty"`
			Limit    int    `json:"limit"`
			Persist  *bool  `json:"persist,omitempty"`
		} `json:"requests,omitempty"`
		SkipExistingReal *bool `json:"skip_existing_real,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	searchRequests := make([]collectorservice.SearchRequest, 0, len(req.Requests))
	if strings.TrimSpace(req.Preset) != "" {
		presetRequests, err := collectorservice.PresetRequests(strings.TrimSpace(req.Preset))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Persist != nil {
			for index := range presetRequests {
				presetRequests[index].Persist = *req.Persist
			}
		}
		searchRequests = append(searchRequests, presetRequests...)
	} else {
		defaultPersist := true
		if req.Persist != nil {
			defaultPersist = *req.Persist
		}
		for _, item := range req.Requests {
			persist := defaultPersist
			if item.Persist != nil {
				persist = *item.Persist
			}
			searchRequests = append(searchRequests, collectorservice.SearchRequest{
				Keyword:  strings.TrimSpace(item.Keyword),
				Category: strings.TrimSpace(item.Category),
				Brand:    strings.TrimSpace(item.Brand),
				Limit:    item.Limit,
				Persist:  persist,
			})
		}
	}

	continueOnError := strings.TrimSpace(req.Preset) != ""
	if req.ContinueOnError != nil {
		continueOnError = *req.ContinueOnError
	}
	skipExistingReal := strings.TrimSpace(req.Preset) != ""
	if req.SkipExistingReal != nil {
		skipExistingReal = *req.SkipExistingReal
	}

	response, err := a.collector.SearchBatchAndStore(r.Context(), searchRequests, a.cfg.JDCollectorMode, strings.TrimSpace(req.Preset), continueOnError, skipExistingReal)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *App) handleListProducts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	shopType := strings.TrimSpace(r.URL.Query().Get("shop_type"))
	selfOperatedOnly := parseBoolQuery(r, "self_operated_only")
	realOnly := parseBoolQuery(r, "real_only")
	products, err := a.collector.ListProducts(r.Context(), collectorservice.ProductListFilter{
		Keyword:          strings.TrimSpace(r.URL.Query().Get("keyword")),
		Category:         strings.TrimSpace(r.URL.Query().Get("category")),
		Limit:            limit,
		ShopType:         model.ShopType(shopType),
		SelfOperatedOnly: selfOperatedOnly,
		RealOnly:         realOnly,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(products),
		"items": products,
	})
}

func (a *App) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	jobs, err := a.collector.ListJobs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"count": len(jobs),
		"items": jobs,
	})
}

func (a *App) handleJobRoutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	if trimmed == "" || trimmed == "/" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}
	if strings.HasSuffix(trimmed, "/retry") {
		a.handleRetryJob(w, r, model.ID(strings.TrimSuffix(trimmed, "/retry")))
		return
	}
	a.handleGetJob(w, r, model.ID(trimmed))
}

func (a *App) handleGetJob(w http.ResponseWriter, r *http.Request, jobID model.ID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if jobID == "" || jobID == "/" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	job, err := a.collector.GetJob(r.Context(), jobID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (a *App) handleRetryJob(w http.ResponseWriter, r *http.Request, jobID model.ID) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if jobID == "" || jobID == "/" {
		writeError(w, http.StatusBadRequest, "job id is required")
		return
	}

	response, err := a.collector.RetryJob(r.Context(), jobID, a.cfg.JDCollectorMode)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseBoolQuery(r *http.Request, key string) bool {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

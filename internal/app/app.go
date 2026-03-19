package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/rigel-labs/rigel-jd-collector/internal/config"
	"github.com/rigel-labs/rigel-jd-collector/internal/domain/model"
	collectorservice "github.com/rigel-labs/rigel-jd-collector/internal/service/collector"
)

// App wires the current minimal HTTP surface for the JD collector.
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
	mux.HandleFunc("/api/v1/products", a.handleListProducts)
	mux.HandleFunc("/", a.handleIndex)
	return mux
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"service":          a.cfg.ServiceName,
		"mode":             a.cfg.JDCollectorMode,
		"schedule_enabled": a.cfg.ScheduleEnabled,
		"schedule_time":    a.cfg.ScheduleTime,
		"request_interval": a.cfg.RequestInterval.String(),
	})
}

func (a *App) handleIndex(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":          a.cfg.ServiceName,
		"mode":             a.cfg.JDCollectorMode,
		"schedule_enabled": a.cfg.ScheduleEnabled,
		"schedule_time":    a.cfg.ScheduleTime,
		"request_interval": a.cfg.RequestInterval.String(),
		"routes": []string{
			"GET /healthz",
			"POST /api/v1/collect/search",
			"GET /api/v1/products",
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

func parseBoolQuery(r *http.Request, key string) bool {
	value := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(key)))
	return value == "1" || value == "true" || value == "yes"
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

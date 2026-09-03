package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/botginx/botginx/modules/payments/models"
	"github.com/botginx/botginx/modules/payments/services"
	"github.com/botginx/botginx/pkg/ctx"
	"github.com/botginx/botginx/pkg/module"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service   *services.PaymentService
	templates *module.TemplateEngine
}

func NewHandler(service *services.PaymentService, templates *module.TemplateEngine) *Handler {
	return &Handler{
		service:   service,
		templates: templates,
	}
}

// ============ User Pages ============

// Deposit shows the deposit page with wallet address
func (h *Handler) Deposit(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	// Get available coins
	coins := h.service.GetAvailableCoins()

	// Get existing wallet (don't auto-create - user clicks to generate)
	var wallet *models.CryptoWallet
	if h.service.IsPaymentEnabled() {
		wallet, _ = h.service.GetExistingWallet(userID, "btc")
	}

	// Get recent transactions
	txs, _ := h.service.GetUserTransactions(userID, 10)

	module.RenderUserSection(w, r, h.templates, "payments:deposit.html", map[string]interface{}{
		"Title":        "Deposit",
		"Wallet":       wallet,
		"Coins":        coins,
		"Transactions": txs,
		"MinDeposit":   h.service.GetMinDeposit(),
		"MaxDeposit":   h.service.GetMaxDeposit(),
		"Enabled":      h.service.IsPaymentEnabled(),
	})
}

// Transactions shows transaction history
func (h *Handler) Transactions(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	txs, _ := h.service.GetUserTransactions(userID, 50)

	module.RenderUserSection(w, r, h.templates, "payments:transactions.html", map[string]interface{}{
		"Title":        "Transaction History",
		"Transactions": txs,
	})
}

// ============ User API ============

// APIGenerateAddress generates or returns wallet address for a coin
func (h *Handler) APIGenerateAddress(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	var input struct {
		Coin string `json:"coin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		input.Coin = "btc" // default
	}
	if input.Coin == "" {
		input.Coin = "btc"
	}

	wallet, err := h.service.GetOrCreateWallet(userID, input.Coin)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"address": wallet.Address,
		"coin":    wallet.Coin,
	})
}

// APIGetTransactions returns user's transactions
func (h *Handler) APIGetTransactions(w http.ResponseWriter, r *http.Request) {
	userID := ctx.GetUserID(r)

	txs, err := h.service.GetUserTransactions(userID, 50)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"transactions": txs,
	})
}

// ============ Webhook ============

// BitGoWebhook handles incoming BitGo webhook callbacks
func (h *Handler) BitGoWebhook(w http.ResponseWriter, r *http.Request) {
	// TODO: Verify webhook signature if BITGO_WEBHOOK_SECRET is set

	var payload models.BitGoWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("[payments] Invalid webhook payload: %v", err)
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	log.Printf("[payments] BitGo webhook: hash=%s type=%s state=%s", payload.Hash, payload.Type, payload.State)

	if payload.Hash == "" {
		http.Error(w, "Missing hash", http.StatusBadRequest)
		return
	}

	// Get BitGo gateway
	bitgoGateway := h.service.GetBitGoGateway()
	if bitgoGateway == nil {
		log.Printf("[payments] BitGo gateway not configured")
		http.Error(w, "Gateway not configured", http.StatusInternalServerError)
		return
	}

	// Fetch full transaction details
	tx, err := bitgoGateway.GetTransaction(payload.Hash)
	if err != nil {
		log.Printf("[payments] Failed to fetch tx %s: %v", payload.Hash, err)
		http.Error(w, "Failed to fetch transaction", http.StatusInternalServerError)
		return
	}

	// Process each output that belongs to us
	for _, output := range tx.Outputs {
		if output.IsOurs {
			_, err := h.service.ProcessDeposit(output.Address, payload.Hash, output.Amount, tx.Confirmations)
			if err != nil {
				log.Printf("[payments] Failed to process deposit: %v", err)
				// Continue processing other outputs
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ============ Admin Pages ============

// AdminDashboard shows payment overview
func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	txs, _ := h.service.GetAllTransactions(20)

	// Calculate stats
	var totalDeposits float64
	var pendingCount int
	for _, tx := range txs {
		if tx.Status == models.TxStatusConfirmed {
			totalDeposits += tx.AmountUSD
		}
		if tx.Status == models.TxStatusPending {
			pendingCount++
		}
	}

	// Check gateway status
	bitgoConfigured := os.Getenv("BITGO_API_KEY") != "" && os.Getenv("BITGO_WALLET_ID") != ""

	module.Render(w, r, h.templates, "payments:admin_dashboard.html", map[string]interface{}{
		"Title":           "Payments",
		"Transactions":    txs,
		"TotalDeposits":   totalDeposits,
		"PendingCount":    pendingCount,
		"BitGoConfigured": bitgoConfigured,
		"Coins":           h.service.GetAvailableCoins(),
	})
}

// AdminTransactions shows all transactions
func (h *Handler) AdminTransactions(w http.ResponseWriter, r *http.Request) {
	txs, _ := h.service.GetAllTransactions(100)

	module.Render(w, r, h.templates, "payments:admin_transactions.html", map[string]interface{}{
		"Title":        "All Transactions",
		"Transactions": txs,
	})
}

// ============ Admin API ============

// APIAdminResync manually resyncs a transaction
func (h *Handler) APIAdminResync(w http.ResponseWriter, r *http.Request) {
	txid := chi.URLParam(r, "txid")

	bitgoGateway := h.service.GetBitGoGateway()
	if bitgoGateway == nil {
		h.jsonError(w, "BitGo gateway not configured", http.StatusInternalServerError)
		return
	}

	// Fetch from BitGo
	tx, err := bitgoGateway.GetTransaction(txid)
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Process outputs
	processed := 0
	for _, output := range tx.Outputs {
		if output.IsOurs {
			_, err := h.service.ProcessDeposit(output.Address, txid, output.Amount, tx.Confirmations)
			if err == nil {
				processed++
			}
		}
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"processed": processed,
	})
}

// APIAdminAddWebhook registers webhook with BitGo
func (h *Handler) APIAdminAddWebhook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	bitgoGateway := h.service.GetBitGoGateway()
	if bitgoGateway == nil {
		h.jsonError(w, "BitGo gateway not configured", http.StatusInternalServerError)
		return
	}

	// Register webhook for 0 and 1 confirmations
	if err := bitgoGateway.GetClient().AddWebhook(input.URL, 0); err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Webhook registered",
	})
}

// APIAdminListWebhooks lists registered webhooks
func (h *Handler) APIAdminListWebhooks(w http.ResponseWriter, r *http.Request) {
	bitgoGateway := h.service.GetBitGoGateway()
	if bitgoGateway == nil {
		h.jsonError(w, "BitGo gateway not configured", http.StatusInternalServerError)
		return
	}

	webhooks, err := bitgoGateway.GetClient().ListWebhooks()
	if err != nil {
		h.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.json(w, http.StatusOK, map[string]interface{}{
		"webhooks": webhooks,
	})
}

// ============ Helpers ============

func (h *Handler) json(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.json(w, status, map[string]interface{}{"error": message})
}

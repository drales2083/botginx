package userbot

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	analyticssvc "github.com/botginx/botginx/modules/analytics/services"
	"github.com/botginx/botginx/modules/auth/models"
	domainsvc "github.com/botginx/botginx/modules/domains/services"
	rlmodels "github.com/botginx/botginx/modules/redirectlinks/models"
	rlsvc "github.com/botginx/botginx/modules/redirectlinks/services"
	"github.com/botginx/botginx/pkg/namegen"
	"github.com/botginx/botginx/pkg/subscription"
)

// Screen identifiers (also used as callback_data for navigation).
const (
	ScreenMain       = "main"
	ScreenStats      = "stats"
	ScreenLinks      = "links"
	ScreenNewLink    = "newlink"
	ScreenSettings   = "settings"
	ScreenBalance    = "balance"
	ScreenHelp       = "help"
	ScreenProtection = "protection"
	ScreenSupport    = "support"
	ScreenReferral   = "referral"

	linksPerPage = 5
	maxTextLen   = 4000
)

// InlineButton is a Telegram inline keyboard button.
type InlineButton struct {
	Text         string        `json:"text"`
	CallbackData string        `json:"callback_data,omitempty"`
	URL          string        `json:"url,omitempty"`
	CopyText     *CopyTextSpec `json:"copy_text,omitempty"`
	Style        string        `json:"style,omitempty"`
}

type CopyTextSpec struct {
	Text string `json:"text"`
}

// InlineKeyboardMarkup is a Telegram inline keyboard.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineButton `json:"inline_keyboard"`
}

// SettingsRenderer renders the settings screen (provided by the settings handler).
type SettingsRenderer func(user *models.User) (string, InlineKeyboardMarkup)

// SettingsCallback handles "set:*" callbacks. It returns the screen to show.
type SettingsCallback func(user *models.User, data string) (string, InlineKeyboardMarkup, string)

// Config wires the handler's dependencies.
type Config struct {
	Bot          *UserBotService
	Links        *rlsvc.RedirectLinkService
	Analytics    *analyticssvc.AnalyticsService
	Domains      *domainsvc.DomainService
	Subscription *subscription.Service

	// Optional hooks.
	Deploy             func(linkID string)               // deploy a freshly created link
	Cleanup            func(link *rlmodels.RedirectLink) // remote cleanup before delete
	RenderSettings     SettingsRenderer                  // settings screen (Task 5)
	SettingsCallback   SettingsCallback                  // "set:*" callbacks (Task 5)
	RenderProtection   func(userID string) (string, InlineKeyboardMarkup)
	ProtectionCallback func(userID, data string) (string, InlineKeyboardMarkup, string)
	RenderTicket       func(userID, ticketID string) (string, InlineKeyboardMarkup, bool)
	RenderReplyPrompt  func(ticketID string) (string, InlineKeyboardMarkup)
	AddTicketReply     func(userID, ticketID, message string) (string, error)
	RenderSupportMenu  func(userID string) (string, InlineKeyboardMarkup)
	RenderTicketList   func(userID string) (string, InlineKeyboardMarkup)
	RenderNewTicketSubject func() (string, InlineKeyboardMarkup)
	RenderNewTicketMessage func(subject string) (string, InlineKeyboardMarkup)
	CreateTicket       func(userID, subject, message string) (string, error)
	NotifyAdminNewTicket func(ticketID, userID, subject, message string)
	NotifyAdminTicketReply func(ticketID, userID, message string)
	ReferralStats      func(userID string) (code string, level1, level2, level3 int, earnings float64)
	DashboardURL       string // e.g. https://guardbot.sbs
	DocsURL            string // default https://docs.guardbot.sbs
}

type UserBotHandler struct {
	cfg               Config
	replyState        map[string]string // userID -> ticketID (waiting for reply)
	ticketSubject     map[string]string // userID -> subject (waiting for message)
	ticketSubjectWait map[string]bool   // userID -> true (waiting for subject)
	editDestState     map[string]string // userID -> linkID (waiting for new destination)
}

func NewUserBotHandler(cfg Config) *UserBotHandler {
	cfg.DashboardURL = strings.TrimSuffix(cfg.DashboardURL, "/")
	if cfg.DocsURL == "" {
		cfg.DocsURL = "https://docs.guardbot.sbs"
	}
	return &UserBotHandler{
		cfg:               cfg,
		replyState:        make(map[string]string),
		ticketSubject:     make(map[string]string),
		ticketSubjectWait: make(map[string]bool),
		editDestState:     make(map[string]string),
	}
}

// Telegram update payloads (only the fields we use).
type update struct {
	Message       *tgMessage       `json:"message"`
	CallbackQuery *tgCallbackQuery `json:"callback_query"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Text      string `json:"text"`
	Chat      struct {
		ID   int64  `json:"id"`
		Type string `json:"type"`
	} `json:"chat"`
}

type tgCallbackQuery struct {
	ID      string     `json:"id"`
	Data    string     `json:"data"`
	Message *tgMessage `json:"message"`
}

// HandleWebhook serves POST /telegram/user-webhook/{secret}.
// It always answers 200 once the secret is valid so Telegram does not retry.
func (h *UserBotHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	secret := chi.URLParam(r, "secret")
	user, err := h.cfg.Bot.GetUserByWebhookSecret(secret)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var upd update
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&upd); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Process synchronously but never let a panic bubble to Telegram retries.
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[userbot] panic handling update for user %s: %v", user.ID, rec)
			}
		}()
		switch {
		case upd.CallbackQuery != nil:
			h.handleCallback(user, upd.CallbackQuery)
		case upd.Message != nil:
			h.handleMessage(user, upd.Message)
		}
	}()
	w.WriteHeader(http.StatusOK)
}

// ---- message handling ----

func (h *UserBotHandler) handleMessage(user *models.User, m *tgMessage) {
	if m.Chat.Type != "private" {
		return
	}
	text := strings.TrimSpace(m.Text)
	cmd, arg := splitCommand(text)

	// Link the chat on first /start; ignore anyone else once a chat is linked.
	if user.TelegramChatID == nil {
		if cmd != "/start" {
			return
		}
		if _, err := h.cfg.Bot.SaveChatID(user.ID, m.Chat.ID); err != nil {
			log.Printf("[userbot] save chat id: %v", err)
			return
		}
	} else if *user.TelegramChatID != m.Chat.ID {
		return
	}

	// Keep the chat clean: remove the user's message.
	if err := h.cfg.Bot.DeleteMessage(user.ID, m.MessageID); err != nil {
		log.Printf("[userbot] delete user message: %v", err)
	}

	// Check if user is in reply state (waiting for ticket reply)
	if ticketID, ok := h.replyState[user.ID]; ok && cmd == "" {
		delete(h.replyState, user.ID)
		if h.cfg.AddTicketReply != nil {
			_, err := h.cfg.AddTicketReply(user.ID, ticketID, text)
			if err != nil {
				h.showWithNotice(user, ScreenMain, "⚠️ "+err.Error())
				return
			}
			// Notify admin about the reply
			if h.cfg.NotifyAdminTicketReply != nil {
				h.cfg.NotifyAdminTicketReply(ticketID, user.ID, text)
			}
			// Show ticket again with success message
			if h.cfg.RenderTicket != nil {
				txt, kb, _ := h.cfg.RenderTicket(user.ID, ticketID)
				h.send(user, "✅ Reply sent!\n\n"+txt, kb)
				return
			}
		}
		h.showWithNotice(user, ScreenMain, "✅ Reply sent")
		return
	}

	// Check if user is waiting to enter ticket subject
	if h.ticketSubjectWait[user.ID] && cmd == "" {
		delete(h.ticketSubjectWait, user.ID)
		if len(text) < 5 || len(text) > 100 {
			h.send(user, "⚠️ Subject must be 5-100 characters. Try again.",
				InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Cancel", CallbackData: ScreenSupport}}}})
			h.ticketSubjectWait[user.ID] = true
			return
		}
		h.ticketSubject[user.ID] = text
		if h.cfg.RenderNewTicketMessage != nil {
			txt, kb := h.cfg.RenderNewTicketMessage(text)
			h.send(user, txt, kb)
		} else {
			h.send(user, "Now send your message.",
				InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Cancel", CallbackData: ScreenSupport}}}})
		}
		return
	}

	// Check if user is waiting to enter ticket message
	if subject, ok := h.ticketSubject[user.ID]; ok && cmd == "" {
		delete(h.ticketSubject, user.ID)
		if h.cfg.CreateTicket != nil {
			ticketID, err := h.cfg.CreateTicket(user.ID, subject, text)
			if err != nil {
				h.showWithNotice(user, ScreenSupport, "⚠️ "+err.Error())
				return
			}
			// Notify admin
			if h.cfg.NotifyAdminNewTicket != nil {
				h.cfg.NotifyAdminNewTicket(ticketID, user.ID, subject, text)
			}
			// Show the created ticket
			if h.cfg.RenderTicket != nil {
				txt, kb, _ := h.cfg.RenderTicket(user.ID, ticketID)
				h.send(user, "✅ Ticket created!\n\n"+txt, kb)
				return
			}
		}
		h.showWithNotice(user, ScreenSupport, "✅ Ticket created")
		return
	}

	// Check if user is editing a link destination
	if linkID, ok := h.editDestState[user.ID]; ok && cmd == "" {
		delete(h.editDestState, user.ID)
		if !isURL(text) {
			h.send(user, "⚠️ That doesn't look like a valid URL. Send a valid http(s) URL.",
				InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Cancel", CallbackData: "ld:" + linkID}}}})
			h.editDestState[user.ID] = linkID // restore state
			return
		}
		h.updateLinkDestination(user, linkID, text)
		return
	}

	switch {
	case cmd == "/start":
		h.show(user, ScreenMain, "")
	case cmd == "/help":
		h.show(user, ScreenHelp, "")
	case cmd == "/stats":
		h.show(user, ScreenStats, "")
	case cmd == "/links":
		h.showLinks(user, 0)
	case cmd == "/balance":
		h.show(user, ScreenBalance, "")
	case cmd == "/settings":
		h.show(user, ScreenSettings, "")
	case cmd == "/protection":
		h.show(user, ScreenProtection, "")
	case cmd == "/support":
		h.show(user, ScreenSupport, "")
	case cmd == "/referral":
		h.show(user, ScreenReferral, "")
	case cmd == "/newlink":
		if arg == "" {
			h.show(user, ScreenNewLink, "")
		} else {
			h.createLink(user, arg)
		}
	case isURL(text):
		h.createLink(user, text)
	default:
		h.showWithNotice(user, ScreenMain, "❓ Unknown command. Use the menu below.")
	}
}

// splitCommand parses "/cmd@botname arg" into ("/cmd", "arg"). Non-commands return ("", text).
func splitCommand(text string) (cmd, arg string) {
	if !strings.HasPrefix(text, "/") {
		return "", text
	}
	parts := strings.Fields(text)
	cmd = strings.ToLower(parts[0])
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	if len(parts) > 1 {
		arg = strings.TrimSpace(strings.TrimPrefix(text, parts[0]))
	}
	return cmd, arg
}

func isURL(s string) bool {
	if strings.ContainsAny(s, " \n\t") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// ---- callback handling ----

func (h *UserBotHandler) handleCallback(user *models.User, cq *tgCallbackQuery) {
	token, err := h.cfg.Bot.GetBotToken(user.ID)
	if err != nil {
		return
	}
	// Ignore callbacks from chats other than the linked one.
	if cq.Message == nil || user.TelegramChatID == nil || cq.Message.Chat.ID != *user.TelegramChatID {
		_ = h.cfg.Bot.AnswerCallback(token, cq.ID, "", false)
		return
	}

	// Always answer immediately to clear the loading spinner.
	toast, alert := "", false
	defer func() {
		_ = h.cfg.Bot.AnswerCallback(token, cq.ID, toast, alert)
	}()

	// If user clicked on a different message than our tracked one, clean up the old one
	if cq.Message.MessageID != 0 {
		if oldID := h.cfg.Bot.GetLastMessageID(user.ID); oldID != 0 && oldID != cq.Message.MessageID {
			// Delete the orphaned message (best effort)
			_ = h.cfg.Bot.DeleteMessage(user.ID, oldID)
		}
		_ = h.cfg.Bot.SaveLastMessageID(user.ID, cq.Message.MessageID)
	}

	data := cq.Data
	switch {
	case data == "nav:"+ScreenMain:
		h.show(user, ScreenMain, "")
	case data == "nav:"+ScreenStats:
		h.show(user, ScreenStats, "")
	case data == "nav:"+ScreenLinks:
		h.showLinks(user, 0)
	case data == "nav:"+ScreenNewLink:
		h.show(user, ScreenNewLink, "")
	case data == "nav:"+ScreenBalance:
		h.show(user, ScreenBalance, "")
	case data == "nav:"+ScreenHelp:
		h.show(user, ScreenHelp, "")
	case data == "nav:"+ScreenSettings:
		h.show(user, ScreenSettings, "")
	case data == "nav:"+ScreenProtection:
		h.show(user, ScreenProtection, "")
	case data == "nav:"+ScreenSupport:
		h.show(user, ScreenSupport, "")
	case data == "nav:"+ScreenReferral:
		h.show(user, ScreenReferral, "")
	case strings.HasPrefix(data, "lp:"):
		page, _ := strconv.Atoi(strings.TrimPrefix(data, "lp:"))
		h.showLinks(user, page)
	case strings.HasPrefix(data, "ld:"):
		h.showLinkDetail(user, strings.TrimPrefix(data, "ld:"))
	case strings.HasPrefix(data, "ls:"):
		h.showLinkStats(user, strings.TrimPrefix(data, "ls:"))
	case strings.HasPrefix(data, "lx:"):
		h.confirmDelete(user, strings.TrimPrefix(data, "lx:"))
	case strings.HasPrefix(data, "ly:"):
		toast = h.deleteLink(user, strings.TrimPrefix(data, "ly:"))
	case strings.HasPrefix(data, "le:"):
		h.promptEditDestination(user, strings.TrimPrefix(data, "le:"))
	case strings.HasPrefix(data, "set:"):
		if h.cfg.SettingsCallback == nil {
			toast = "Settings are not available yet"
			return
		}
		text, kb, t := h.cfg.SettingsCallback(user, strings.TrimPrefix(data, "set:"))
		toast = t
		h.send(user, text, kb)
	case strings.HasPrefix(data, "prot:"):
		if h.cfg.ProtectionCallback == nil {
			toast = "Protection settings are not available"
			return
		}
		text, kb, t := h.cfg.ProtectionCallback(user.ID, strings.TrimPrefix(data, "prot:"))
		toast = t
		h.send(user, text, kb)
	case strings.HasPrefix(data, "support:"):
		h.handleSupportCallback(user, strings.TrimPrefix(data, "support:"))
	case data == "noop":
	default:
		toast = "Unknown action"
	}
}

// ---- screen display ----

func (h *UserBotHandler) show(user *models.User, screen, notice string) {
	// Reload user so balance/settings are fresh.
	text, kb := h.renderScreen(user, screen, nil)
	if notice != "" {
		text = notice + "\n\n" + text
	}
	h.send(user, text, kb)
}

func (h *UserBotHandler) showWithNotice(user *models.User, screen, notice string) {
	h.show(user, screen, notice)
}

func (h *UserBotHandler) send(user *models.User, text string, kb InlineKeyboardMarkup) {
	if len(text) > maxTextLen {
		text = text[:maxTextLen]
	}
	if _, err := h.cfg.Bot.SendOrEditMessage(user.ID, text, kb); err != nil {
		log.Printf("[userbot] send screen to user %s: %v", user.ID, err)
	}
}

func backRow() []InlineButton {
	return []InlineButton{{Text: "⬅️ Main menu", CallbackData: "nav:" + ScreenMain}}
}

// renderScreen builds the text and keyboard for a screen.
func (h *UserBotHandler) renderScreen(user *models.User, screen string, data interface{}) (string, InlineKeyboardMarkup) {
	switch screen {
	case ScreenStats:
		return h.renderStats(user)
	case ScreenNewLink:
		return "➕ <b>New protected link</b>\n\nSend me a URL to protect, or use <code>/newlink https://example.com</code>.",
			InlineKeyboardMarkup{[][]InlineButton{backRow()}}
	case ScreenBalance:
		return h.renderBalance(user)
	case ScreenHelp:
		return h.renderHelp()
	case ScreenSettings:
		if h.cfg.RenderSettings != nil {
			return h.cfg.RenderSettings(user)
		}
		return "⚙️ <b>Settings</b>\n\nNotification settings are not available yet.",
			InlineKeyboardMarkup{[][]InlineButton{backRow()}}
	case ScreenProtection:
		if h.cfg.RenderProtection != nil {
			return h.cfg.RenderProtection(user.ID)
		}
		return "🛡️ <b>Protection</b>\n\nProtection settings are not available yet.",
			InlineKeyboardMarkup{[][]InlineButton{backRow()}}
	case ScreenSupport:
		if h.cfg.RenderSupportMenu != nil {
			return h.cfg.RenderSupportMenu(user.ID)
		}
		return "🎫 <b>Support</b>\n\nSupport is not available yet.",
			InlineKeyboardMarkup{[][]InlineButton{backRow()}}
	case ScreenReferral:
		return h.renderReferral(user)
	default:
		return h.renderMain(user)
	}
}

func (h *UserBotHandler) renderMain(user *models.User) (string, InlineKeyboardMarkup) {
	appName := os.Getenv("UI_APP_NAME")
	if appName == "" {
		appName = "GuardBot"
	}

	// Get subscription status (admins don't need subscriptions)
	subLine := ""
	if h.cfg.Subscription != nil && user.Role != "admin" {
		state := h.cfg.Subscription.StateFor(user.ID)
		switch state.Status {
		case "active":
			subLine = fmt.Sprintf("\n\n✅ Subscription: %s", state.ExpiresAt.Format("Jan 2, 2006"))
		case "expiring":
			if state.DaysLeft == 1 {
				subLine = "\n\n⚠️ Expires tomorrow — renew now"
			} else {
				subLine = fmt.Sprintf("\n\n⚠️ Expires in %d days — renew now", state.DaysLeft)
			}
		case "expired":
			subLine = "\n\n❌ Subscription expired — contact admin"
		default:
			subLine = "\n\n🔒 No subscription — contact admin"
		}
	}

	text := fmt.Sprintf("🛡 <b>Welcome to %s</b>\n\n"+
		"The most advanced bot protection on the market. "+
		"Block bots, verify humans, protect your links — all in real-time.%s\n\n"+
		"📲 Send me any URL to create a protected link instantly.", appName, subLine)
	return text, InlineKeyboardMarkup{[][]InlineButton{
		{
			{Text: "📊 Stats", CallbackData: "nav:" + ScreenStats, Style: "primary"},
			{Text: "🔗 Links", CallbackData: "nav:" + ScreenLinks, Style: "primary"},
			{Text: "➕ New Link", CallbackData: "nav:" + ScreenNewLink},
		},
		{
			{Text: "🤖 Antibot Control", CallbackData: "nav:" + ScreenProtection},
			{Text: "⚙️ Settings", CallbackData: "nav:" + ScreenSettings},
			{Text: "💰 Balance", CallbackData: "nav:" + ScreenBalance},
		},
		{
			{Text: "🎁 Referral", CallbackData: "nav:" + ScreenReferral},
			{Text: "🎫 Support", CallbackData: "nav:" + ScreenSupport},
			{Text: "❓ Help", CallbackData: "nav:" + ScreenHelp},
		},
	}}
}

func (h *UserBotHandler) renderReferral(user *models.User) (string, InlineKeyboardMarkup) {
	if h.cfg.ReferralStats == nil {
		return "🎁 <b>Referral Program</b>\n\nReferral program is not available.",
			InlineKeyboardMarkup{[][]InlineButton{backRow()}}
	}

	code, level1, level2, level3, earnings := h.cfg.ReferralStats(user.ID)
	totalReferrals := level1 + level2 + level3

	// Build referral link
	refLink := h.cfg.DashboardURL + "/signup?ref=" + code

	text := fmt.Sprintf("🎁 <b>Referral Program</b>\n\n"+
		"Share your link and earn commissions when your referrals pay!\n\n"+
		"<b>Your code:</b> <code>%s</code>\n"+
		"<b>Your link:</b>\n<code>%s</code>\n\n"+
		"📊 <b>Stats</b>\n"+
		"• Referrals: %d\n"+
		"• Earnings: $%.2f",
		code, refLink, totalReferrals, earnings)

	return text, InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "📋 Copy Link", CopyText: &CopyTextSpec{Text: refLink}, Style: "primary"}},
		backRow(),
	}}
}

func (h *UserBotHandler) renderStats(user *models.User) (string, InlineKeyboardMarkup) {
	kb := InlineKeyboardMarkup{[][]InlineButton{backRow()}}
	stats, err := h.cfg.Analytics.GetUserStats(user.ID)
	if err != nil {
		log.Printf("[userbot] stats: %v", err)
		return "📊 <b>Stats</b>\n\n⚠️ Could not load stats right now.", kb
	}
	active := 0
	if links, err := h.cfg.Links.List(user.ID); err == nil {
		active = len(links)
	}
	humans := stats.TotalVisits - stats.BotVisits
	if humans < 0 {
		humans = 0
	}
	text := fmt.Sprintf("📊 <b>Stats</b>\n\n"+
		"📈 Today's clicks: <b>%d</b>\n"+
		"🤖 Bots blocked: <b>%d</b>\n"+
		"👤 Humans verified: <b>%d</b>\n"+
		"🔗 Active links: <b>%d</b>",
		stats.TodayVisits, stats.BlockedVisits, humans, active)
	return text, kb
}

func (h *UserBotHandler) renderBalance(user *models.User) (string, InlineKeyboardMarkup) {
	rows := [][]InlineButton{}
	if h.cfg.DashboardURL != "" {
		rows = append(rows, []InlineButton{{Text: "➕ Add Funds", URL: h.cfg.DashboardURL + "/payments", Style: "primary"}})
	}
	rows = append(rows, backRow())
	return fmt.Sprintf("💰 <b>Balance</b>\n\nCurrent balance: <b>$%.2f</b>", user.Balance), InlineKeyboardMarkup{rows}
}

func (h *UserBotHandler) renderHelp() (string, InlineKeyboardMarkup) {
	text := "❓ <b>Help</b>\n\n" +
		"/start — main menu\n" +
		"/stats — traffic overview\n" +
		"/links — your links\n" +
		"/newlink [url] — create a protected link\n" +
		"/protection — bot protection defaults\n" +
		"/referral — your referral link and stats\n" +
		"/support — create or view tickets\n" +
		"/balance — account balance\n" +
		"/settings — notifications\n" +
		"/help — this screen\n\n" +
		"💡 You can also just send me a URL and I'll protect it."
	return text, InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "📖 Open docs", URL: h.cfg.DocsURL}},
		backRow(),
	}}
}

// ---- links ----

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (h *UserBotHandler) showLinks(user *models.User, page int) {
	links, err := h.cfg.Links.List(user.ID)
	if err != nil {
		log.Printf("[userbot] list links: %v", err)
		h.send(user, "🔗 <b>Links</b>\n\n⚠️ Could not load links.", InlineKeyboardMarkup{[][]InlineButton{backRow()}})
		return
	}
	if len(links) == 0 {
		h.send(user, "🔗 <b>Links</b>\n\nYou have no links yet. Send me a URL to create one.",
			InlineKeyboardMarkup{[][]InlineButton{
				{{Text: "➕ New Link", CallbackData: "nav:" + ScreenNewLink, Style: "primary"}},
				backRow(),
			}})
		return
	}
	pages := (len(links) + linksPerPage - 1) / linksPerPage
	if page < 0 {
		page = 0
	}
	if page >= pages {
		page = pages - 1
	}
	start := page * linksPerPage
	end := start + linksPerPage
	if end > len(links) {
		end = len(links)
	}

	var rows [][]InlineButton
	for _, l := range links[start:end] {
		label := fmt.Sprintf("%s · 👁 %d", truncate(strings.TrimPrefix(l.FullURL(), "https://"), 32), l.ViewCount)
		rows = append(rows, []InlineButton{{Text: label, CallbackData: "ld:" + l.ID}})
	}
	if pages > 1 {
		nav := []InlineButton{}
		if page > 0 {
			nav = append(nav, InlineButton{Text: "◀️ Prev", CallbackData: "lp:" + strconv.Itoa(page-1)})
		}
		nav = append(nav, InlineButton{Text: fmt.Sprintf("Page %d/%d", page+1, pages), CallbackData: "noop"})
		if page < pages-1 {
			nav = append(nav, InlineButton{Text: "Next ▶️", CallbackData: "lp:" + strconv.Itoa(page+1)})
		}
		rows = append(rows, nav)
	}
	rows = append(rows, backRow())
	h.send(user, fmt.Sprintf("🔗 <b>Your links</b> (%d)\n\nTap a link for details.", len(links)), InlineKeyboardMarkup{rows})
}

// ownedLink loads a link and verifies it belongs to the user.
func (h *UserBotHandler) ownedLink(user *models.User, id string) *rlmodels.RedirectLink {
	link, err := h.cfg.Links.Get(id)
	if err != nil || link.UserID != user.ID {
		return nil
	}
	return link
}

func (h *UserBotHandler) linkNotFound(user *models.User) {
	h.send(user, "🔗 Link not found.", InlineKeyboardMarkup{[][]InlineButton{
		{{Text: "⬅️ Links", CallbackData: "nav:" + ScreenLinks}},
	}})
}

func (h *UserBotHandler) showLinkDetail(user *models.User, id string) {
	link := h.ownedLink(user, id)
	if link == nil {
		h.linkNotFound(user)
		return
	}
	dest := ""
	if len(link.DestinationURLs) > 0 {
		dest = link.DestinationURLs[0]
	}
	stats, _ := h.cfg.Analytics.GetLinkStats(link.ID)
	clicks, blocked := 0, 0
	if stats != nil {
		clicks, blocked = stats.TotalVisits, stats.BlockedVisits
	}
	text := fmt.Sprintf("🔗 <b>Link details</b>\n\n"+
		"<b>URL</b>\n<code>%s</code>\n\n"+
		"<b>Destination</b>\n%s\n\n"+
		"👁 Clicks: <b>%d</b>\n🤖 Bots blocked: <b>%d</b>\n📅 Created: %s",
		html.EscapeString(link.FullURL()), html.EscapeString(dest), clicks, blocked,
		link.CreatedAt.Format("2006-01-02"))
	h.send(user, text, InlineKeyboardMarkup{[][]InlineButton{
		{
			{Text: "📊 Stats", CallbackData: "ls:" + link.ID, Style: "primary"},
			{Text: "📋 Copy", CopyText: &CopyTextSpec{Text: link.FullURL()}},
		},
		{
			{Text: "✏️ Edit Destination", CallbackData: "le:" + link.ID},
		},
		{
			{Text: "🗑️ Delete", CallbackData: "lx:" + link.ID},
			{Text: "⬅️ Back", CallbackData: "nav:" + ScreenLinks},
		},
	}})
}

func (h *UserBotHandler) showLinkStats(user *models.User, id string) {
	link := h.ownedLink(user, id)
	if link == nil {
		h.linkNotFound(user)
		return
	}
	kb := InlineKeyboardMarkup{[][]InlineButton{{{Text: "⬅️ Back", CallbackData: "ld:" + link.ID}}}}
	s, err := h.cfg.Analytics.GetLinkStats(link.ID)
	if err != nil {
		h.send(user, "📊 ⚠️ Could not load stats.", kb)
		return
	}
	humans := s.TotalVisits - s.BotVisits
	if humans < 0 {
		humans = 0
	}
	h.send(user, fmt.Sprintf("📊 <b>Link stats</b>\n<code>%s</code>\n\n"+
		"📈 Today: <b>%d</b>\n👁 Total: <b>%d</b>\n🤖 Bots blocked: <b>%d</b>\n👤 Humans: <b>%d</b>",
		html.EscapeString(link.FullURL()), s.TodayVisits, s.TotalVisits, s.BlockedVisits, humans), kb)
}

func (h *UserBotHandler) confirmDelete(user *models.User, id string) {
	link := h.ownedLink(user, id)
	if link == nil {
		h.linkNotFound(user)
		return
	}
	h.send(user, fmt.Sprintf("🗑️ <b>Delete this link?</b>\n\n<code>%s</code>\n\nThis cannot be undone.",
		html.EscapeString(link.FullURL())), InlineKeyboardMarkup{[][]InlineButton{{
		{Text: "✅ Yes, delete", CallbackData: "ly:" + link.ID},
		{Text: "✖️ Cancel", CallbackData: "ld:" + link.ID},
	}}})
}

// deleteLink deletes an owned link and returns a toast message.
func (h *UserBotHandler) deleteLink(user *models.User, id string) string {
	link := h.ownedLink(user, id)
	if link == nil {
		h.linkNotFound(user)
		return "Link not found"
	}
	if h.cfg.Cleanup != nil {
		h.cfg.Cleanup(link)
	}
	if err := h.cfg.Links.Delete(link.ID); err != nil {
		log.Printf("[userbot] delete link %s: %v", link.ID, err)
		return "Could not delete link"
	}
	h.showLinks(user, 0)
	return "Link deleted"
}

func (h *UserBotHandler) promptEditDestination(user *models.User, id string) {
	link := h.ownedLink(user, id)
	if link == nil {
		h.linkNotFound(user)
		return
	}
	h.editDestState[user.ID] = id
	dest := ""
	if len(link.DestinationURLs) > 0 {
		dest = link.DestinationURLs[0]
	}
	h.send(user, fmt.Sprintf("✏️ <b>Edit Destination</b>\n\n"+
		"<b>Current:</b>\n%s\n\n"+
		"Send me the new destination URL.", html.EscapeString(dest)),
		InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Cancel", CallbackData: "ld:" + id}}}})
}

func (h *UserBotHandler) updateLinkDestination(user *models.User, linkID, newURL string) {
	link := h.ownedLink(user, linkID)
	if link == nil {
		h.linkNotFound(user)
		return
	}
	// Update the destination
	_, err := h.cfg.Links.Update(linkID, rlmodels.UpdateRedirectLinkInput{
		DestinationURLs: []string{newURL},
	})
	if err != nil {
		log.Printf("[userbot] update link destination %s: %v", linkID, err)
		h.send(user, "⚠️ Could not update destination. Try again.",
			InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Back", CallbackData: "ld:" + linkID}}}})
		return
	}
	// Redeploy the link
	if h.cfg.Deploy != nil {
		go h.cfg.Deploy(linkID)
	}
	// Show updated link detail
	h.send(user, "✅ Destination updated!", InlineKeyboardMarkup{[][]InlineButton{}})
	h.showLinkDetail(user, linkID)
}

// createLink creates a protected link for destURL using the user's default domain or first available.
func (h *UserBotHandler) createLink(user *models.User, destURL string) {
	back := InlineKeyboardMarkup{[][]InlineButton{backRow()}}

	// Check subscription
	if h.cfg.Subscription != nil {
		state := h.cfg.Subscription.StateFor(user.ID)
		if !state.Active {
			h.send(user, "➕ <b>New protected link</b>\n\n⚠️ Your subscription is not active.\n\nContact admin to renew your subscription.", back)
			return
		}
	}

	if !isURL(destURL) {
		h.send(user, "➕ <b>New protected link</b>\n\n⚠️ That doesn't look like a valid http(s) URL. Send me a URL to protect.", back)
		return
	}
	domains, err := h.cfg.Domains.ListAvailable(user.ID)
	if err != nil {
		log.Printf("[userbot] list domains: %v", err)
		h.send(user, "⚠️ Could not load your domains.", back)
		return
	}
	var domain *struct{ ID, Name string }
	// Use default domain if set and still valid
	if user.TelegramDefaultDomainID != nil && *user.TelegramDefaultDomainID != "" {
		for _, d := range domains {
			if d.ID == *user.TelegramDefaultDomainID && d.DNSVerified && d.SSLEnabled {
				domain = &struct{ ID, Name string }{d.ID, d.Name}
				break
			}
		}
	}
	// Fall back to first verified SSL domain
	if domain == nil {
		for _, d := range domains {
			if d.DNSVerified && d.SSLEnabled {
				domain = &struct{ ID, Name string }{d.ID, d.Name}
				break
			}
		}
	}
	if domain == nil {
		h.send(user, "➕ <b>New protected link</b>\n\n⚠️ You need a verified domain with SSL first. Add one in the dashboard.", back)
		return
	}

	link, err := h.cfg.Links.Create(user.ID, rlmodels.CreateRedirectLinkInput{
		DomainID:        domain.ID,
		Subdomain:       namegen.Subdomain(),
		Path:            namegen.TrackingPath(),
		Type:            string(rlmodels.LinkTypeRedirect),
		DestinationURLs: []string{destURL},
		BotProtection:   true,
		PassParams:      true,
		AppendSignature: true,
	})
	if err != nil {
		log.Printf("[userbot] create link: %v", err)
		h.send(user, "⚠️ Could not create the link. Please try again.", back)
		return
	}
	link.DomainName = domain.Name
	if h.cfg.Deploy != nil {
		go h.cfg.Deploy(link.ID)
	}

	full := link.FullURL()
	h.send(user, fmt.Sprintf("✅ <b>Link created</b>\n\n<b>Protected URL</b>\n<code>%s</code>\n\n<b>Destination</b>\n%s\n\n"+
		"It may take a minute to go live.", html.EscapeString(full), html.EscapeString(destURL)),
		InlineKeyboardMarkup{[][]InlineButton{
			{{Text: "📋 Copy", CopyText: &CopyTextSpec{Text: full}, Style: "primary"}, {Text: "🔗 Details", CallbackData: "ld:" + link.ID}},
			backRow(),
		}})
}

// handleSupportCallback handles support ticket viewing and replies.
func (h *UserBotHandler) handleSupportCallback(user *models.User, data string) {
	// Clear any pending ticket creation states when navigating
	if data != "new" {
		delete(h.ticketSubjectWait, user.ID)
		delete(h.ticketSubject, user.ID)
	}

	switch {
	case data == "new":
		// Start new ticket creation
		h.ticketSubjectWait[user.ID] = true
		if h.cfg.RenderNewTicketSubject != nil {
			text, kb := h.cfg.RenderNewTicketSubject()
			h.send(user, text, kb)
		} else {
			h.send(user, "➕ <b>New Ticket</b>\n\nSend a short subject for your ticket.",
				InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Cancel", CallbackData: ScreenSupport}}}})
		}
	case data == "list":
		// Show ticket list
		if h.cfg.RenderTicketList != nil {
			text, kb := h.cfg.RenderTicketList(user.ID)
			h.send(user, text, kb)
		} else {
			h.showWithNotice(user, ScreenSupport, "⚠️ Ticket list not available")
		}
	case strings.HasPrefix(data, "reply:"):
		// User wants to reply to a ticket - set reply state
		ticketID := strings.TrimPrefix(data, "reply:")
		h.replyState[user.ID] = ticketID
		if h.cfg.RenderReplyPrompt != nil {
			text, kb := h.cfg.RenderReplyPrompt(ticketID)
			h.send(user, text, kb)
		} else {
			h.send(user, "💬 <b>Reply</b>\n\nType your message and send it.",
				InlineKeyboardMarkup{[][]InlineButton{{{Text: "← Cancel", CallbackData: "support:" + ticketID}}}})
		}
	default:
		// View ticket details
		ticketID := data
		if h.cfg.RenderTicket != nil {
			text, kb, _ := h.cfg.RenderTicket(user.ID, ticketID)
			h.send(user, text, kb)
		} else {
			h.showWithNotice(user, ScreenMain, "⚠️ Support tickets are not available")
		}
	}
}

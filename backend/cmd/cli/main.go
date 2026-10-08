// Command cli is the operator's tool for managing a running Atish instance
// from a terminal: users, moderation, premium, settings, reports and
// broadcasts. The Makefile wraps it (make admin-…); it talks to the same
// database as the API and records every change in the audit log.
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"atish/internal/config"
	"atish/internal/db"
	"atish/internal/models"
	"atish/internal/repository"
	"atish/internal/services"
	"atish/internal/storage"
	"atish/internal/telegram"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type env struct {
	ctx  context.Context
	cfg  *config.Config
	repo *repository.Repo
	rdb  *redis.Client // optional: only used to cut off banned users immediately
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "help" || os.Args[1] == "-h" {
		usage()
		return
	}
	cmd, args := os.Args[1], os.Args[2:]
	if cmd == "keygen" { // needs no database
		keygen()
		return
	}

	cfg, err := config.Load()
	check(err)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	check(err)
	defer pool.Close()
	check(db.Migrate(ctx, pool))

	e := &env{ctx: ctx, cfg: cfg, repo: repository.New(pool)}
	if opts, err := redis.ParseURL(cfg.RedisURL); err == nil {
		c := redis.NewClient(opts)
		pctx, pc := context.WithTimeout(ctx, 2*time.Second)
		if c.Ping(pctx).Err() == nil {
			e.rdb = c
		}
		pc()
	}

	switch cmd {
	case "stats":
		e.stats()
	case "list-users":
		e.listUsers(args)
	case "user":
		e.user(args)
	case "promote":
		e.promote(args)
	case "demote":
		e.setRole(args, models.RoleUser)
	case "set-status":
		e.setStatus(args)
	case "verify":
		e.verify(args)
	case "grant-premium":
		e.grantPremium(args)
	case "revoke-premium":
		e.revokePremium(args)
	case "delete-user":
		e.deleteUser(args)
	case "plans":
		e.plans()
	case "settings":
		e.settings()
	case "set-setting":
		e.setSetting(args)
	case "reports":
		e.reports(args)
	case "resolve-report":
		e.resolveReport(args)
	case "audit":
		e.audit(args)
	case "broadcast":
		e.broadcast(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`Atish admin CLI

Users
  stats                                   platform counters
  list-users  [-q text] [-status s] [-role r] [-premium yes|no] [-limit 25] [-page 1]
  user        -user REF                   full detail (REF = id, Atish_name, name, telegram id or @username)
  promote     -telegram-id N [-role admin|moderator]
  demote      -user REF                   back to a normal user
  set-status  -user REF -status active|suspended|banned [-reason "..."]
  verify      -user REF -flag telegram|phone|photo|identity [-value true|false]
  delete-user -user REF -yes              erase the account (irreversible)

Premium
  plans
  grant-premium  -user REF -plan CODE [-days 30]   (days 0 = lifetime)
  revoke-premium -user REF

Platform
  settings                                list every setting
  set-setting -key K -value V             V is JSON (true, 60, "text", ["a","b"]) or plain text
  reports     [-status open]              moderation queue
  resolve-report -id N -status resolved|dismissed|reviewing [-resolution "..."] [-action suspend|ban]
  audit       [-limit 30]                 recent admin actions
  broadcast   -text "..." -yes            message every active user through the bot
  keygen                                  print fresh JWT_SECRET / ENCRYPTION_KEY values
`)
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func need(v, name string) {
	if strings.TrimSpace(v) == "" {
		fmt.Fprintf(os.Stderr, "error: -%s is required\n", name)
		os.Exit(2)
	}
}

func tw() *tabwriter.Writer { return tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0) }

func randB64(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func keygen() {
	fmt.Println("JWT_SECRET=" + strings.NewReplacer("+", "", "/", "", "=", "").Replace(randB64(48)))
	fmt.Println("ENCRYPTION_KEY=" + randB64(32))
}

func (e *env) resolve(ref string) *models.User {
	need(ref, "user")
	u, err := e.repo.FindUser(e.ctx, ref)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no user matches %q\n", ref)
		os.Exit(1)
	}
	return u
}

func (e *env) audit0(action, target string, details any) {
	e.repo.Audit(e.ctx, "cli", action, "user", target, details)
}

func (e *env) invalidate(id uuid.UUID) {
	if e.rdb != nil {
		e.rdb.Del(e.ctx, "ustate:"+id.String())
	}
}

func name(u *models.User) string {
	if u.AtishUsername != nil {
		return *u.AtishUsername
	}
	return u.ID.String()[:8]
}

// ---- commands ----

func (e *env) stats() {
	raw, err := e.repo.Stats(e.ctx)
	check(err)
	var m map[string]any
	check(json.Unmarshal(raw, &m))
	delete(m, "daily")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w := tw()
	for _, k := range keys {
		fmt.Fprintf(w, "%s\t%v\n", k, m[k])
	}
	w.Flush()
}

func (e *env) listUsers(args []string) {
	fs := flag.NewFlagSet("list-users", flag.ExitOnError)
	q := fs.String("q", "", "search")
	status := fs.String("status", "", "")
	role := fs.String("role", "", "")
	premium := fs.String("premium", "", "yes|no")
	limit := fs.Int("limit", 25, "")
	page := fs.Int("page", 1, "")
	_ = fs.Parse(args)
	raw, total, err := e.repo.AdminUsers(e.ctx, repository.UserQuery{Q: *q, Status: *status, Role: *role, Premium: *premium, Limit: *limit, Offset: (*page - 1) * *limit})
	check(err)
	var rows []map[string]any
	check(json.Unmarshal(raw, &rows))
	w := tw()
	fmt.Fprintln(w, "USERNAME\tNAME\tCITY\tSTATUS\tROLE\tPLUS\tTELEGRAM ID\tLAST ACTIVE")
	for _, r := range rows {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", dash(r["atish_username"]), dash(r["display_name"]), dash(r["city"]), r["status"], r["role"],
			r["premium"], dash(r["telegram_user_id"]), shortTime(r["last_active_at"]))
	}
	w.Flush()
	fmt.Printf("\n%d of %d users (page %d)\n", len(rows), total, *page)
}

func dash(v any) any {
	if v == nil || v == "" {
		return "-"
	}
	if f, ok := v.(float64); ok {
		return strconv.FormatInt(int64(f), 10)
	}
	return v
}

func shortTime(v any) string {
	s, _ := v.(string)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return "-"
}

func (e *env) user(args []string) {
	fs := flag.NewFlagSet("user", flag.ExitOnError)
	ref := fs.String("user", "", "")
	_ = fs.Parse(args)
	u := e.resolve(*ref)
	p, err := e.repo.LoadProfile(e.ctx, u.ID)
	check(err)
	w := tw()
	row := func(k string, v any) { fmt.Fprintf(w, "%s\t%v\n", k, v) }
	row("id", u.ID)
	row("username", name(u))
	row("name", dash(p.DisplayName))
	row("status", u.Status+reasonSuffix(u.StatusReason))
	row("role", u.Role)
	if u.TelegramUserID != nil {
		row("telegram id", *u.TelegramUserID)
	}
	row("telegram username", dash(u.TelegramUsername))
	row("age / gender", fmt.Sprintf("%d / %s", p.Age, dash(p.Gender)))
	if p.Location != nil {
		row("location", fmt.Sprintf("%s %s, %s", p.Location.City, p.Location.Area, p.Location.Country))
	}
	row("looking for", strings.Join(p.ConnectionTypes, ", "))
	row("interests", len(p.Interests))
	row("photos", len(p.Photos))
	row("verified", fmt.Sprintf("telegram=%v phone=%v photo=%v identity=%v", u.TelegramVerified, u.PhoneVerified, u.PhotoVerified, u.IdentityVerified))
	if sub, err := e.repo.ActiveSubscription(e.ctx, u.ID); err == nil {
		until := "lifetime"
		if sub.CurrentPeriodEnd != nil {
			until = sub.CurrentPeriodEnd.Local().Format("2006-01-02")
		}
		row("plus", fmt.Sprintf("%s via %s until %s", sub.PlanCode, sub.Provider, until))
	} else {
		row("plus", "no")
	}
	row("onboarded", u.OnboardedAt != nil)
	row("created", u.CreatedAt.Local().Format("2006-01-02 15:04"))
	row("last active", u.LastActiveAt.Local().Format("2006-01-02 15:04"))
	w.Flush()
}

func reasonSuffix(r string) string {
	if r == "" {
		return ""
	}
	return " (" + r + ")"
}

func (e *env) promote(args []string) {
	fs := flag.NewFlagSet("promote", flag.ExitOnError)
	tg := fs.Int64("telegram-id", 0, "")
	role := fs.String("role", models.RoleAdmin, "")
	_ = fs.Parse(args)
	if *tg <= 0 {
		need("", "telegram-id")
	}
	if *role != models.RoleAdmin && *role != models.RoleModerator {
		check(fmt.Errorf("role must be admin or moderator"))
	}
	id, err := e.repo.PromoteTelegram(e.ctx, *tg, *role)
	check(err)
	e.invalidate(id)
	e.audit0("user.promote", id.String(), map[string]any{"telegram_id": *tg, "role": *role})
	fmt.Printf("Telegram user %d is now %s. They get access on their next login (open /admin from Telegram).\n", *tg, *role)
}

func (e *env) setRole(args []string, role string) {
	fs := flag.NewFlagSet("demote", flag.ExitOnError)
	ref := fs.String("user", "", "")
	_ = fs.Parse(args)
	u := e.resolve(*ref)
	check(e.repo.AdminSetUser(e.ctx, u.ID, map[string]any{"role": role}))
	e.invalidate(u.ID)
	e.audit0("user.update", u.ID.String(), map[string]any{"role": role})
	fmt.Printf("%s is now %s\n", name(u), role)
}

func (e *env) setStatus(args []string) {
	fs := flag.NewFlagSet("set-status", flag.ExitOnError)
	ref := fs.String("user", "", "")
	status := fs.String("status", "", "active|suspended|banned")
	reason := fs.String("reason", "", "")
	_ = fs.Parse(args)
	switch *status {
	case models.StatusActive, models.StatusSuspended, models.StatusBanned:
	default:
		check(fmt.Errorf("status must be active, suspended or banned"))
	}
	u := e.resolve(*ref)
	check(e.repo.AdminSetUser(e.ctx, u.ID, map[string]any{"status": *status, "status_reason": *reason}))
	e.invalidate(u.ID)
	e.audit0("user.update", u.ID.String(), map[string]any{"status": *status, "reason": *reason})
	fmt.Printf("%s is now %s\n", name(u), *status)
}

func (e *env) verify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	ref := fs.String("user", "", "")
	flagName := fs.String("flag", "", "telegram|phone|photo|identity")
	value := fs.Bool("value", true, "")
	_ = fs.Parse(args)
	switch *flagName {
	case "telegram", "phone", "photo", "identity":
	default:
		check(fmt.Errorf("flag must be telegram, phone, photo or identity"))
	}
	u := e.resolve(*ref)
	field := *flagName + "_verified"
	check(e.repo.AdminSetUser(e.ctx, u.ID, map[string]any{field: *value}))
	e.audit0("user.update", u.ID.String(), map[string]any{field: *value})
	fmt.Printf("%s: %s = %v\n", name(u), field, *value)
}

func (e *env) grantPremium(args []string) {
	fs := flag.NewFlagSet("grant-premium", flag.ExitOnError)
	ref := fs.String("user", "", "")
	plan := fs.String("plan", "premium_month", "")
	days := fs.Int("days", 30, "0 = lifetime")
	_ = fs.Parse(args)
	u := e.resolve(*ref)
	pid, err := e.repo.PlanIDByCode(e.ctx, *plan)
	check(err)
	check(e.repo.GrantPremium(e.ctx, u.ID, pid, *days))
	e.audit0("premium.grant", u.ID.String(), map[string]any{"plan": *plan, "days": *days})
	fmt.Printf("%s now has %s (%d days; 0 = lifetime)\n", name(u), *plan, *days)
}

func (e *env) revokePremium(args []string) {
	fs := flag.NewFlagSet("revoke-premium", flag.ExitOnError)
	ref := fs.String("user", "", "")
	_ = fs.Parse(args)
	u := e.resolve(*ref)
	check(e.repo.RevokePremium(e.ctx, u.ID))
	e.audit0("premium.revoke", u.ID.String(), nil)
	fmt.Printf("%s no longer has Plus\n", name(u))
}

func (e *env) deleteUser(args []string) {
	fs := flag.NewFlagSet("delete-user", flag.ExitOnError)
	ref := fs.String("user", "", "")
	yes := fs.Bool("yes", false, "confirm")
	_ = fs.Parse(args)
	u := e.resolve(*ref)
	if !*yes {
		check(fmt.Errorf("this erases %s permanently; re-run with -yes", name(u)))
	}
	keys, err := e.repo.DeleteAccount(e.ctx, u.ID)
	check(err)
	if store, err := storage.NewLocal(e.cfg.MediaDir); err == nil {
		for _, k := range keys {
			_ = store.Delete(e.ctx, k)
		}
	}
	e.invalidate(u.ID)
	e.audit0("user.delete", u.ID.String(), nil)
	fmt.Printf("Deleted %s (%d photo file(s) removed)\n", name(u), len(keys))
}

func (e *env) plans() {
	ps, err := e.repo.ListPlans(e.ctx, false)
	check(err)
	w := tw()
	fmt.Fprintln(w, "CODE\tNAME\tINTERVAL\tPRICE\tSTARS\tACTIVE")
	for _, p := range ps {
		stars := "-"
		if p.StarsPrice != nil {
			stars = strconv.Itoa(*p.StarsPrice)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%.2f %s\t%s\t%v\n", p.Code, p.Name, p.Interval, float64(p.PriceCents)/100, strings.ToUpper(p.Currency), stars, p.IsActive)
	}
	w.Flush()
}

func (e *env) settings() {
	all, err := e.repo.AllSettings(e.ctx)
	check(err)
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	w := tw()
	for _, k := range keys {
		fmt.Fprintf(w, "%s\t%s\n", k, string(all[k]))
	}
	w.Flush()
}

func (e *env) setSetting(args []string) {
	fs := flag.NewFlagSet("set-setting", flag.ExitOnError)
	key := fs.String("key", "", "")
	val := fs.String("value", "", "")
	_ = fs.Parse(args)
	need(*key, "key")
	raw := json.RawMessage(*val)
	if !json.Valid(raw) { // plain text such as: -value "We are back at noon"
		raw, _ = json.Marshal(*val)
	}
	check(services.NewSettings(e.repo).Set(e.ctx, *key, raw))
	e.repo.Audit(e.ctx, "cli", "setting.update", "setting", *key, raw)
	fmt.Printf("%s = %s\n", *key, string(raw))
}

func (e *env) reports(args []string) {
	fs := flag.NewFlagSet("reports", flag.ExitOnError)
	status := fs.String("status", "open", "open|reviewing|resolved|dismissed|'' for all")
	limit := fs.Int("limit", 25, "")
	_ = fs.Parse(args)
	raw, err := e.repo.AdminReports(e.ctx, *status, *limit, 0)
	check(err)
	var rows []map[string]any
	check(json.Unmarshal(raw, &rows))
	w := tw()
	fmt.Fprintln(w, "ID\tREPORTED\tREASON\tBY\tSTATUS\tFILED")
	for _, r := range rows {
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\n", r["id"], r["reported_username"], r["reason"], r["reporter_username"], r["status"], shortTime(r["created_at"]))
	}
	w.Flush()
}

func (e *env) resolveReport(args []string) {
	fs := flag.NewFlagSet("resolve-report", flag.ExitOnError)
	id := fs.Int64("id", 0, "")
	status := fs.String("status", "resolved", "")
	resolution := fs.String("resolution", "", "")
	action := fs.String("action", "", "suspend|ban")
	_ = fs.Parse(args)
	if *id <= 0 {
		need("", "id")
	}
	if *action == "suspend" || *action == "ban" {
		target, err := e.repo.ReportedUser(e.ctx, *id)
		check(err)
		st := models.StatusSuspended
		if *action == "ban" {
			st = models.StatusBanned
		}
		check(e.repo.AdminSetUser(e.ctx, target, map[string]any{"status": st, "status_reason": *resolution}))
		e.invalidate(target)
		fmt.Printf("reported user set to %s\n", st)
	}
	check(e.repo.ResolveReport(e.ctx, *id, *status, *resolution, "cli"))
	e.repo.Audit(e.ctx, "cli", "report.resolve", "report", strconv.FormatInt(*id, 10), map[string]any{"status": *status, "action": *action})
	fmt.Printf("report #%d -> %s\n", *id, *status)
}

func (e *env) audit(args []string) {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	limit := fs.Int("limit", 30, "")
	_ = fs.Parse(args)
	raw, err := e.repo.AuditLog(e.ctx, *limit, 0)
	check(err)
	var rows []map[string]any
	check(json.Unmarshal(raw, &rows))
	w := tw()
	fmt.Fprintln(w, "WHEN\tACTOR\tACTION\tTARGET\tDETAILS")
	for _, r := range rows {
		d, _ := json.Marshal(r["details"])
		fmt.Fprintf(w, "%s\t%v\t%v\t%v %v\t%s\n", shortTime(r["created_at"]), r["actor"], r["action"], r["target_type"], dash(r["target_id"]), d)
	}
	w.Flush()
}

func (e *env) broadcast(args []string) {
	fs := flag.NewFlagSet("broadcast", flag.ExitOnError)
	text := fs.String("text", "", "")
	yes := fs.Bool("yes", false, "confirm")
	_ = fs.Parse(args)
	need(*text, "text")
	bot := telegram.NewBot(e.cfg.TelegramBotToken)
	if !bot.Enabled() {
		check(fmt.Errorf("TELEGRAM_BOT_TOKEN is not set"))
	}
	targets, err := e.repo.BroadcastTargets(e.ctx)
	check(err)
	if !*yes {
		check(fmt.Errorf("this would message %d users; re-run with -yes", len(targets)))
	}
	e.repo.Audit(e.ctx, "cli", "broadcast", "all", "", map[string]any{"recipients": len(targets), "text": *text})
	sent := 0
	tick := time.NewTicker(40 * time.Millisecond) // stay under Telegram's ~30 msg/s limit
	defer tick.Stop()
	for _, tg := range targets {
		<-tick.C
		if bot.SendMessage(e.ctx, tg, *text, nil) == nil {
			sent++
		}
	}
	fmt.Printf("delivered %d of %d\n", sent, len(targets))
}

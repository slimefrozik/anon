package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	tele "gopkg.in/telebot.v3"

	"github.com/slimefrozik/anon/internal/model"
	"github.com/slimefrozik/anon/internal/service"
)

const (
	actionNone    = ""
	actionPost    = "post_text"
	actionComment = "comment"
)

var (
	menuBtnPost  = tele.ReplyButton{Text: "📝 Создать пост"}
	menuBtnFeed  = tele.ReplyButton{Text: "📰 Лента"}
	menuBtnNotif = tele.ReplyButton{Text: "🔔 Уведомления"}
	menuBtnHelp  = tele.ReplyButton{Text: "❓ Помощь"}

	menuKeys = &tele.ReplyMarkup{
		ResizeKeyboard:  true,
		ReplyKeyboard:   [][]tele.ReplyButton{
			{menuBtnPost, menuBtnFeed},
			{menuBtnNotif, menuBtnHelp},
		},
	}

	cancelBtn    = tele.ReplyButton{Text: "❌ Отмена"}
	cancelKeys   = &tele.ReplyMarkup{
		ResizeKeyboard:  true,
		ReplyKeyboard:   [][]tele.ReplyButton{{cancelBtn}},
	}
)

type Bot struct {
	tb          *tele.Bot
	pool        *pgxpool.Pool
	feedSvc     *service.FeedService
	reactionSvc *service.ReactionService
	commentSvc  *service.CommentService
	abuseSvc    *service.AbuseService
	mediaBase   string
}

func New(token string, pool *pgxpool.Pool, feedSvc *service.FeedService, reactionSvc *service.ReactionService, commentSvc *service.CommentService, abuseSvc *service.AbuseService, mediaBase string) (*Bot, error) {
	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	tb, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create bot: %w", err)
	}

	b := &Bot{
		tb:          tb,
		pool:        pool,
		feedSvc:     feedSvc,
		reactionSvc: reactionSvc,
		commentSvc:  commentSvc,
		abuseSvc:    abuseSvc,
		mediaBase:   mediaBase,
	}

	b.registerHandlers()
	return b, nil
}

func (b *Bot) Start() {
	log.Println("telegram bot started")
	b.tb.Start()
}

func (b *Bot) Stop() {
	b.tb.Stop()
}

func (b *Bot) registerHandlers() {
	b.tb.Handle("/start", b.onStart)
	b.tb.Handle("/cancel", b.onCancel)

	b.tb.Handle(tele.OnPhoto, b.onPhoto)
	b.tb.Handle(tele.OnText, b.onAllText)

	b.tb.Handle("\fextend", b.onReactionCallback)
	b.tb.Handle("\fpromote", b.onReactionCallback)
	b.tb.Handle("\fskip", b.onReactionCallback)
	b.tb.Handle("\fsuppress", b.onReactionCallback)
	b.tb.Handle("\fcomment", b.onCommentCallback)
}

func (b *Bot) getUserID(tgUserID int64) string {
	var userID string
	err := b.pool.QueryRow(context.Background(),
		`SELECT user_id FROM telegram_sessions WHERE telegram_id = $1`,
		tgUserID,
	).Scan(&userID)
	if err != nil {
		return ""
	}
	return userID
}

func (b *Bot) ensureUser(tgUserID int64) (string, error) {
	userID := b.getUserID(tgUserID)
	if userID != "" {
		return userID, nil
	}

	userID = uuid.New().String()
	_, err := b.pool.Exec(context.Background(),
		`INSERT INTO users (id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	if err != nil {
		return "", err
	}

	_, err = b.pool.Exec(context.Background(),
		`INSERT INTO user_influence (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	if err != nil {
		return "", err
	}

	_, err = b.pool.Exec(context.Background(),
		`INSERT INTO telegram_sessions (telegram_id, user_id) VALUES ($1, $2)`,
		tgUserID, userID,
	)
	if err != nil {
		return "", err
	}

	return userID, nil
}

func (b *Bot) getState(userID string) (action, refID string) {
	err := b.pool.QueryRow(context.Background(),
		`SELECT action, ref_id FROM user_state WHERE user_id = $1`,
		userID,
	).Scan(&action, &refID)
	if err != nil {
		return actionNone, ""
	}
	return action, refID
}

func (b *Bot) setState(userID, action, refID string) {
	b.pool.Exec(context.Background(),
		`INSERT INTO user_state (user_id, action, ref_id) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id) DO UPDATE SET action = $2, ref_id = $3, created_at = now()`,
		userID, action, refID,
	)
}

func (b *Bot) clearState(userID string) {
	b.pool.Exec(context.Background(),
		`UPDATE user_state SET action = '', ref_id = '' WHERE user_id = $1`,
		userID,
	)
}

func (b *Bot) sendMenu(c tele.Context, text string) error {
	return c.Send(text, menuKeys)
}

func (b *Bot) sendCancelUI(c tele.Context, text string) error {
	return c.Send(text, cancelKeys)
}

func (b *Bot) onStart(c tele.Context) error {
	_, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return b.sendMenu(c, "Что-то пошло не так. Попробуй ещё раз.")
	}

	return b.sendMenu(c,
		"Добро пожаловать в анонимное пространство.\n\n"+
			"Нет имён. Нет профилей. Контент живёт или умирает по анонимным реакциям.\n\n"+
			"Используй кнопки ниже 👇",
	)
}

func (b *Bot) onMenuHelp(c tele.Context) error {
	return b.sendMenu(c,
		"📖 Как это работает:\n\n"+
			"• Пост живёт 24ч и угасает без поддержки\n"+
			"• 🔄 Продлить — добавляет 2ч жизни + небольшой буст\n"+
			"• 🚀 Продвинуть — большой буст охвата\n"+
			"• ⏭ Пропустить — без эффекта\n"+
			"• ⬇ Подавить — уменьшает охват и время жизни\n"+
			"• 💬 Комментарий — оставить ОДИН комментарий (видит только автор поста)\n\n"+
			"Комментарии приватные — только автор поста их видит.\n"+
			"Если автор ответит — ты получишь уведомление.\n"+
			"Дальнейшая переписка невозможна — одноразовое взаимодействие.\n\n"+
			"Никто не видит, кто запостил. Никто не видит, кто реагировал.",
	)
}

func (b *Bot) onMenuPost(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return b.sendMenu(c, "Ошибка создания сессии.")
	}

	b.setState(userID, actionPost, "")
	return b.sendCancelUI(c, "📝 Напиши текст поста или отправь фото с подписью.\n\nЛимит: 1 пост в час. До 1000 символов.")
}

func (b *Bot) onMenuFeed(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return b.sendMenu(c, "Ошибка создания сессии.")
	}

	posts, err := b.feedSvc.Generate(context.Background(), userID, 5)
	if err != nil || len(posts) == 0 {
		return b.sendMenu(c, "Постов пока нет. Попробуй позже.")
	}

	for _, p := range posts {
		msg := formatPost(p)

		btns := &tele.ReplyMarkup{}
		row := btns.Row(
			btns.Data("🔄", "extend", p.ID),
			btns.Data("🚀", "promote", p.ID),
			btns.Data("⏭", "skip", p.ID),
			btns.Data("⬇", "suppress", p.ID),
			btns.Data("💬", "comment", p.ID),
		)
		btns.Inline(row)

		if p.ContentType == "image" && p.MediaURL != "" {
			photo := &tele.Photo{File: tele.FromURL(p.MediaURL), Caption: msg}
			_, err = b.tb.Send(c.Recipient(), photo, btns)
		} else {
			_, err = b.tb.Send(c.Recipient(), msg, btns)
		}
		if err != nil {
			log.Printf("send feed post error: %v", err)
		}
	}

	return nil
}

func (b *Bot) onMenuNotif(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return b.sendMenu(c, "Ошибка создания сессии.")
	}

	rows, err := b.pool.Query(context.Background(),
		`SELECT n.id, n.comment_id, n.post_id, n.created_at
		 FROM notifications n
		 WHERE n.user_id = $1 AND n.read = false
		 ORDER BY n.created_at DESC LIMIT 10`,
		userID,
	)
	if err != nil {
		return b.sendMenu(c, "Ошибка загрузки уведомлений.")
	}
	defer rows.Close()

	type notif struct {
		ID        string
		CommentID string
		PostID    string
		CreatedAt time.Time
	}

	var notifs []notif
	for rows.Next() {
		var n notif
		if err := rows.Scan(&n.ID, &n.CommentID, &n.PostID, &n.CreatedAt); err != nil {
			continue
		}
		notifs = append(notifs, n)
	}

	if len(notifs) == 0 {
		return b.sendMenu(c, "Нет новых уведомлений.")
	}

	for _, n := range notifs {
		ctx, err := b.commentSvc.GetNotificationContext(context.Background(), n.ID, userID)
		if err != nil {
			continue
		}

		msg := fmt.Sprintf("📩 Кто-то ответил на твой комментарий:\n\n"+
			"💬 Ты написал: %s\n"+
			"↩️ Ответ: %s\n\n"+
			"📝 Пост: %s",
			truncate(ctx.YourComment, 100),
			truncate(ctx.Reply, 100),
			truncate(ctx.Post.TextContent, 80),
		)

		c.Send(msg)
	}

	return b.sendMenu(c, fmt.Sprintf("Показано уведомлений: %d", len(notifs)))
}

func (b *Bot) onCancel(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return b.sendMenu(c, "Ошибка.")
	}

	b.clearState(userID)
	return b.sendMenu(c, "❌ Действие отменено.")
}

func (b *Bot) onPhoto(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return b.sendMenu(c, "Ошибка создания сессии.")
	}

	action, _ := b.getState(userID)

	if action != actionPost && action != actionNone {
		return b.sendMenu(c, "Сначала отмени текущее действие (❌ Отмена).")
	}

	caption := c.Message().Caption

	if caption != "" {
		if err := b.abuseSvc.ValidateComment(caption); err != nil {
			return b.sendCancelUI(c, "Контент отклонён. Попробуй другой текст.")
		}
	}

	photo := c.Message().Photo
	if photo == nil {
		return b.sendCancelUI(c, "Фото не найдено. Попробуй ещё раз.")
	}

	shadow := b.abuseSvc.ShouldShadowBan(caption)
	postID, expiresAt, err := b.createPost(userID, "image", caption, shadow)
	if err != nil {
		return b.sendMenu(c, "Не удалось создать пост. Лимит: 1 пост в час.")
	}

	b.clearState(userID)

	if shadow {
		return b.sendMenu(c, fmt.Sprintf("Пост создан. Истекает: %s", expiresAt.Format("15:04 02 Jan")))
	}

	_ = postID
	return b.sendMenu(c, fmt.Sprintf("✅ Пост с картинкой опубликован!\nИстекает: %s", expiresAt.Format("15:04 02 Jan")))
}

func (b *Bot) onAllText(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		return nil
	}

	text := c.Message().Text

	switch text {
	case "📝 Создать пост":
		return b.onMenuPost(c)
	case "📰 Лента":
		return b.onMenuFeed(c)
	case "🔔 Уведомления":
		return b.onMenuNotif(c)
	case "❓ Помощь":
		return b.onMenuHelp(c)
	case "❌ Отмена":
		return b.onCancel(c)
	case "/cancel":
		return b.onCancel(c)
	}

	action, refID := b.getState(userID)

	switch action {
	case actionPost:
		return b.handlePostText(c, userID, text)
	case actionComment:
		return b.handleCommentText(c, userID, refID, text)
	default:
		return nil
	}
}

func (b *Bot) handlePostText(c tele.Context, userID, text string) error {
	if err := b.abuseSvc.ValidatePost(&model.CreatePostRequest{
		ContentType: "text",
		TextContent: text,
	}); err != nil {
		return b.sendCancelUI(c, "Контент отклонён. Попробуй другой текст или ❌ Отмена.")
	}

	shadow := b.abuseSvc.ShouldShadowBan(text)
	postID, expiresAt, err := b.createPost(userID, "text", text, shadow)
	if err != nil {
		return b.sendMenu(c, "Не удалось создать пост. Лимит: 1 пост в час.")
	}

	b.clearState(userID)

	if shadow {
		return b.sendMenu(c, fmt.Sprintf("Пост создан. Истекает: %s", expiresAt.Format("15:04 02 Jan")))
	}

	_ = postID
	return b.sendMenu(c, fmt.Sprintf("✅ Пост опубликован!\nИстекает: %s", expiresAt.Format("15:04 02 Jan")))
}

func (b *Bot) handleCommentText(c tele.Context, userID, postID, text string) error {
	if err := b.abuseSvc.ValidateComment(text); err != nil {
		return b.sendCancelUI(c, "Комментарий отклонён. Попробуй другой текст или ❌ Отмена.")
	}

	_, err := b.commentSvc.Create(context.Background(), postID, userID, text)
	if err != nil {
		return b.sendMenu(c, "Ошибка: "+err.Error())
	}

	b.clearState(userID)
	return b.sendMenu(c, "✅ Комментарий отправлен. Его видит только автор поста.")
}

func (b *Bot) onReactionCallback(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		c.Respond(&tele.CallbackResponse{Text: "Ошибка"})
		return nil
	}

	data := c.Callback().Data
	if data == "" {
		c.Respond(&tele.CallbackResponse{Text: "Некорректно"})
		return nil
	}

	reactionMap := map[string]int{
		"extend":   model.ReactionExtend,
		"promote":  model.ReactionPromote,
		"skip":     model.ReactionSkip,
		"suppress": model.ReactionSuppress,
	}

	rt, ok := reactionMap[c.Callback().Unique]
	if !ok {
		c.Respond(&tele.CallbackResponse{Text: "Неизвестная реакция"})
		return nil
	}

	err = b.reactionSvc.Process(context.Background(), data, userID, rt)
	if err != nil {
		c.Respond(&tele.CallbackResponse{Text: err.Error()})
		return nil
	}

	labels := map[string]string{
		"extend":   "🔄 Продлён",
		"promote":  "🚀 Продвинут",
		"skip":     "⏭ Пропущен",
		"suppress": "⬇ Подавлен",
	}

	c.Respond(&tele.CallbackResponse{Text: labels[c.Callback().Unique]})
	return nil
}

func (b *Bot) onCommentCallback(c tele.Context) error {
	userID, err := b.ensureUser(c.Sender().ID)
	if err != nil {
		c.Respond(&tele.CallbackResponse{Text: "Ошибка"})
		return nil
	}

	postID := c.Callback().Data
	if postID == "" {
		c.Respond(&tele.CallbackResponse{Text: "Некорректно"})
		return nil
	}

	b.setState(userID, actionComment, postID)

	c.Respond(&tele.CallbackResponse{Text: "💬 Напиши комментарий:"})
	b.sendCancelUI(c, "💬 Напиши комментарий к посту "+postID[:8]+":\n\nДо 300 символов. Или ❌ Отмена.")
	return nil
}

func (b *Bot) createPost(userID, contentType, text string, shadow bool) (string, time.Time, error) {
	ct := 0
	if contentType == "image" {
		ct = 1
	}

	now := time.Now()
	expiresAt := now.Add(service.DefaultPostTTL)
	health := 1.0
	status := 0

	if shadow {
		health = 0
		status = 1
	}

	postID := uuid.New().String()

	_, err := b.pool.Exec(context.Background(),
		`INSERT INTO posts (id, author_id, content_type, text_content, expires_at, health, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		postID, userID, ct, text, expiresAt, health, status,
	)
	if err != nil {
		return "", time.Time{}, err
	}

	return postID, expiresAt, nil
}

func formatPost(p model.PostResponse) string {
	ct := "📝"
	if p.ContentType == "image" {
		ct = "🖼"
	}

	body := ""
	if p.TextContent != "" {
		body = truncate(p.TextContent, 400)
	}

	return fmt.Sprintf("%s %s\n\nID: %s | До: %s",
		ct, body, p.ID[:8], p.ExpiresAt[11:16])
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

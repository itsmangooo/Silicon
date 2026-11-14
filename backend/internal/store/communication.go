package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AccountMessage struct {
	ID          uuid.UUID  `json:"id"`
	SenderID    uuid.UUID  `json:"senderId"`
	RecipientID uuid.UUID  `json:"recipientId"`
	ParentID    *uuid.UUID `json:"parentId,omitempty"`
	Sender      string     `json:"sender"`
	Recipient   string     `json:"recipient"`
	Subject     string     `json:"subject"`
	Body        string     `json:"body"`
	CreatedAt   time.Time  `json:"createdAt"`
	ReadAt      *time.Time `json:"readAt,omitempty"`
}

func (r Repository) Messages(ctx context.Context, org, user uuid.UUID, sent bool, search string, offset int) ([]AccountMessage, error) {
	rows, err := r.Pool.Query(ctx, `SELECT m.id,m.sender_id,m.recipient_id,m.parent_id,s.display_name,d.display_name,m.subject,m.body,m.created_at,m.read_at
		FROM account_messages m JOIN users s ON s.id=m.sender_id JOIN users d ON d.id=m.recipient_id
		WHERE m.organization_id=$1 AND (CASE WHEN $3 THEN m.sender_id ELSE m.recipient_id END)=$2
		AND ($4='' OR strpos(lower(m.subject||' '||m.body||' '||s.display_name||' '||d.display_name),lower($4))>0)
		ORDER BY m.created_at DESC,m.id DESC LIMIT 51 OFFSET $5`, org, user, sent, search, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[AccountMessage])
}

func (r Repository) SendMessage(ctx context.Context, org, sender, recipient uuid.UUID, parent *uuid.UUID, subject, body string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.Pool.QueryRow(ctx, `WITH sent AS (
		INSERT INTO account_messages(organization_id,sender_id,recipient_id,parent_id,subject,body)
		SELECT $1,$2,$3,$4,$5,$6 FROM memberships m JOIN users u ON u.id=m.user_id
		WHERE m.organization_id=$1 AND m.user_id=$3 AND u.status='active'
		AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM account_messages p WHERE p.id=$4 AND p.organization_id=$1 AND
		((p.sender_id=$2 AND p.recipient_id=$3) OR (p.sender_id=$3 AND p.recipient_id=$2))))
		RETURNING id)
		INSERT INTO account_notifications(user_id,organization_id,kind,resource_id) SELECT $3,$1,'message',id FROM sent RETURNING resource_id`,
		org, sender, recipient, parent, subject, body).Scan(&id)
	return id, notFound(err)
}

func (r Repository) ReadMessage(ctx context.Context, org, user, id uuid.UUID) (AccountMessage, error) {
	var message AccountMessage
	err := r.Pool.QueryRow(ctx, `WITH marked AS (
		UPDATE account_messages SET read_at=COALESCE(read_at,now()) WHERE organization_id=$1 AND recipient_id=$2 AND id=$3 RETURNING read_at),
		notifications AS (UPDATE account_notifications SET read_at=COALESCE(read_at,now()) WHERE organization_id=$1 AND user_id=$2 AND kind='message' AND resource_id=$3)
		SELECT m.id,m.sender_id,m.recipient_id,m.parent_id,s.display_name,d.display_name,m.subject,m.body,m.created_at,COALESCE((SELECT read_at FROM marked),m.read_at)
		FROM account_messages m JOIN users s ON s.id=m.sender_id JOIN users d ON d.id=m.recipient_id
		WHERE m.organization_id=$1 AND m.id=$3 AND $2 IN (m.sender_id,m.recipient_id)`, org, user, id).
		Scan(&message.ID, &message.SenderID, &message.RecipientID, &message.ParentID, &message.Sender, &message.Recipient, &message.Subject, &message.Body, &message.CreatedAt, &message.ReadAt)
	return message, notFound(err)
}

type SupportTicket struct {
	ID          uuid.UUID  `json:"id"`
	RequesterID uuid.UUID  `json:"requesterId"`
	AssigneeID  *uuid.UUID `json:"assigneeId,omitempty"`
	Subject     string     `json:"subject"`
	Category    string     `json:"category"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type SupportEvent struct {
	ID        int64     `json:"id"`
	Actor     string    `json:"actor"`
	Kind      string    `json:"kind"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

const ticketFields = `id,requester_id,assignee_id,subject,category,status,priority,created_at,updated_at`

func (r Repository) Tickets(ctx context.Context, org, user uuid.UUID, staff bool, search, status string, offset int) ([]SupportTicket, error) {
	rows, err := r.Pool.Query(ctx, `SELECT `+ticketFields+` FROM support_tickets
		WHERE organization_id=$1 AND (requester_id=$2 OR $3) AND ($4='' OR strpos(lower(subject),lower($4))>0)
		AND ($5='' OR status=$5) ORDER BY updated_at DESC,id DESC LIMIT 51 OFFSET $6`, org, user, staff, search, status, offset)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[SupportTicket])
}

func (r Repository) CreateTicket(ctx context.Context, org, user uuid.UUID, subject, category, body string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.Pool.QueryRow(ctx, `WITH created AS (
		INSERT INTO support_tickets(organization_id,requester_id,subject,category) VALUES($1,$2,$3,$4) RETURNING id),
		first_event AS (INSERT INTO support_ticket_events(ticket_id,actor_id,kind,body) SELECT id,$2,'reply',$5 FROM created)
		SELECT id FROM created`, org, user, subject, category, body).Scan(&id)
	return id, err
}

func (r Repository) Ticket(ctx context.Context, org, user, id uuid.UUID, staff bool, offset int) (SupportTicket, []SupportEvent, error) {
	rows, err := r.Pool.Query(ctx, `SELECT `+ticketFields+` FROM support_tickets WHERE organization_id=$1 AND id=$3 AND (requester_id=$2 OR $4)`, org, user, id, staff)
	if err != nil {
		return SupportTicket{}, nil, err
	}
	ticket, err := pgx.CollectOneRow(rows, pgx.RowToStructByPos[SupportTicket])
	if err != nil {
		return ticket, nil, notFound(err)
	}
	rows, err = r.Pool.Query(ctx, `SELECT e.id,u.display_name,e.kind,e.body,e.created_at FROM support_ticket_events e JOIN users u ON u.id=e.actor_id
		WHERE e.ticket_id=$1 ORDER BY e.id DESC LIMIT 51 OFFSET $2`, id, offset)
	if err != nil {
		return ticket, nil, err
	}
	events, err := pgx.CollectRows(rows, pgx.RowToStructByPos[SupportEvent])
	return ticket, events, err
}

func (r Repository) UpdateTicket(ctx context.Context, org, user, id uuid.UUID, staff bool, kind, body string, assign *uuid.UUID) error {
	var ticket uuid.UUID
	err := r.Pool.QueryRow(ctx, `WITH changed AS (
		UPDATE support_tickets t SET updated_at=now(),
		status=CASE WHEN $5='status' THEN $6 ELSE status END,
		priority=CASE WHEN $5='priority' THEN $6 ELSE priority END,
		assignee_id=CASE WHEN $5='assignment' THEN $7 ELSE assignee_id END
		WHERE organization_id=$1 AND id=$3 AND (requester_id=$2 OR $4)
		AND ($4 OR (($5='reply' OR ($5='status' AND $6='closed')) AND status<>'closed'))
		AND ($7::uuid IS NULL OR EXISTS (SELECT 1 FROM memberships m JOIN users u ON u.id=m.user_id
		WHERE m.organization_id=$1 AND m.user_id=$7 AND u.is_system_admin AND u.status='active'))
		RETURNING id,requester_id,assignee_id),
		event AS (INSERT INTO support_ticket_events(ticket_id,actor_id,kind,body) SELECT id,$2,$5,$6 FROM changed),
		notice AS (INSERT INTO account_notifications(user_id,organization_id,kind,resource_id)
		SELECT CASE WHEN requester_id=$2 THEN assignee_id ELSE requester_id END,$1,'support_'||$5,id FROM changed
		WHERE (requester_id<>$2 OR assignee_id IS NOT NULL) AND CASE WHEN requester_id=$2 THEN assignee_id ELSE requester_id END<>$2
		UNION ALL SELECT assignee_id,$1,'support_assignment',id FROM changed
		WHERE $5='assignment' AND assignee_id IS NOT NULL AND assignee_id<>$2 AND assignee_id<>requester_id)
		SELECT id FROM changed`, org, user, id, staff, kind, body, assign).Scan(&ticket)
	return notFound(err)
}

type AccountNotification struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organizationId,omitempty"`
	Kind           string     `json:"kind"`
	ResourceID     *uuid.UUID `json:"resourceId,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	ReadAt         *time.Time `json:"readAt,omitempty"`
}

func (r Repository) Notifications(ctx context.Context, user uuid.UUID) ([]AccountNotification, int, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id,organization_id,kind,resource_id,created_at,read_at FROM account_notifications
		WHERE user_id=$1 AND (organization_id IS NULL OR EXISTS (SELECT 1 FROM memberships m WHERE m.user_id=$1 AND m.organization_id=account_notifications.organization_id))
		ORDER BY created_at DESC,id DESC LIMIT 50`, user)
	if err != nil {
		return nil, 0, err
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[AccountNotification])
	if err != nil {
		return nil, 0, err
	}
	var unread int
	err = r.Pool.QueryRow(ctx, `SELECT count(*) FROM account_notifications WHERE user_id=$1 AND read_at IS NULL
		AND (organization_id IS NULL OR EXISTS (SELECT 1 FROM memberships m WHERE m.user_id=$1 AND m.organization_id=account_notifications.organization_id))`, user).Scan(&unread)
	return items, unread, err
}

func (r Repository) ReadNotification(ctx context.Context, user, id uuid.UUID) error {
	result, err := r.Pool.Exec(ctx, `UPDATE account_notifications SET read_at=COALESCE(read_at,now()) WHERE user_id=$1 AND id=$2`, user, id)
	if err == nil && result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

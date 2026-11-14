CREATE TABLE account_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    sender_id uuid NOT NULL REFERENCES users(id),
    recipient_id uuid NOT NULL REFERENCES users(id),
    parent_id uuid REFERENCES account_messages(id) ON DELETE SET NULL,
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 160),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 8000),
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at timestamptz,
    CHECK (sender_id <> recipient_id)
);
CREATE INDEX account_messages_inbox ON account_messages(organization_id,recipient_id,created_at DESC);
CREATE INDEX account_messages_sent ON account_messages(organization_id,sender_id,created_at DESC);

CREATE TABLE support_tickets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    requester_id uuid NOT NULL REFERENCES users(id),
    assignee_id uuid REFERENCES users(id),
    subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 160),
    category text NOT NULL CHECK (category IN ('general','account','deployment','server','bug','feature')),
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','waiting_for_user','resolved','closed')),
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low','normal','high','urgent')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX support_tickets_requester ON support_tickets(organization_id,requester_id,updated_at DESC);
CREATE INDEX support_tickets_org ON support_tickets(organization_id,updated_at DESC);

CREATE TABLE support_ticket_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id uuid NOT NULL REFERENCES support_tickets(id) ON DELETE CASCADE,
    actor_id uuid NOT NULL REFERENCES users(id),
    kind text NOT NULL CHECK (kind IN ('reply','status','priority','assignment')),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 8000),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX support_ticket_events_history ON support_ticket_events(ticket_id,id);

CREATE TABLE account_notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
    kind text NOT NULL,
    resource_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    read_at timestamptz
);
CREATE INDEX account_notifications_inbox ON account_notifications(user_id,created_at DESC);

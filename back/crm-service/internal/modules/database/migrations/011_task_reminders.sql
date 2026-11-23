ALTER TABLE tasks
    ADD COLUMN reminder_minutes integer CHECK (reminder_minutes BETWEEN 0 AND 43200),
    ADD COLUMN reminder_at timestamp without time zone,
    ADD COLUMN reminder_token uuid,
    ADD COLUMN reminder_dismissed boolean NOT NULL DEFAULT false;

-- The UTC instant and token identify the current open occurrence. Unrelated
-- task edits preserve a dismissal; a new occurrence must be dismissed anew.
CREATE FUNCTION refresh_task_reminder() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' OR
       (NEW.due_at, NEW.owner_id, NEW.status, NEW.reminder_minutes) IS DISTINCT FROM
       (OLD.due_at, OLD.owner_id, OLD.status, OLD.reminder_minutes) THEN
        NEW.reminder_dismissed := false;
        IF NEW.status = 'open' AND NEW.reminder_minutes IS NOT NULL THEN
            NEW.reminder_at := (NEW.due_at AT TIME ZONE 'UTC') - NEW.reminder_minutes * interval '1 minute';
            NEW.reminder_token := gen_random_uuid();
        ELSE
            NEW.reminder_at := NULL;
            NEW.reminder_token := NULL;
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_refresh_reminder
BEFORE INSERT OR UPDATE OF due_at, owner_id, status, reminder_minutes ON tasks
FOR EACH ROW EXECUTE FUNCTION refresh_task_reminder();

CREATE INDEX tasks_personal_reminders ON tasks (workspace_id, owner_id, reminder_at DESC, id)
    WHERE reminder_at IS NOT NULL AND NOT reminder_dismissed;

ALTER TABLE tasks ADD COLUMN completed_at timestamptz;

-- Previously completed tasks have no status-transition history. Their last
-- saved timestamp is the best available estimate for this one-time backfill.
UPDATE tasks SET completed_at = updated_at WHERE status = 'done';
ALTER TABLE tasks ADD CONSTRAINT tasks_completion_status
    CHECK ((status = 'done') = (completed_at IS NOT NULL));

CREATE FUNCTION record_task_completion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status = 'open' THEN
        NEW.completed_at := NULL;
    ELSIF TG_OP = 'INSERT' THEN
        NEW.completed_at := NEW.created_at;
    ELSIF OLD.status <> NEW.status THEN
        NEW.completed_at := clock_timestamp();
    ELSE
        NEW.completed_at := OLD.completed_at;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER tasks_record_completion
BEFORE INSERT OR UPDATE OF status, completed_at ON tasks
FOR EACH ROW EXECUTE FUNCTION record_task_completion();

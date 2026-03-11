-- pg_notify trigger for notification SSE stream.
-- Run this on the forum Postgres database.
-- It fires a NOTIFY on the 'notification_changes' channel whenever
-- a new row is inserted into the 'notifications' table.

CREATE OR REPLACE FUNCTION notify_notification_insert()
RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('notification_changes', json_build_object(
    'id', NEW.id,
    'user_id', NEW.user_id,
    'type', NEW.type,
    'title', NEW.title,
    'message', NEW.message,
    'link', NEW.link,
    'read', NEW.read,
    'created_at', NEW.created_at
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_notification_insert ON notifications;
CREATE TRIGGER trg_notification_insert
  AFTER INSERT ON notifications
  FOR EACH ROW
  EXECUTE FUNCTION notify_notification_insert();

-- pg_notify trigger for chat message SSE stream.
CREATE OR REPLACE FUNCTION notify_chat_message_insert()
RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('chat_message_changes', json_build_object(
    'id', NEW.id,
    'channel_id', NEW.channel_id,
    'author_id', NEW.author_id,
    'content', NEW.content,
    'created_at', NEW.created_at
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_chat_message_insert ON chat_messages;
CREATE TRIGGER trg_chat_message_insert
  AFTER INSERT ON chat_messages
  FOR EACH ROW
  EXECUTE FUNCTION notify_chat_message_insert();

-- pg_notify trigger for voice presence changes.
CREATE OR REPLACE FUNCTION notify_voice_presence_change()
RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    PERFORM pg_notify('voice_presence_changes', json_build_object(
      'event', 'DELETE',
      'user_id', OLD.user_id,
      'room_slug', OLD.room_slug
    )::text);
    RETURN OLD;
  ELSE
    PERFORM pg_notify('voice_presence_changes', json_build_object(
      'event', TG_OP,
      'user_id', NEW.user_id,
      'room_slug', NEW.room_slug,
      'joined_at', NEW.joined_at
    )::text);
    RETURN NEW;
  END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_voice_presence_change ON voice_presence;
CREATE TRIGGER trg_voice_presence_change
  AFTER INSERT OR UPDATE OR DELETE ON voice_presence
  FOR EACH ROW
  EXECUTE FUNCTION notify_voice_presence_change();

-- pg_notify trigger for post inserts (thread realtime updates).
CREATE OR REPLACE FUNCTION notify_post_insert()
RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('post_changes', json_build_object(
    'id', NEW.id,
    'thread_id', NEW.thread_id,
    'author_id', NEW.author_id,
    'content', NEW.content,
    'reply_to_id', NEW.reply_to_id,
    'created_at', NEW.created_at
  )::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_post_insert ON posts;
CREATE TRIGGER trg_post_insert
  AFTER INSERT ON posts
  FOR EACH ROW
  EXECUTE FUNCTION notify_post_insert();

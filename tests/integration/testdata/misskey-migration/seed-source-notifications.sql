SET session_replication_role = replica;

DELETE FROM notification
WHERE id IN ('9z00000001', '9z00000002', '9z00000003');

INSERT INTO notification (
  id,
  "createdAt",
  "notifieeId",
  "notifierId",
  type,
  "isRead",
  "noteId",
  reaction,
  choice,
  "customBody",
  "customHeader",
  "customIcon",
  "appAccessTokenId"
) VALUES
  (
    '9z00000001',
    '2026-07-01T00:00:01Z',
    '9yuser0001',
    '9yactor001',
    'follow',
    false,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL
  ),
  (
    '9z00000002',
    '2026-07-01T00:00:02Z',
    '9yuser0001',
    '9yactor002',
    'reaction',
    true,
    '9ynote0001',
    ':miwkey:',
    NULL,
    NULL,
    NULL,
    NULL,
    NULL
  ),
  (
    '9z00000003',
    '2026-07-01T00:00:03Z',
    '9yuser0001',
    NULL,
    'app',
    false,
    NULL,
    NULL,
    NULL,
    'migration app notification',
    'Migration',
    'https://example.com/icon.png',
    NULL
  );

SET session_replication_role = DEFAULT;

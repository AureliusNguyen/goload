# Transactional outbox for task events (reliable dispatch, step 1).
# CreateTask and RetryTask insert a row here in the same transaction as the
# task row. The outbox relay publishes rows whose sent_at is NULL and then
# sets sent_at.
#
# id            stable unique event id (UUID), reused as the message UUID
# aggregate_id  the id of the task row
# destination   dispatch or lifecycle (recorded for later routing)
# event_type    for example TaskCreated
# payload       event JSON bytes, published verbatim
# sent_at       NULL means not published yet
#
# Idempotent on purpose: run-migrations.sh applies every file on each start.
CREATE TABLE
    IF NOT EXISTS task_outbox (
        id CHAR(36) NOT NULL,
        aggregate_id BIGINT UNSIGNED NOT NULL,
        destination VARCHAR(32) NOT NULL,
        event_type VARCHAR(64) NOT NULL,
        payload MEDIUMBLOB NOT NULL,
        created_at DATETIME(6) NOT NULL,
        sent_at DATETIME(6) NULL DEFAULT NULL,
        PRIMARY KEY (id),
        INDEX task_outbox_unsent_idx (sent_at, created_at),
        INDEX task_outbox_aggregate_idx (aggregate_id)
    );

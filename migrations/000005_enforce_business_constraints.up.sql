DROP INDEX IF EXISTS idx_payments_task_id;
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_task_id ON payments(task_id);

CREATE UNIQUE INDEX IF NOT EXISTS uq_bids_task_one_accepted
    ON bids(task_id)
    WHERE status = 'accepted';

-- Check each index so an interrupted migration can be retried and a fresh
-- database can use the same command. Run with the application database selected.
SET @ddl = IF(
    EXISTS (
        SELECT 1 FROM information_schema.statistics
        WHERE table_schema = DATABASE() AND table_name = 'tbl_user_file' AND index_name = 'idx_user_file_list'
    ),
    'SELECT 1 WHERE FALSE',
    'ALTER TABLE tbl_user_file ADD INDEX idx_user_file_list (user_id, status, upload_at, id), ALGORITHM=INPLACE'
);
PREPARE migrate_index FROM @ddl;
EXECUTE migrate_index;
DEALLOCATE PREPARE migrate_index;

-- The new index starts with user_id and can enforce the owner foreign key.
SET @ddl = IF(
    EXISTS (
        SELECT 1 FROM information_schema.statistics
        WHERE table_schema = DATABASE() AND table_name = 'tbl_user_file' AND index_name = 'idx_user_id'
    ),
    'ALTER TABLE tbl_user_file DROP INDEX idx_user_id, ALGORITHM=INPLACE',
    'SELECT 1 WHERE FALSE'
);
PREPARE migrate_index FROM @ddl;
EXECUTE migrate_index;
DEALLOCATE PREPARE migrate_index;

SET @ddl = IF(
    EXISTS (
        SELECT 1 FROM information_schema.statistics
        WHERE table_schema = DATABASE() AND table_name = 'tbl_user_file' AND index_name = 'idx_status'
    ),
    'ALTER TABLE tbl_user_file DROP INDEX idx_status, ALGORITHM=INPLACE',
    'SELECT 1 WHERE FALSE'
);
PREPARE migrate_index FROM @ddl;
EXECUTE migrate_index;
DEALLOCATE PREPARE migrate_index;

SET @ddl = IF(
    EXISTS (
        SELECT 1 FROM information_schema.statistics
        WHERE table_schema = DATABASE() AND table_name = 'tbl_user' AND index_name = 'idx_status'
    ),
    'ALTER TABLE tbl_user DROP INDEX idx_status, ALGORITHM=INPLACE',
    'SELECT 1 WHERE FALSE'
);
PREPARE migrate_index FROM @ddl;
EXECUTE migrate_index;
DEALLOCATE PREPARE migrate_index;

SET @ddl = IF(
    EXISTS (
        SELECT 1 FROM information_schema.statistics
        WHERE table_schema = DATABASE() AND table_name = 'tbl_file' AND index_name = 'idx_status'
    ),
    'ALTER TABLE tbl_file DROP INDEX idx_status, ALGORITHM=INPLACE',
    'SELECT 1 WHERE FALSE'
);
PREPARE migrate_index FROM @ddl;
EXECUTE migrate_index;
DEALLOCATE PREPARE migrate_index;

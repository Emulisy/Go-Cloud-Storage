CREATE TABLE IF NOT EXISTS tbl_file (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT 'Internal file metadata identifier',

    file_sha CHAR(64) NOT NULL
        COMMENT 'File SHA-256 hash as 64 hexadecimal characters',
    file_size BIGINT NOT NULL DEFAULT 0
        COMMENT 'File size in bytes',
    file_addr VARCHAR(1024) NOT NULL
        COMMENT 'Server-side storage location',

    status TINYINT NOT NULL DEFAULT 0
        COMMENT 'File status: 0=inactive, 1=active',

    PRIMARY KEY (id),
    UNIQUE KEY idx_file_sha (file_sha),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='Shared stored file content';


CREATE TABLE IF NOT EXISTS tbl_user (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT 'Internal user identifier',

    user_name VARCHAR(64) NOT NULL
        COMMENT 'Non-unique display name',
    user_pwd VARCHAR(255) NOT NULL
        COMMENT 'Bcrypt password hash',

    email VARCHAR(254) NOT NULL
        COMMENT 'Unique normalized sign-in email address',

    email_validated BOOLEAN NOT NULL DEFAULT FALSE
        COMMENT 'Whether the email address has been verified',

    signup_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        COMMENT 'Time the account was created',
    last_active DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        COMMENT 'Time of the user most recent activity',

    profile TEXT
        COMMENT 'Optional user profile data',
    status TINYINT NOT NULL DEFAULT 0
        COMMENT 'User status: 0=active, 1=disabled',

    PRIMARY KEY (id),
    UNIQUE KEY idx_email (email),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='Application user accounts';

CREATE TABLE IF NOT EXISTS `tbl_user_file` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT 'Internal user-file relationship identifier',

    `user_id` BIGINT UNSIGNED NOT NULL
        COMMENT 'User ID that owns the file relationship',

    `file_sha256` CHAR(64) NOT NULL
        COMMENT 'File SHA-256 hash as 64 hexadecimal characters',

    `file_size` BIGINT NOT NULL DEFAULT 0
        COMMENT 'File size in bytes',

    `file_name` VARCHAR(255) NOT NULL
        COMMENT 'User-specific display and download name',

    `upload_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        COMMENT 'File upload time',

    `last_update` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP
        COMMENT 'Last modification time',

    `status` INT NOT NULL DEFAULT 0
        COMMENT 'File status: 0=active, 1=deleted, 2=disabled',

    PRIMARY KEY (`id`),

    KEY `idx_user_id` (`user_id`),

    KEY `idx_file_sha256` (`file_sha256`),

    KEY `idx_status` (`status`),

    CONSTRAINT `fk_user_file_owner`
        FOREIGN KEY (`user_id`)
        REFERENCES `tbl_user` (`id`)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT `fk_user_file_content`
        FOREIGN KEY (`file_sha256`)
        REFERENCES `tbl_file` (`file_sha`)
        ON UPDATE RESTRICT
        ON DELETE RESTRICT

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='Per-user file names and upload metadata';

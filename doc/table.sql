CREATE TABLE IF NOT EXISTS tbl_file (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT 'Internal file metadata identifier',

    file_sha CHAR(64) NOT NULL
        COMMENT 'File SHA-256 hash as 64 hexadecimal characters',
    file_name VARCHAR(255) NOT NULL
        COMMENT 'Original client-provided file name',
    file_size BIGINT NOT NULL DEFAULT 0
        COMMENT 'File size in bytes',
    file_addr VARCHAR(1024) NOT NULL
        COMMENT 'Server-side storage location',

    create_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        COMMENT 'Time the file metadata was created',
    update_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6)
        COMMENT 'Time the file metadata was last updated',

    status TINYINT NOT NULL DEFAULT 0
        COMMENT 'File status: 0=inactive, 1=active',

    PRIMARY KEY (id),
    UNIQUE KEY idx_file_sha (file_sha),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='Stored file metadata';


CREATE TABLE IF NOT EXISTS tbl_user (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT 'Internal user identifier',

    user_name VARCHAR(64) NOT NULL
        COMMENT 'Unique sign-in name',
    user_pwd VARCHAR(255) NOT NULL
        COMMENT 'Bcrypt password hash',

    email VARCHAR(254) DEFAULT NULL
        COMMENT 'Optional user email address',
    phone VARCHAR(32) DEFAULT NULL
        COMMENT 'Optional user phone number',

    email_validated BOOLEAN NOT NULL DEFAULT FALSE
        COMMENT 'Whether the email address has been verified',
    phone_validated BOOLEAN NOT NULL DEFAULT FALSE
        COMMENT 'Whether the phone number has been verified',

    signup_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        COMMENT 'Time the account was created',
    last_active DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        COMMENT 'Time of the user most recent activity',

    profile TEXT
        COMMENT 'Optional user profile data',
    status TINYINT NOT NULL DEFAULT 0
        COMMENT 'User status: 0=active, 1=disabled',

    PRIMARY KEY (id),
    UNIQUE KEY idx_user_name (user_name),
    UNIQUE KEY idx_email (email),
    UNIQUE KEY idx_phone (phone),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='Application user accounts';

CREATE TABLE IF NOT EXISTS `tbl_user_file` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT
        COMMENT 'Internal user-file relationship identifier',

    `user_id` BIGINT UNSIGNED NOT NULL
        COMMENT 'User who owns the file relationship',

    `file_sha256` CHAR(64) NOT NULL
        COMMENT 'File SHA-256 hash as 64 hexadecimal characters',

    `file_size` BIGINT NOT NULL DEFAULT 0
        COMMENT 'File size in bytes',

    `file_name` VARCHAR(255) NOT NULL
        COMMENT 'Original file name',

    `upload_at` DATETIME DEFAULT CURRENT_TIMESTAMP
        COMMENT 'File upload time',

    `last_update` DATETIME DEFAULT CURRENT_TIMESTAMP
        ON UPDATE CURRENT_TIMESTAMP
        COMMENT 'Last modification time',

    `status` INT NOT NULL DEFAULT 0
        COMMENT 'File status: 0=active, 1=deleted, 2=disabled',

    PRIMARY KEY (`id`),

    KEY `idx_user_id` (`user_id`),

    KEY `idx_file_sha256` (`file_sha256`),

    KEY `idx_status` (`status`),

    CONSTRAINT `fk_file_user`
        FOREIGN KEY (`user_id`)
        REFERENCES `tbl_user` (`id`)

) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
  COMMENT='Per-user file names and upload metadata';

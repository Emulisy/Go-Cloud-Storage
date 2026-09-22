CREATE TABLE IF NOT EXISTS tbl_file (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    file_sha CHAR(64) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_size BIGINT NOT NULL DEFAULT 0,
    file_addr VARCHAR(1024) NOT NULL,

    create_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    update_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),

    status TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (id),
    UNIQUE KEY idx_file_sha (file_sha),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


CREATE TABLE IF NOT EXISTS tbl_user (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    user_name VARCHAR(64) NOT NULL,
    user_pwd VARCHAR(255) NOT NULL,

    email VARCHAR(254) DEFAULT NULL,
    phone VARCHAR(32) DEFAULT NULL,

    email_validated BOOLEAN NOT NULL DEFAULT FALSE,
    phone_validated BOOLEAN NOT NULL DEFAULT FALSE,

    signup_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    last_active DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

    profile TEXT,
    status TINYINT NOT NULL DEFAULT 0,

    PRIMARY KEY (id),
    UNIQUE KEY idx_user_name (user_name),
    UNIQUE KEY idx_email (email),
    UNIQUE KEY idx_phone (phone),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
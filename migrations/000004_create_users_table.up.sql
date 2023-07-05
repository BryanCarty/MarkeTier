CREATE TABLE IF NOT EXISTS base_users (
    user_id bigserial PRIMARY KEY,
    first_name varchar(255) NOT NULL
    last_name varchar(255) NOT NULL
    email varchar(100) UNIQUE NOT NULL,
    date_of_birth timestamp NOT NULL,
    gender varchar(5) NOT NULL,
    address text NOT NULL,
    password bytea NOT NULL,
    account_creation_time timestamp with time zone NOT NULL DEFAULT NOW(),
    last_login_time timestamp,
    account_status varchar(20) NOT NULL DEFAULT 'REGISTERING',
    version integer NOT NULL DEFAULT 1,
    account_type smallint NOT NULL

);
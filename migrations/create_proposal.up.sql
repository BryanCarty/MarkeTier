CREATE TABLE IF NOT EXISTS proposals (
    proposal_id bigserial PRIMARY KEY,
    title text NOT NULL,
    about text NOT NULL,
    version int NOT NULL DEFAULT 1
);
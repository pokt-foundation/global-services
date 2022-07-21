-- Tables
CREATE TABLE IF NOT EXISTS cherry_picker_session (
  public_key CHAR(64),
  chain CHAR(4),
  session_key CHAR(44),
  session_height INT,
  address CHAR(40),
  total_success INT,
  total_failure INT,
  avg_success_time REAL,
  failure BOOLEAN,
  application_public_key CHAR(64),
  PRIMARY KEY(public_key, chain, session_key)
);
CREATE TABLE IF NOT EXISTS cherry_picker_session_region (
  public_key CHAR(64),
  chain CHAR(4),
  session_key CHAR(44),
  session_height INT,
  region VARCHAR(20),
  address CHAR(40),
  total_success INT,
  total_failure INT,
  median_success_latency REAL [ ],
  weighted_success_latency REAL [ ],
  avg_success_latency REAL,
  avg_weighted_success_latency REAL,
  p_90_latency REAL [ ],
  attempts INT [ ],
  success_rate REAL [ ],
  failure BOOLEAN,
  application_public_key CHAR(64),
  PRIMARY KEY(public_key, chain, session_key, region),
  FOREIGN KEY(public_key, chain, session_key) REFERENCES cherry_picker_session(public_key, chain, session_key)
);
-- Indexes 
CREATE INDEX IF NOT EXISTS public_key_idx ON cherry_picker_session (public_key);
CREATE INDEX IF NOT EXISTS chain_idx ON cherry_picker_session (chain);
CREATE INDEX IF NOT EXISTS session_height_idx ON cherry_picker_session (session_height);
CREATE INDEX IF NOT EXISTS session_key_idx ON cherry_picker_session (session_key);
CREATE INDEX IF NOT EXISTS app_public_key_idx on cherry_picker_session (application_public_key);
CREATE INDEX IF NOT EXISTS public_key_idx ON cherry_picker_session_region (public_key);
CREATE INDEX IF NOT EXISTS chain_idx ON cherry_picker_session_region (chain);
CREATE INDEX IF NOT EXISTS session_height_idx ON cherry_picker_session_region (session_height);
CREATE INDEX IF NOT EXISTS session_key_idx ON cherry_picker_session_region (session_key);
CREATE INDEX IF NOT EXISTS app_public_key_idx on cherry_picker_session_region (application_public_key);
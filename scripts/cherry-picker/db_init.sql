-- Tables
CREATE TABLE IF NOT EXISTS cherry_picker_session (
  public_key CHAR(64),
  chain CHAR(4),
  session_key CHAR(44),
  session_height INT,
  address CHAR(40),
  aggregate_success INT,
  aggregate_failures INT,
  avg_success_time REAL,
  failure BOOLEAN,
  PRIMARY KEY(public_key, chain, session_key)
);
CREATE TABLE IF NOT EXISTS cherry_picker_session_region (
  public_key CHAR(64),
  chain CHAR(4),
  session_key CHAR(44),
  region VARCHAR(20),
  median_success_latency REAL [ ],
  weighted_success_latency REAL [ ],
  avg_success_latency REAL,
  avg_weighted_success_latency REAL,
  failure BOOLEAN,
  PRIMARY KEY(public_key, chain, session_key, region),
  FOREIGN KEY(public_key, chain, session_key) REFERENCES cherry_picker_session(public_key, chain, session_key)
);
-- Indexes 
CREATE INDEX IF NOT EXISTS public_key_idx ON cherry_picker_session (public_key);
CREATE INDEX IF NOT EXISTS chain_dix ON cherry_picker_session (chain);
CREATE INDEX IF NOT EXISTS session_height_idx ON cherry_picker_session (session_height);
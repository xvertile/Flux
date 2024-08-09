-- init.sql

-- Table for Providers
CREATE TABLE IF NOT EXISTS providers
(
    ID UUID DEFAULT generateUUIDv4(),
    Name String,
    Domain String
) ENGINE = MergeTree()
      ORDER BY ID
      SETTINGS index_granularity = 8192;

-- Table for Proxy Requests
CREATE TABLE IF NOT EXISTS proxy_requests
(
    ProviderName String,
    IP String,
    TimeTaken Float32,
    StatusCode UInt16,
    RequestTime DateTime,
    EventTime DateTime DEFAULT now(),
    ContinentCode String,
    ContinentName String,
    CountryISOCode String,
    CountryName String,
    CityName String,
    Success UInt8,
    Type String,
    Pool String,
    ErrorMessage String,
    Latitude Float32,
    Longitude Float32,
    AccuracyRadius UInt16,
    TimeZone String,
    PostalCode String
) ENGINE = MergeTree()
      PARTITION BY toYYYYMM(RequestTime)
      ORDER BY (ProviderName, RequestTime, IP, Pool, Success, Type, CountryName, CountryISOCode)
      SETTINGS index_granularity = 8192;

-- Table for Jobs
CREATE TABLE IF NOT EXISTS jobs
(
    JobName String,
    ProviderName String,
    Proxy String,
    Pool String,
    URL String,
    Type String,
    Status String,
    Threads UInt16,
    StartTime DateTime,
    EndTime DateTime
) ENGINE = MergeTree()
      ORDER BY JobName
      SETTINGS index_granularity = 8192;

-- Table for Job Heartbeats
CREATE TABLE IF NOT EXISTS job_heartbeats
(
    HeartbeatID UUID DEFAULT generateUUIDv4(),
    JobName String,
    Status String,
    Timestamp DateTime DEFAULT now(),
    Message String
) ENGINE = MergeTree()
      ORDER BY (JobName, Timestamp)
      SETTINGS index_granularity = 8192;

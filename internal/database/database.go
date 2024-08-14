package database

import (
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"log"
	"sync"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

var (
	db   *sql.DB
	once sync.Once
)

func InitDatabase(dsn string) {
	once.Do(func() {
		var err error
		db, err = sql.Open("clickhouse", dsn)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}

		if err = db.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}

		log.Println("Connected to ClickHouse database.")
	})
	createTables()
}

type RequestData struct {
	ProviderID    string
	IP            string
	TimeTaken     float32
	StatusCode    uint16
	RequestTime   string
	ContinentCode string
	ContinentName string
	CountryISO    string
	CountryName   string
	CityName      string
	Latitude      float32
	Longitude     float32
	Accuracy      uint16
	TimeZone      string
	PostalCode    string
	ErrorMessage  string
	Success       uint8
	ProxyType     string
	Pool          string
}

func InsertRequests(requests []RequestData) error {
	tx, err := db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction: %v", err)
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO proxy_requests (ProviderName, IP, TimeTaken, StatusCode, RequestTime, ContinentCode, ContinentName, CountryISOCode, CountryName, CityName, Latitude, Longitude, AccuracyRadius, TimeZone, PostalCode, ErrorMessage, Success, Type, Pool) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		return err
	}
	defer stmt.Close()

	for _, req := range requests {
		_, err := stmt.Exec(req.ProviderID, req.IP, req.TimeTaken, req.StatusCode, req.RequestTime, req.ContinentCode, req.ContinentName, req.CountryISO, req.CountryName, req.CityName, req.Latitude, req.Longitude, req.Accuracy, req.TimeZone, req.PostalCode, req.ErrorMessage, req.Success, req.ProxyType, req.Pool)
		if err != nil {
			log.Printf("Failed to execute statement: %v", err)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func GetProviderByName(providerName string) (string, error) {
	var providerID string
	err := db.QueryRow(`SELECT Name FROM providers WHERE Name = ?`, providerName).Scan(&providerID)
	if err != nil {
		log.Printf("Failed to get provider ID: %v", err)
		return "", err
	}
	return providerID, nil
}

func GetJobByName(name string) (Job, error) {
	var job Job
	err := db.QueryRow(`SELECT JobName, ProviderName, Proxy, Pool, Type, Status, Threads FROM jobs WHERE JobName = ?`, name).Scan(&job.JobName, &job.ProviderName, &job.Proxy, &job.Pool, &job.Type, &job.Status, &job.Threads)
	if err != nil {
		log.Printf("Failed to get job: %v", err)
		return job, err
	}
	return job, nil
}

func GetLastHeartbeat(name string) (string, error) {
	var status string
	err := db.QueryRow(`SELECT Timestamp FROM job_heartbeats WHERE JobName = ? ORDER BY Timestamp DESC LIMIT 1`, name).Scan(&status)
	if err != nil {
		log.Printf("Failed to get last heartbeat: %v", err)
		return "", err
	}
	return status, nil

}
func GetJobStatusByName(name string) (string, error) {
	var status string
	err := db.QueryRow(`SELECT Status FROM jobs WHERE JobName = ?`, name).Scan(&status)
	if err != nil {
		log.Printf("Failed to get job status: %v", err)
		return "", err
	}
	return status, nil
}

func UpdateJobStatus(name, status string) error {
	query := fmt.Sprintf("ALTER TABLE jobs UPDATE Status = '%s' WHERE JobName = '%s'", status, name)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to update job status: %v", err)
		return err
	}
	return nil
}

func InsertJobHeartbeat(jobName, status, message string) error {
	heartbeatID := uuid.New().String()
	query := fmt.Sprintf("INSERT INTO job_heartbeats (HeartbeatID, JobName, Status, Timestamp, Message) VALUES ('%s', '%s', '%s', now(), '%s')", heartbeatID, jobName, status, message)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to insert job heartbeat: %v", err)
		return err
	}
	return nil
}

func GetJobStatus(jobname string) (string, error) {

	var status string
	err := db.QueryRow(`SELECT Status FROM jobs WHERE JobName = ?`, jobname).Scan(&status)
	if err != nil {
		log.Printf("Failed to get job status: %v", err)
		return "", err
	}
	return status, nil
}

func DeleteJob(jobName string) error {
	query := fmt.Sprintf("DELETE FROM jobs WHERE JobName = '%s'", jobName)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to delete job: %v", err)
		return err
	}
	return nil
}
func StopJob(jobName string) error {

	query := fmt.Sprintf("ALTER TABLE jobs UPDATE Status = '%s' WHERE JobName = '%s'", "cancelled", jobName)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to stop job: %v", err)
		return err
	}
	return nil
}
func GetAllJobs() ([]Job, error) {
	rows, err := db.Query(`SELECT JobName, ProviderName, Proxy, Pool, Type, Status, Threads, URL FROM jobs ORDER BY Status`)
	if err != nil {
		log.Printf("Failed to get all jobs: %v", err)
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		err := rows.Scan(&job.JobName, &job.ProviderName, &job.Proxy, &job.Pool, &job.Type, &job.Status, &job.Threads, &job.URL)
		if err != nil {
			log.Printf("Failed to scan job: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating over job rows: %v", err)
		return nil, err
	}

	return jobs, nil
}

func GetPendingJobs() ([]Job, error) {
	rows, err := db.Query(`
	SELECT JobName, ProviderName, Proxy, Pool, Type, Status, Threads, URL
	FROM jobs
	WHERE Status NOT IN ('running', 'failed')
	`)
	if err != nil {
		log.Printf("Failed to get pending jobs: %v", err)
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		err := rows.Scan(&job.JobName, &job.ProviderName, &job.Proxy, &job.Pool, &job.Type, &job.Status, &job.Threads, &job.URL)
		if err != nil {
			log.Printf("Failed to scan job: %v", err)
			continue
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating over job rows: %v", err)
		return nil, err
	}

	return jobs, nil
}

func InsertJob(job Job) error {
	query := fmt.Sprintf("INSERT INTO jobs (JobName, ProviderName, Proxy, Pool, Type, Status, Threads, URL) VALUES ('%s', '%s', '%s', '%s', '%s', '%s', %d, '%s')", job.JobName, job.ProviderName, job.Proxy, job.Pool, job.Type, job.Status, job.Threads, job.URL)
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Failed to insert job: %v", err)
		return err
	}
	return nil
}

func createTables() {
	query := `
CREATE TABLE IF NOT EXISTS providers
(
    ID UUID DEFAULT generateUUIDv4(),
    Name String,
    Domain String
) ENGINE = MergeTree()
      ORDER BY ID
      SETTINGS index_granularity = 8192;
`
	if _, err := db.Exec(query); err != nil {
		panic(err)
	}
	query = `
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
`
	if _, err := db.Exec(query); err != nil {
		panic(err)
	}
	query = `
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
      SETTINGS index_granularity = 8192;`
	if _, err := db.Exec(query); err != nil {
		panic(err)
	}
	query = `CREATE TABLE IF NOT EXISTS job_heartbeats
(
    HeartbeatID UUID DEFAULT generateUUIDv4(),
    JobName String,
    Status String,
    Timestamp DateTime DEFAULT now(),
    Message String
) ENGINE = MergeTree()
      ORDER BY (JobName, Timestamp)
      SETTINGS index_granularity = 8192;
`
	if _, err := db.Exec(query); err != nil {
		panic(err)
	}
}

type Job struct {
	JobName       string
	ProviderName  string
	Proxy         string
	Pool          string
	Type          string
	Status        string
	Threads       int
	URL           string
	LastHeartbeat string
}

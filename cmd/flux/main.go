package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"flux/internal/database"
	"flux/internal/request"
	"flux/internal/roundtrip"
	"flux/pkg/maxmind"
)

var jobCancelMap sync.Map

func main() {
	time.Sleep(10 * time.Second)
	database.InitDatabase("tcp://clickhouse:9000")

	for {
		jobs, err := database.GetPendingJobs()
		if err != nil {
			log.Fatalf("Failed to get pending jobs: %v", err)
		}

		if len(jobs) == 0 {
			log.Println("No pending jobs found.")
			time.Sleep(1 * time.Minute)
			continue
		}

		for _, job := range jobs {
			log.Printf("Processing job: %s", job.JobName)

			if _, exists := jobCancelMap.Load(job.JobName); exists {
				currentStatus, err := database.GetJobStatus(job.JobName)
				if err != nil {
					log.Printf("Failed to get job status for %s: %v", job.JobName, err)
					cancelJob(job.JobName)
					continue
				}

				if currentStatus == "cancelled" || currentStatus == "deleted" {
					cancelJob(job.JobName)
				} else {
					log.Printf("Job %s is already running.", job.JobName)
				}
				continue
			}

			if job.Status == "pending" {
				log.Printf("Starting job: %s", job.JobName)

				err = database.UpdateJobStatus(job.JobName, "running")
				if err != nil {
					log.Printf("Failed to update job status for %s: %v", job.JobName, err)
					continue
				}

				ctx, cancel := context.WithCancel(context.Background())
				jobCancelMap.Store(job.JobName, cancel)

				go runJob(ctx, job)
			}
		}

		for _, job := range jobs {
			if job.Status == "running" {
				currentStatus, err := database.GetJobStatus(job.JobName)
				if err != nil {
					log.Printf("Failed to get job status for %s: %v", job.JobName, err)
					cancelJob(job.JobName)
					continue
				}

				if currentStatus == "cancelled" || currentStatus == "deleted" {
					cancelJob(job.JobName)
				}
			}
		}

		time.Sleep(1 * time.Minute)
	}
}
func runJob(ctx context.Context, job database.Job) {
	var wg sync.WaitGroup
	wg.Add(job.Threads)

	clientPool := make(chan *http.Client, job.Threads)
	results := make(chan database.RequestData, job.Threads*10)

	for i := 0; i < job.Threads; i++ {
		job.Proxy = strings.TrimSpace(job.Proxy)
		client, err := roundtrip.NewHTTPClient(job.Proxy)
		if err != nil {
			err = database.UpdateJobStatus(job.JobName, "failed")
			log.Printf("Failed to create HTTP client: %v", err)
		}
		clientPool <- client
	}

	for i := 0; i < job.Threads; i++ {
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					log.Printf("Job %s is cancelled. Exiting goroutine.", job.JobName)
					return
				case client := <-clientPool:
					reqResult, err := request.FetchIP(client, job.URL)
					success := uint8(1)
					errorMessage := ""
					statusCode := uint16(0)
					var geo maxmind.GeoData

					if err != nil {
						success = 0
						errorMessage = err.Error()
						reqResult = &request.Request{IP: "", TimeTaken: 0, StatusCode: 500}
					} else {
						ip := net.ParseIP(strings.TrimSpace(reqResult.IP))
						if ip == nil {
							success = 0
							errorMessage = "Invalid IP address"
						} else {
							geo, err = maxmind.LookupIp(ip)
							if err != nil {
								success = 0
								errorMessage = err.Error()
							} else {
								statusCode = uint16(reqResult.StatusCode)
							}
						}
					}
					inserted := database.RequestData{
						ProviderID:    job.ProviderName,
						IP:            reqResult.IP,
						TimeTaken:     float32(reqResult.TimeTaken.Milliseconds()),
						StatusCode:    statusCode,
						RequestTime:   time.Now().Format("2006-01-02 15:04:05"),
						ContinentCode: geo.Continent.Code,
						ContinentName: geo.Continent.Names.English,
						CountryISO:    geo.Country.ISOCode,
						CountryName:   geo.Country.Names.English,
						CityName:      geo.City.Names.English,
						Latitude:      float32(geo.Location.Latitude),
						Longitude:     float32(geo.Location.Longitude),
						Accuracy:      uint16(geo.Location.AccuracyRadius),
						TimeZone:      geo.Location.TimeZone,
						PostalCode:    geo.Postal.Code,
						ErrorMessage:  errorMessage,
						Success:       success,
						ProxyType:     job.Type,
						Pool:          job.Pool,
					}
					log.Printf("Inserted: %+v", inserted)
					results <- inserted
					clientPool <- client
				}
			}
		}()
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("Stopping heartbeat for job %s", job.JobName)
				return
			case <-time.After(1 * time.Minute):
				err := database.InsertJobHeartbeat(job.JobName, "running", "Job is running")
				if err != nil {
					log.Printf("Failed to update heartbeat: %v", err)
				}
			}
		}
	}()

	go func() {
		batchSize := 1000
		var batch []database.RequestData

		for result := range results {
            repData, err := ipreputation.CheckIPReputation(result.IP)
            if err == nil {
                result.IsProxy = boolToUint8(repData.IsProxy)
                result.ProxyType = repData.ProxyType
                result.VPNScore = repData.VPNScore
                result.ProxyProvider = repData.ProxyProvider
            }

            batch = append(batch, result)
            if len(batch) >= batchSize {
                if err := database.InsertRequests(batch); err != nil {
                    log.Printf("Failed to insert batch: %v", err)
                }
                batch = batch[:0]
            }
		}

		if len(batch) > 0 {
			if err := database.InsertRequests(batch); err != nil {
				log.Printf("Failed to insert last batch: %v", err)
			}
		}
	}()

	wg.Wait()
	close(results)
	log.Println("All goroutines completed.")

	jobCancelMap.Delete(job.JobName)

	select {
	case <-ctx.Done():
		err := database.UpdateJobStatus(job.JobName, "cancelled")
		if err != nil {
			log.Printf("Failed to update job status to cancelled: %v", err)
		}
	default:
		err := database.UpdateJobStatus(job.JobName, "completed")
		if err != nil {
			log.Printf("Failed to update job status to completed: %v", err)
		}
	}
}

func boolToUint8(b bool) uint8 {
    if b {
        return 1
    }
    return 0
}

func cancelJob(jobName string) {
	if cancelFunc, exists := jobCancelMap.Load(jobName); exists {
		cancelFunc.(context.CancelFunc)()
		jobCancelMap.Delete(jobName)
		log.Printf("Job %s has been cancelled.", jobName)
	}
}

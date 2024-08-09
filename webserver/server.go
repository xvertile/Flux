package webserver

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"flux/internal/database"
)

var templates = template.Must(template.ParseGlob("/app/webserver/templates/*.html"))

func StartServer() {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/job/create", createJobHandler)
	http.HandleFunc("/job/edit", editJobHandler)
	http.HandleFunc("/job/delete", deleteJobHandler)
	http.HandleFunc("/job/stop", stopJobHandler)

	log.Println("Starting web server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	jobs, err := database.GetAllJobs()
	if err != nil {
		http.Error(w, "Failed to load jobs", http.StatusInternalServerError)
		return
	}

	for i, job := range jobs {
		heartbeat, err := database.GetLastHeartbeat(job.JobName)
		if err != nil {
			log.Printf("Failed to get last heartbeat for job %s: %v", job.JobName, err)
			continue
		}
		jobs[i].LastHeartbeat = heartbeat
	}
	if err := templates.ExecuteTemplate(w, "index.html", jobs); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func createJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := templates.ExecuteTemplate(w, "create.html", nil); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodPost {
		job := database.Job{
			JobName:      r.FormValue("job_name"),
			ProviderName: r.FormValue("provider_name"),
			Proxy:        r.FormValue("proxy"),
			Pool:         r.FormValue("pool"),
			Type:         r.FormValue("type"),
			Status:       "pending",
			Threads:      toInt(r.FormValue("threads")),
			URL:          r.FormValue("url"),
		}
		log.Printf("Creating job: %+v", job)

		err := database.InsertJob(job)
		if err != nil {
			http.Error(w, "Failed to create job", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func editJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		jobName := r.URL.Query().Get("job_name")
		job, err := database.GetJobByName(jobName)
		if err != nil {
			http.Error(w, "Failed to get job", http.StatusInternalServerError)
			return
		}

		if err := templates.ExecuteTemplate(w, "edit.html", job); err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == http.MethodPost {
		jobName := r.FormValue("job_name")
		status := r.FormValue("status")

		if err := database.UpdateJobStatus(jobName, status); err != nil {
			http.Error(w, "Failed to update job", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func deleteJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		jobname := r.FormValue("job_name")
		log.Printf("Deleting job: %s", jobname)

		err := database.DeleteJob(jobname)
		if err != nil {
			http.Error(w, "Failed to delete job", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
func stopJobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		jobname := r.FormValue("job_name")
		log.Printf("Stopping job: %s", jobname)

		err := database.StopJob(jobname)
		if err != nil {
			http.Error(w, "Failed to delete job", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func toInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

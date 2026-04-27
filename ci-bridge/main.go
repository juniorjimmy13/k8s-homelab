package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// WebhookPayload is what we expect to receive in the POST body
type WebhookPayload struct {
	Repo   string `json:"repo"`
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// BuildResult is what we send back to the caller
type BuildResult struct {
	JobName   string `json:"job_name"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Commit    string `json:"commit"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// secret token — in production this would come from an environment variable
const webhookSecret = "homelab-secret"

func getClient() (*kubernetes.Clientset, error) {
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func createBuildJob(client *kubernetes.Clientset, payload WebhookPayload) (string, error) {
	// Generate a unique job name from repo and timestamp
	jobName := fmt.Sprintf("build-%d", time.Now().Unix())

	// Define the Job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: "default",
			Labels: map[string]string{
				"app":    "ci-bridge",
				"repo":   payload.Repo,
				"branch": payload.Branch,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "build",
							Image: "alpine",
							Command: []string{
								"sh", "-c",
								fmt.Sprintf(
									"echo 'Starting build for %s on branch %s commit %s' && sleep 5 && echo 'Build complete'",
									payload.Repo, payload.Branch, payload.Commit,
								),
							},
						},
					},
				},
			},
		},
	}

	_, err := client.BatchV1().Jobs("default").Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return jobName, nil
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Validate the secret token from the header
	token := r.Header.Get("X-Webhook-Secret")
	if token != webhookSecret {
		log.Printf("rejected request with invalid token: %q", token)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse the JSON body
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if payload.Repo == "" || payload.Branch == "" {
		http.Error(w, "repo and branch are required", http.StatusBadRequest)
		return
	}

	log.Printf("received webhook: repo=%s branch=%s commit=%s", payload.Repo, payload.Branch, payload.Commit)

	// Connect to Kubernetes and create a Job
	client, err := getClient()
	if err != nil {
		log.Printf("failed to connect to cluster: %v", err)
		http.Error(w, "failed to connect to cluster", http.StatusInternalServerError)
		return
	}

	jobName, err := createBuildJob(client, payload)
	if err != nil {
		log.Printf("failed to create job: %v", err)
		http.Error(w, "failed to create build job", http.StatusInternalServerError)
		return
	}

	log.Printf("created job: %s", jobName)

	// Return the result
	result := BuildResult{
		JobName:   jobName,
		Repo:      payload.Repo,
		Branch:    payload.Branch,
		Commit:    payload.Commit,
		Status:    "triggered",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(result)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	http.HandleFunc("/webhook", handleWebhook)
	http.HandleFunc("/health", handleHealth)

	log.Println("ci-bridge listening on :9000")
	if err := http.ListenAndServe(":9000", nil); err != nil {
		log.Fatal(err)
	}
}

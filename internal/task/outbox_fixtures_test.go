package task

import "time"

// taskCreatedGoldenPayload is the exact task.created payload for
// sampleOutboxTask. It pins the format the download service consumes.
const taskCreatedGoldenPayload = `{"task_id":42,"of_account_id":7,"file_name":"file.bin",` +
	`"source_url":"https://example.test/file.bin","source_type":"HTTPS",` +
	`"source_auth":{"type":"basic","username":"user","password":"secret"},` +
	`"download_options":{"concurrency":16,"max_retries":3},` +
	`"metadata":{"origin":"test"},"created_at":"2026-01-02T03:04:05Z"}`

// freshTaskCreatedGoldenPayload is the payload for freshOutboxTask: a task
// exactly as CreateTask hands it over (no auth, no metadata, zero CreatedAt).
const freshTaskCreatedGoldenPayload = `{"task_id":1,"of_account_id":7,"file_name":"/file.bin",` +
	`"source_url":"https://example.test/file.bin","source_type":"HTTPS",` +
	`"download_options":{"concurrency":16,"max_retries":3},` +
	`"created_at":"0001-01-01T00:00:00Z"}`

func sampleOutboxTask() *Task {
	return &Task{
		ID:          42,
		OfAccountID: 7,
		FileName:    "file.bin",
		SourceURL:   "https://example.test/file.bin",
		SourceType:  SourceHTTPS,
		SourceAuth: &AuthConfig{
			Type:     "basic",
			Username: "user",
			Password: "secret",
		},
		DownloadOptions: &DownloadOptions{Concurrency: 16, MaxRetries: 3},
		Metadata:        map[string]any{"origin": "test"},
		Status:          StatusPending,
		CreatedAt:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func freshOutboxTask() *Task {
	return &Task{
		ID:              1,
		OfAccountID:     7,
		FileName:        "/file.bin",
		SourceURL:       "https://example.test/file.bin",
		SourceType:      SourceHTTPS,
		DownloadOptions: &DownloadOptions{Concurrency: 16, MaxRetries: 3},
		Status:          StatusPending,
	}
}

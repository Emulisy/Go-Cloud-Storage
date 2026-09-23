package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goCloudStorage/auth"
	"goCloudStorage/cache"

	"github.com/redis/go-redis/v9"
)

//initialize mp upload
func InitialMPUploadHandler(
	w http.ResponseWriter,
	r *http.Request,
	user auth.User,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse the request.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	fileHash := strings.TrimSpace(r.PostForm.Get("filehash"))

	fileSize, err := strconv.ParseInt(
		r.PostForm.Get("filesize"),
		10,
		64,
	)
	if err != nil || fileSize <= 0 {
		http.Error(w, "Invalid file size", http.StatusBadRequest)
		return
	}

	if len(fileHash) != 64 {
		http.Error(w, "Invalid SHA-256 hash", http.StatusBadRequest)
		return
	}

	// 2. Get the shared Redis client.
	redisClient := cache.RedisClient()
	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}

	// 3. Generate a random upload ID.
	randomBytes := make([]byte, 16)

	if _, err := rand.Read(randomBytes); err != nil {
		http.Error(w, "Unable to initialize upload", http.StatusInternalServerError)
		return
	}

	uploadID := hex.EncodeToString(randomBytes)

	// 4. Calculate chunk information.
	const chunkSize int64 = 5 * 1024 * 1024 // 5 MiB

	chunkCount := (fileSize-1)/chunkSize + 1

	info := cache.MPUploadInfo{
		FileHash:   fileHash,
		FileSize:   fileSize,
		UploadID:   uploadID,
		ChunkSize:  chunkSize,
		ChunkCount: chunkCount,
	}

	// 5. Save upload information in Redis.
	key := "upload:" + uploadID

	_, err = redisClient.TxPipelined(
		r.Context(),
		func(pipe redis.Pipeliner) error {
			pipe.HSet(r.Context(), key, map[string]any{
				"username":    user.Username,
				"file_hash":   info.FileHash,
				"file_size":   info.FileSize,
				"upload_id":   info.UploadID,
				"chunk_size":  info.ChunkSize,
				"chunk_count": info.ChunkCount,
				"status":      "uploading",
			})

			pipe.Expire(r.Context(), key, 24*time.Hour)

			return nil
		},
	)

	if err != nil {
		http.Error(w, "Unable to save upload information", http.StatusInternalServerError)
		return
	}

	// 6. Return upload information to the frontend.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(info); err != nil {
		// The response has already started; log this error.
		fmt.Printf("Encode upload information: %v\n", err)
	}
}
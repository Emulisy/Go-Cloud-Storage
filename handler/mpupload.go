package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"goCloudStorage/auth"
	"goCloudStorage/cache"
	"goCloudStorage/db"
	"goCloudStorage/util"

	"github.com/redis/go-redis/v9"
)

// initialize mp upload, return the data for later partition
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

// UploadPartHandler receives one raw chunk for an owned upload session.
func UploadPartHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Read chunk identifiers without consuming the binary body.
	uploadID := r.URL.Query().Get("uploadid")
	index, err := strconv.ParseInt(r.URL.Query().Get("index"), 10, 64)
	decoded, decodeErr := hex.DecodeString(uploadID)
	if err != nil || index < 0 || decodeErr != nil || len(decoded) != 16 {
		http.Error(w, "Invalid chunk identifiers", http.StatusBadRequest)
		return
	}
	client := cache.RedisClient()
	if client == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}

	// 2. Check ownership and the expected size of this chunk.
	key := "upload:" + uploadID
	info, err := client.HGetAll(r.Context(), key).Result()
	if err != nil {
		http.Error(w, "Unable to read upload session", http.StatusInternalServerError)
		return
	}
	if len(info) == 0 || info["username"] != user.Username {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}
	count, countErr := strconv.ParseInt(info["chunk_count"], 10, 64)
	size, sizeErr := strconv.ParseInt(info["file_size"], 10, 64)
	chunkSize, chunkErr := strconv.ParseInt(info["chunk_size"], 10, 64)
	if countErr != nil || sizeErr != nil || chunkErr != nil || size <= 0 || chunkSize <= 0 || count != (size-1)/chunkSize+1 {
		http.Error(w, "Invalid upload metadata", http.StatusInternalServerError)
		return
	}
	if index >= count || info["status"] != "uploading" {
		http.Error(w, "Chunk is outside an active upload", http.StatusConflict)
		return
	}
	expected := chunkSize
	if index == count-1 {
		expected = size - index*chunkSize
	}

	// 3. Write to a temporary file; publish only a complete chunk.
	directory := filepath.Join(os.TempDir(), "gocloudstorage-parts", uploadID)
	if err := os.MkdirAll(directory, 0700); err != nil {
		http.Error(w, "Unable to create chunk directory", http.StatusInternalServerError)
		return
	}
	part, err := os.CreateTemp(directory, "part-*")
	if err != nil {
		http.Error(w, "Unable to create chunk", http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = part.Close()
		_ = os.Remove(part.Name())
	}()
	written, copyErr := io.Copy(part, http.MaxBytesReader(w, r.Body, expected))
	closeErr := part.Close()
	if copyErr != nil || written != expected {
		http.Error(w, "Incomplete or oversized chunk", http.StatusBadRequest)
		return
	}
	if closeErr != nil {
		http.Error(w, "Unable to save chunk", http.StatusInternalServerError)
		return
	}
	chunkIndex := strconv.FormatInt(index, 10)
	if err := os.Rename(part.Name(), filepath.Join(directory, chunkIndex)); err != nil {
		http.Error(w, "Unable to save chunk", http.StatusInternalServerError)
		return
	}

	// 4. Record the chunk and keep its status bounded by the session lifetime.
	_, err = client.TxPipelined(r.Context(), func(pipe redis.Pipeliner) error {
		pipe.SAdd(r.Context(), key+":chunks", chunkIndex)
		pipe.Expire(r.Context(), key+":chunks", 24*time.Hour)
		return nil
	})
	if err != nil {
		http.Error(w, "Unable to record chunk", http.StatusInternalServerError)
		return
	}

	// 5. Return the upload result.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "OK", "uploadId": uploadID, "index": index,
	})
}

// combiner parts
func CompleteUploadHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	//1. parse form and get redis client
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	uploadID := strings.TrimSpace(r.PostForm.Get("uploadid"))
	fileName := strings.TrimSpace(r.PostForm.Get("filename"))
	if decoded, err := hex.DecodeString(uploadID); err != nil || len(decoded) != 16 {
		http.Error(w, "Invalid upload ID", http.StatusBadRequest)
		return
	}
	if fileName == "" || len(fileName) > 255 {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	redisClient := cache.RedisClient()
	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	sessionKey := "upload:" + uploadID
	chunkKey := sessionKey + ":chunks"

	//2. check if all parts are uploaded ready to merge
	info, err := redisClient.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		http.Error(w, "Failed to retrieve upload information", http.StatusInternalServerError)
		return
	}
	if len(info) == 0 || info["username"] != user.Username {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}
	if info["status"] != "uploading" {
		http.Error(w, "Upload is not in progress", http.StatusConflict)
		return
	}
	totalCount, countErr := strconv.ParseInt(info["chunk_count"], 10, 64)
	expectedSize, sizeErr := strconv.ParseInt(info["file_size"], 10, 64)
	if countErr != nil || sizeErr != nil || totalCount <= 0 || expectedSize <= 0 {
		http.Error(w, "Invalid upload information", http.StatusInternalServerError)
		return
	}
	uploadedCount, err := redisClient.SCard(ctx, chunkKey).Result()
	if err != nil {
		http.Error(w, "Failed to check chunk status", http.StatusInternalServerError)
		return
	}
	if uploadedCount != totalCount {
		http.Error(w, "Not all chunks have been uploaded", http.StatusConflict)
		return
	}

	//3. merge the chunks
	newFile, err := os.CreateTemp("", "gocloudstorage-*")
	if err != nil {
		http.Error(w, "Failed to create merged file", http.StatusInternalServerError)
		return
	}
	storedFile := db.StoredFile{Addr: newFile.Name()}
	keepFile := false
	defer func() {
		_ = newFile.Close()
		if !keepFile {
			_ = os.Remove(storedFile.Addr)
		}
	}()

	chunkDirectory := filepath.Join(os.TempDir(), "gocloudstorage-parts", uploadID)
	for i := int64(0); i < totalCount; i++ {
		chunk, err := os.Open(filepath.Join(chunkDirectory, strconv.FormatInt(i, 10)))
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to read chunk %d", i), http.StatusConflict)
			return
		}
		written, copyErr := io.Copy(newFile, chunk)
		closeErr := chunk.Close()
		if copyErr != nil || closeErr != nil {
			http.Error(w, "Failed to merge chunks", http.StatusInternalServerError)
			return
		}
		storedFile.Size += written
	}
	if _, err := newFile.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Failed to read merged file", http.StatusInternalServerError)
		return
	}
	storedFile.Hash, err = util.CalculateSHA256(newFile)
	if err != nil {
		http.Error(w, "Failed to hash merged file", http.StatusInternalServerError)
		return
	}
	if storedFile.Size != expectedSize || !strings.EqualFold(storedFile.Hash, info["file_hash"]) {
		http.Error(w, "Uploaded file does not match its size or hash", http.StatusUnprocessableEntity)
		return
	}
	if err := newFile.Close(); err != nil {
		http.Error(w, "Failed to close merged file", http.StatusInternalServerError)
		return
	}

	//4. update the tbl_file and tbl_user_file
	keepFile, err = db.StoreUserFile(user.Username, fileName, storedFile)
	if err != nil {
		log.Printf("Store multipart upload: %v", err)
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	if err := redisClient.Del(ctx, sessionKey, chunkKey).Err(); err != nil {
		log.Printf("Remove multipart upload state: %v", err)
	}
	for i := int64(0); i < totalCount; i++ {
		if err := os.Remove(filepath.Join(chunkDirectory, strconv.FormatInt(i, 10))); err != nil {
			log.Printf("Remove uploaded chunk: %v", err)
		}
	}
	if err := os.Remove(chunkDirectory); err != nil {
		log.Printf("Remove upload directory: %v", err)
	}

	//5. return result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status": "OK", "uploadId": uploadID,
		"fileHash": storedFile.Hash, "fileSize": storedFile.Size,
	}); err != nil {
		log.Printf("Encode upload result: %v", err)
	}
}

package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"goCloudStorage/auth"
	"goCloudStorage/cache"
	"goCloudStorage/db"
	"goCloudStorage/storage"
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
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse the request.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	fileHash := strings.ToLower(
		strings.TrimSpace(r.PostForm.Get("filehash")),
	)

	if len(fileHash) != 64 {
		http.Error(w, "Invalid SHA-256 hash", http.StatusBadRequest)
		return
	}

	if _, err := hex.DecodeString(fileHash); err != nil {
		http.Error(w, "Invalid SHA-256 hash", http.StatusBadRequest)
		return
	}

	fileName := strings.TrimSpace(r.PostForm.Get("filename"))
	if fileName == "" || len(fileName) > 255 {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	fileSize, err := strconv.ParseInt(
		r.PostForm.Get("filesize"),
		10,
		64,
	)
	if err != nil || fileSize <= 0 {
		http.Error(w, "Invalid file size", http.StatusBadRequest)
		return
	}

	// 2. Get the shared Redis client.
	redisClient := cache.RedisClient()
	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}

	// 3. Generate a session key with file hash and user ID
	sessionKey := "mpupload:session:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash

	//Check if there is existing upload session for the file
	existingSession, err := redisClient.HGetAll(
		r.Context(),
		sessionKey,
	).Result()

	if err != nil {
		http.Error(
			w,
			"Unable to retrieve upload session",
			http.StatusInternalServerError,
		)
		return
	}

	//if found existing session
	if len(existingSession) > 0 {

		// Verify that the existing session matches the requested file.
		if existingSession["user_id"] != strconv.FormatInt(user.UserID, 10) ||
			existingSession["file_hash"] != fileHash ||
			existingSession["file_size"] != strconv.FormatInt(fileSize, 10) {

			http.Error(
				w,
				"Existing upload session does not match",
				http.StatusConflict,
			)
			return
		}

		if existingSession["r2_upload_id"] == "" || existingSession["object_key"] == "" {
			http.Error(w, "Legacy upload session cannot be resumed in R2", http.StatusConflict)
			return
		}

		if existingSession["status"] != "uploading" {
			http.Error(
				w,
				"Upload session is not active",
				http.StatusConflict,
			)
			return
		}

		// Retrieve the original chunk configuration.
		existingChunkSize, err := strconv.ParseInt(
			existingSession["chunk_size"],
			10,
			64,
		)

		if err != nil || existingChunkSize <= 0 {
			http.Error(
				w,
				"Invalid stored chunk size",
				http.StatusInternalServerError,
			)
			return
		}

		existingChunkCount, err := strconv.ParseInt(
			existingSession["chunk_count"],
			10,
			64,
		)

		if err != nil ||
			existingChunkCount > 10000 || existingChunkCount != (fileSize-1)/existingChunkSize+1 {

			http.Error(
				w,
				"Invalid stored chunk count",
				http.StatusInternalServerError,
			)
			return
		}

		createdAt, err := time.Parse(
			time.RFC3339Nano,
			existingSession["created_at"],
		)

		if err != nil {
			http.Error(
				w,
				"Invalid stored creation time",
				http.StatusInternalServerError,
			)
			return
		}

		// Return the original session.
		info := cache.MPUploadInfo{
			FileHash:   fileHash,
			FileSize:   fileSize,
			FileName:   existingSession["file_name"],
			UploadID:   sessionKey,
			ChunkSize:  existingChunkSize,
			ChunkCount: existingChunkCount,
			Status:     "uploading",
			CreatedAt:  createdAt,
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(info); err != nil {
			log.Printf("Encode existing upload session: %v", err)
		}

		return
	}

	// 4. Calculate chunk information.
	const chunkSize int64 = 5 * 1024 * 1024 // 5 MiB

	chunkCount := (fileSize-1)/chunkSize + 1
	if chunkCount > 10000 {
		http.Error(w, "File exceeds the multipart part limit", http.StatusBadRequest)
		return
	}

	objectKey := "objects/" + fileHash + "/" + rand.Text()
	r2UploadID, err := storage.R2().CreateMultipartUpload(r.Context(), objectKey, "application/octet-stream")
	if err != nil {
		log.Printf("Create R2 multipart upload: %v", err)
		http.Error(w, "Unable to create multipart upload", http.StatusInternalServerError)
		return
	}

	info := cache.MPUploadInfo{
		FileHash:   fileHash,
		FileSize:   fileSize,
		FileName:   fileName,
		UploadID:   sessionKey,
		ChunkSize:  chunkSize,
		ChunkCount: chunkCount,
		Status:     "uploading",
		CreatedAt:  time.Now(),
	}

	_, err = redisClient.TxPipelined(
		r.Context(),
		func(pipe redis.Pipeliner) error {
			// A new R2 upload must not reuse stale part ETags.
			pipe.Del(r.Context(), "mpupload:chunks:userid:"+strconv.FormatInt(user.UserID, 10)+":"+fileHash)
			pipe.HSet(r.Context(), sessionKey, map[string]any{
				"user_id":      strconv.FormatInt(user.UserID, 10),
				"file_hash":    info.FileHash,
				"file_size":    info.FileSize,
				"file_name":    info.FileName,
				"upload_id":    info.UploadID,
				"r2_upload_id": r2UploadID,
				"object_key":   objectKey,
				"chunk_size":   info.ChunkSize,
				"chunk_count":  info.ChunkCount,
				"status":       "uploading",
				"created_at":   info.CreatedAt,
			})

			pipe.Expire(r.Context(), sessionKey, 24*time.Hour)

			return nil
		},
	)

	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
		defer cancel()
		if abortErr := storage.R2().AbortMultipartUpload(cleanupCtx, objectKey, r2UploadID); abortErr != nil {
			log.Printf("Abort unsaved R2 multipart upload %q: %v", objectKey, abortErr)
		}
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
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Read chunk identifiers.
	fileHash := strings.ToLower(
		strings.TrimSpace(r.PathValue("filehash")),
	)

	index, err := strconv.ParseInt(
		r.PathValue("index"),
		10,
		64,
	)

	if err != nil || index < 0 {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	// Validate SHA-256.
	if len(fileHash) != 64 {
		http.Error(w, "Invalid file hash", http.StatusBadRequest)
		return
	}

	if _, err := hex.DecodeString(fileHash); err != nil {
		http.Error(w, "Invalid file hash", http.StatusBadRequest)
		return
	}

	redisClient := cache.RedisClient()
	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}

	// 2. Check ownership and the expected size of this chunk.
	sessionKey := "mpupload:session:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash

	chunkKey := "mpupload:chunks:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash
	info, err := redisClient.HGetAll(r.Context(), sessionKey).Result()
	if err != nil {
		http.Error(w, "Unable to read upload session", http.StatusInternalServerError)
		return
	}
	if len(info) == 0 ||
		info["user_id"] != strconv.FormatInt(user.UserID, 10) ||
		info["file_hash"] != fileHash {

		http.Error(
			w,
			"Upload session not found",
			http.StatusNotFound,
		)
		return
	}

	count, countErr := strconv.ParseInt(info["chunk_count"], 10, 64)
	size, sizeErr := strconv.ParseInt(info["file_size"], 10, 64)
	chunkSize, chunkErr := strconv.ParseInt(info["chunk_size"], 10, 64)
	if countErr != nil || sizeErr != nil || chunkErr != nil || size <= 0 || chunkSize <= 0 || count > 10000 || count != (size-1)/chunkSize+1 {
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

	// 3. Stream the chunk to R2; public chunk indices remain zero-based.
	if info["r2_upload_id"] == "" || info["object_key"] == "" {
		http.Error(w, "Upload session has no R2 upload", http.StatusConflict)
		return
	}
	if r.ContentLength >= 0 && r.ContentLength != expected {
		http.Error(w, "Incomplete or oversized chunk", http.StatusBadRequest)
		return
	}
	body := &io.LimitedReader{R: r.Body, N: expected + 1}
	part, err := storage.R2().UploadPart(r.Context(), info["object_key"], info["r2_upload_id"], int32(index+1), body, expected)
	if err != nil {
		log.Printf("Upload R2 part %d: %v", index+1, err)
		http.Error(w, "Unable to upload chunk", http.StatusInternalServerError)
		return
	}
	if body.N != 1 {
		http.Error(w, "Incomplete or oversized chunk", http.StatusBadRequest)
		return
	}
	chunkIndex := strconv.FormatInt(index, 10)

	// 4. Refresh the remaining session lifetime and record the success uploaded chunk
	_, err = redisClient.TxPipelined(
		r.Context(),
		func(pipe redis.Pipeliner) error {

			// Record the successfully uploaded chunk.
			pipe.HSet(
				r.Context(),
				chunkKey,
				chunkIndex,
				part.ETag,
			)

			// Refresh the upload session expiry.
			pipe.Expire(
				r.Context(),
				sessionKey,
				24*time.Hour,
			)

			// Refresh the chunk tracking expiry.
			pipe.Expire(
				r.Context(),
				chunkKey,
				24*time.Hour,
			)

			return nil
		},
	)

	if err != nil {
		http.Error(
			w,
			"Unable to record chunk",
			http.StatusInternalServerError,
		)
		return
	}

	// 5. Return the upload result.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "OK", "uploadId": sessionKey, "index": index,
	})
}

// UploadCompleteHandler asks R2 to assemble the uploaded parts.
func UploadCompleteHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	//1. parse form and get redis client
	fileHash := strings.ToLower(
		strings.TrimSpace(r.PathValue("filehash")),
	)
	if len(fileHash) != 64 {
		http.Error(w, "Invalid SHA-256 hash", http.StatusBadRequest)
		return
	}
	if _, err := hex.DecodeString(fileHash); err != nil {
		http.Error(w, "Invalid SHA-256 hash", http.StatusBadRequest)
		return
	}
	redisClient := cache.RedisClient()
	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()

	//verify all chunks have been uploaded
	sessionKey := "mpupload:session:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash

	chunkKey := "mpupload:chunks:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash

	info, err := redisClient.HGetAll(ctx, sessionKey).Result()
	if err != nil {
		http.Error(w, "Failed to retrieve upload information", http.StatusInternalServerError)
		return
	}
	if len(info) == 0 || info["user_id"] != strconv.FormatInt(user.UserID, 10) || info["file_hash"] != fileHash {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}
	if info["status"] != "uploading" {
		http.Error(w, "Upload is not in progress", http.StatusConflict)
		return
	}
	if info["r2_upload_id"] == "" || info["object_key"] == "" {
		http.Error(w, "Upload session has no R2 upload", http.StatusConflict)
		return
	}
	totalCount, countErr := strconv.ParseInt(info["chunk_count"], 10, 64)
	expectedSize, sizeErr := strconv.ParseInt(info["file_size"], 10, 64)
	fileName := info["file_name"]
	if countErr != nil || sizeErr != nil || totalCount <= 0 || totalCount > 10000 || expectedSize <= 0 {
		http.Error(w, "Invalid upload information", http.StatusInternalServerError)
		return
	}

	uploadedChunks, err := redisClient.HGetAll(
		ctx,
		chunkKey,
	).Result()

	if err != nil {
		http.Error(w, "Failed to retrieve uploaded chunks", http.StatusInternalServerError)
		return
	}
	//Verify the number of uploaded chunks.
	if int64(len(uploadedChunks)) != totalCount {
		http.Error(w, "Not all chunks have been uploaded", http.StatusConflict)
		return
	}

	// 3. Build the ordered list of R2 part numbers and ETags.
	parts := make([]storage.MultipartPart, 0, totalCount)
	for i := int64(0); i < totalCount; i++ {
		chunkIndex := strconv.FormatInt(i, 10)
		etag := uploadedChunks[chunkIndex]
		if etag == "" {
			http.Error(w, "Missing chunk: "+chunkIndex, http.StatusConflict)
			return
		}
		parts = append(parts, storage.MultipartPart{PartNumber: int32(i + 1), ETag: etag})
	}

	// 4. R2 assembles the object. Stream it back only to preserve SHA-256
	// verification before shared content is recorded in MySQL; no local files.
	completeErr := storage.R2().CompleteMultipartUpload(ctx, info["object_key"], info["r2_upload_id"], parts)
	object, err := storage.R2().GetObject(ctx, info["object_key"])
	if err != nil {
		log.Printf("Complete/read R2 multipart upload: complete=%v, get=%v", completeErr, err)
		http.Error(w, "Failed to complete or verify upload", http.StatusInternalServerError)
		return
	}
	// A prior completion may have succeeded even if its response was lost.
	verifiedHash, hashErr := util.CalculateSHA256(object.Body)
	closeErr := object.Body.Close()
	if hashErr != nil || closeErr != nil {
		log.Printf("Verify R2 multipart object: read=%v, close=%v", hashErr, closeErr)
		http.Error(w, "Failed to verify uploaded file", http.StatusInternalServerError)
		return
	}
	if object.Size != expectedSize || verifiedHash != fileHash {
		if err := removeStoredFile(ctx, "r2://"+info["object_key"]); err != nil {
			log.Printf("Remove invalid R2 object: %v", err)
		}
		if err := redisClient.Del(ctx, sessionKey, chunkKey).Err(); err != nil {
			log.Printf("Remove invalid multipart upload state: %v", err)
		}
		http.Error(w, "Uploaded file size or hash does not match", http.StatusConflict)
		return
	}

	//5. update the mysql upon successful upload
	storedFile := db.StoredFile{}

	storedFile.Addr = "r2://" + info["object_key"]
	storedFile.Hash = verifiedHash
	storedFile.Size = object.Size

	contentCreated, err := db.StoreUserFile(
		user.UserID,
		fileName,
		storedFile,
	)
	if err != nil {
		// Retain the object when the database commit outcome is uncertain.
		log.Printf("failed to store user file; retained %q for reconciliation: %v", storedFile.Addr, err)
		http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	if !contentCreated {
		// A retried completion may already have committed this exact object.
		shared, err := db.GetStoredFile(storedFile.Hash)
		if err != nil {
			log.Printf("Check redundant R2 object %q: %v", storedFile.Addr, err)
		} else if shared.Addr != storedFile.Addr {
			if err := removeStoredFile(ctx, storedFile.Addr); err != nil {
				log.Printf("Remove redundant R2 object %q: %v", storedFile.Addr, err)
			}
		}
	}

	// Retire the upload; chunk bytes were stored only in R2.
	if err := redisClient.Del(ctx, sessionKey, chunkKey).Err(); err != nil {
		log.Printf("Remove multipart upload state: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"status": "OK", "uploadId": sessionKey,
		"fileHash": storedFile.Hash, "fileSize": storedFile.Size,
	}); err != nil {
		log.Printf("Encode upload result: %v", err)
	}
}

// UploadCancelHandler aborts an active R2 multipart upload and removes its
// resumable upload state.
func UploadCancelHandler(
	w http.ResponseWriter,
	r *http.Request,
	user auth.User,
) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileHash := strings.ToLower(strings.TrimSpace(r.PathValue("filehash")))
	if len(fileHash) != 64 {
		http.Error(w, "Invalid file hash", http.StatusBadRequest)
		return
	}
	if _, err := hex.DecodeString(fileHash); err != nil {
		http.Error(w, "Invalid file hash", http.StatusBadRequest)
		return
	}

	redisClient := cache.RedisClient()
	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}

	userID := strconv.FormatInt(user.UserID, 10)
	sessionKey := "mpupload:session:userid:" + userID + ":" + fileHash
	chunkKey := "mpupload:chunks:userid:" + userID + ":" + fileHash

	info, err := redisClient.HGetAll(r.Context(), sessionKey).Result()
	if err != nil {
		http.Error(w, "Unable to retrieve upload session", http.StatusInternalServerError)
		return
	}
	if len(info) == 0 || info["user_id"] != userID || info["file_hash"] != fileHash {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}
	if info["status"] != "uploading" {
		http.Error(w, "Upload is not active", http.StatusConflict)
		return
	}
	if info["r2_upload_id"] == "" || info["object_key"] == "" {
		http.Error(w, "Upload session has no R2 upload", http.StatusConflict)
		return
	}

	// Finish cleanup even if the client disconnects after requesting cancellation.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 30*time.Second)
	defer cancel()

	if err := storage.R2().AbortMultipartUpload(ctx, info["object_key"], info["r2_upload_id"]); err != nil {
		log.Printf("Abort R2 multipart upload: %v", err)
		http.Error(w, "Unable to cancel upload", http.StatusInternalServerError)
		return
	}

	if err := redisClient.Del(ctx, sessionKey, chunkKey).Err(); err != nil {
		log.Printf("Remove cancelled multipart upload state: %v", err)
		http.Error(w, "Unable to remove upload session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// check the upload status
func UploadStatusHandler(
	w http.ResponseWriter,
	r *http.Request,
	user auth.User,
) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Read and validate the file hash.
	fileHash := strings.ToLower(
		strings.TrimSpace(r.PathValue("filehash")),
	)

	if len(fileHash) != 64 {
		http.Error(w, "Invalid file hash", http.StatusBadRequest)
		return
	}

	if _, err := hex.DecodeString(fileHash); err != nil {
		http.Error(w, "Invalid file hash", http.StatusBadRequest)
		return
	}

	// 2. Get the Redis client.
	redisClient := cache.RedisClient()

	if redisClient == nil {
		http.Error(w, "Redis is not initialized", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	sessionKey := "mpupload:session:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash

	chunkKey := "mpupload:chunks:userid:" + strconv.FormatInt(user.UserID, 10) + ":" + fileHash

	// 3. Retrieve the existing upload session.
	info, err := redisClient.HGetAll(
		ctx,
		sessionKey,
	).Result()

	if err != nil {
		http.Error(w, "Unable to retrieve upload session", http.StatusInternalServerError)
		return
	}

	if len(info) == 0 ||
		info["user_id"] != strconv.FormatInt(user.UserID, 10) ||
		info["file_hash"] != fileHash {

		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}

	if info["status"] != "uploading" {
		http.Error(w, "Upload is not active", http.StatusConflict)
		return
	}

	if info["r2_upload_id"] == "" || info["object_key"] == "" {
		http.Error(w, "Upload session has no R2 upload", http.StatusConflict)
		return
	}

	// 4. Retrieve the expected chunk configuration.
	totalCount, err := strconv.ParseInt(
		info["chunk_count"],
		10,
		64,
	)

	if err != nil || totalCount <= 0 {
		http.Error(w, "Invalid upload information", http.StatusInternalServerError)
		return
	}

	// 5. Retrieve the uploaded chunk indices.
	uploadedChunks, err := redisClient.HKeys(
		ctx,
		chunkKey,
	).Result()

	if err != nil {
		http.Error(w, "Unable to retrieve uploaded chunks", http.StatusInternalServerError)
		return
	}

	// Convert the uploaded indices into a map.
	uploadedMap := make(map[string]bool)

	for _, chunkIndex := range uploadedChunks {
		uploadedMap[chunkIndex] = true
	}

	// 6. Return the indices in ascending order.
	uploadedIndices := make([]int64, 0)

	for i := int64(0); i < totalCount; i++ {

		chunkIndex := strconv.FormatInt(i, 10)

		if uploadedMap[chunkIndex] {
			uploadedIndices = append(uploadedIndices, i)
		}
	}

	// 7. Return the upload progress.
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string]any{
		"fileHash":       fileHash,
		"chunkCount":     totalCount,
		"uploadedChunks": uploadedIndices,
		"status":         info["status"],
	}); err != nil {
		log.Printf("Encode upload status: %v", err)
	}
}

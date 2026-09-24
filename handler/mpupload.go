package handler

import (
	"crypto/sha256"
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
			existingChunkCount != (fileSize-1)/existingChunkSize+1 {

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

	info := cache.MPUploadInfo{
		FileHash:   fileHash,
		FileSize:   fileSize,
		FileName:   fileName,
		UploadID:   sessionKey,
		ChunkSize:  chunkSize,
		ChunkCount: chunkCount,
		CreatedAt:  time.Now(),
	}

	_, err = redisClient.TxPipelined(
		r.Context(),
		func(pipe redis.Pipeliner) error {
			pipe.HSet(r.Context(), sessionKey, map[string]any{
				"user_id":    strconv.FormatInt(user.UserID, 10),
				"file_hash":   info.FileHash,
				"file_size":   info.FileSize,
				"file_name":   info.FileName,
				"upload_id":   info.UploadID,
				"chunk_size":  info.ChunkSize,
				"chunk_count": info.ChunkCount,
				"status":      "uploading",
				"created_at":  info.CreatedAt,
			})

			pipe.Expire(r.Context(), sessionKey, 24*time.Hour)

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

	// 1. Read chunk identifiers.
	fileHash := strings.ToLower(
		strings.TrimSpace(r.URL.Query().Get("filehash")),
	)

	index, err := strconv.ParseInt(
		r.URL.Query().Get("index"),
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
	//create a safe temp directory
	sum := sha256.Sum256(
		[]byte("userid:" + strconv.FormatInt(user.UserID, 10) + "\x00" + fileHash),
	)

	directory := filepath.Join(
		os.TempDir(),
		"gocloudstorage-parts",
		hex.EncodeToString(sum[:]),
	)
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

	// 4. Refresh the remaining session lifetime and record the success uploaded chunk
	_, err = redisClient.TxPipelined(
		r.Context(),
		func(pipe redis.Pipeliner) error {

			// Record the successfully uploaded chunk.
			pipe.SAdd(
				r.Context(),
				chunkKey,
				chunkIndex,
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

// combiner parts
func UploadCompleteHandler(w http.ResponseWriter, r *http.Request, user auth.User) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	//1. parse form and get redis client
	fileHash := strings.ToLower(
		strings.TrimSpace(r.URL.Query().Get("filehash")),
	)
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
	if len(info) == 0 || info["user_id"] != strconv.FormatInt(user.UserID, 10) {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}
	if info["status"] != "uploading" {
		http.Error(w, "Upload is not in progress", http.StatusConflict)
		return
	}
	totalCount, countErr := strconv.ParseInt(info["chunk_count"], 10, 64)
	expectedSize, sizeErr := strconv.ParseInt(info["file_size"], 10, 64)
	fileName := info["file_name"]
	if countErr != nil || sizeErr != nil || totalCount <= 0 || expectedSize <= 0 {
		http.Error(w, "Invalid upload information", http.StatusInternalServerError)
		return
	}

	uploadedChunks, err := redisClient.SMembers(
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

	// 3. Convert the uploaded indices into a map.
	uploadedMap := make(map[string]bool)

	for _, chunkIndex := range uploadedChunks {
		uploadedMap[chunkIndex] = true
	}

	// 4. Verify that every expected index exists.
	for i := int64(0); i < totalCount; i++ {

		chunkIndex := strconv.FormatInt(i, 10)

		if !uploadedMap[chunkIndex] {
			http.Error(w, "Missing chunk: "+chunkIndex, http.StatusConflict)
			return
		}
	}

	//4. merge the chunks

	//Locate the directory containing uploaded chunks.
	sum := sha256.Sum256(
		[]byte("userid:" + strconv.FormatInt(user.UserID, 10) + "\x00" + fileHash),
	)

	directory := filepath.Join(
		os.TempDir(),
		"gocloudstorage-parts",
		hex.EncodeToString(sum[:]),
	)

	// Keep stored content outside the chunk directory so cleanup cannot remove it.
	mergedFile, err := os.CreateTemp(
		"",
		"gocloudstorage-*",
	)

	if err != nil {
		http.Error(w, "Failed to create merged file", http.StatusInternalServerError)
		return
	}

	keepFile := false
	defer func() {
		_ = mergedFile.Close()
		if !keepFile {
			_ = os.Remove(mergedFile.Name())
		}
	}()

	var mergedSize int64 //track the real size of the merged file

	for i := int64(0); i < totalCount; i++ {
		//The path of the current chunk
		chunkPath := filepath.Join(
			directory,
			strconv.FormatInt(i, 10),
		)

		chunk, err := os.Open(chunkPath)
		if err != nil {
			http.Error(w, "Failed to open chunk", http.StatusInternalServerError)
			return
		}
		written, copyErr := io.Copy(
			mergedFile,
			chunk,
		)

		closeErr := chunk.Close()

		if copyErr != nil || closeErr != nil {
			http.Error(w, "Failed to merge chunk", http.StatusInternalServerError)
			return
		}

		mergedSize += written
	}

	//verify merged file size
	if mergedSize != expectedSize {
		http.Error(
			w,
			"Merged file size does not match",
			http.StatusConflict,
		)
		return
	}

	//verify the hash of the merged file
	if _, err := mergedFile.Seek(0, io.SeekStart); err != nil {
		http.Error(w, "Failed to read merged file", http.StatusInternalServerError)
		return
	}
	hasher := sha256.New()

	_, err = io.Copy(hasher, mergedFile)
	if err != nil {
		http.Error(w, "Failed to calculate file hash", http.StatusInternalServerError)
		return
	}
	mergedHash := hex.EncodeToString(hasher.Sum(nil))

	if mergedHash != fileHash {
		http.Error(
			w,
			"Merged file hash does not match",
			http.StatusConflict,
		)
		return
	}

	if err := mergedFile.Close(); err != nil {
		http.Error(w, "Failed to close merged file", http.StatusInternalServerError)
		return
	}

	//5. update the mysql upon successful upload
	storedFile := db.StoredFile{}

	storedFile.Addr = mergedFile.Name()
	storedFile.Hash = mergedHash
	storedFile.Size = mergedSize

	keepFile, err = db.StoreUserFile(
		user.UserID,
		fileName,
		storedFile,
	)
	if err != nil {
		log.Printf("failed to store user file: %v", err)
		http.Error(w, "failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// Retire the upload and remove only its numbered chunk files.
	if err := redisClient.Del(ctx, sessionKey, chunkKey).Err(); err != nil {
		log.Printf("Remove multipart upload state: %v", err)
	}
	for i := int64(0); i < totalCount; i++ {
		if err := os.Remove(filepath.Join(directory, strconv.FormatInt(i, 10))); err != nil {
			log.Printf("Remove uploaded chunk: %v", err)
		}
	}
	if err := os.Remove(directory); err != nil {
		log.Printf("Remove upload directory: %v", err)
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

//check the upload status
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
        strings.TrimSpace(r.URL.Query().Get("filehash")),
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
    uploadedChunks, err := redisClient.SMembers(
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

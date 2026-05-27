//go:build ignore

package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hermesclaw/internal/ai"
	"hermesclaw/internal/config"
	"hermesclaw/internal/content"
	"hermesclaw/internal/model"
	"hermesclaw/internal/store"
)

const batchSize = 10

func main() {
	cfg := config.Load()
	st, err := store.OpenJSON("data/store.json")
	if err != nil {
		fmt.Println("Open store:", err)
		return
	}

	embedder := ai.NewEmbeddingProvider(cfg)
	fmt.Println("Embedder dims:", embedder.Dimensions())

	data, _ := os.ReadFile("data/store.json")
	var db map[string]interface{}
	json.Unmarshal(data, &db)
	mats := db["materials"].(map[string]interface{})
	hashSet := make(map[string]bool)
	for _, v := range mats {
		if m, ok := v.(map[string]interface{}); ok {
			if h, ok := m["sha256"].(string); ok {
				hashSet[h] = true
			}
		}
	}

	type chunkEntry struct {
		id    string
		chunk model.Chunk
	}
	pendingChunks := []chunkEntry{}

	ctx := context.Background()
	scanCount := 0
	imported := 0
	duplicates := 0
	chunkCount := 0

	err = filepath.WalkDir("D:/study/teach/秋季课", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".pdf") {
			return nil
		}
		scanCount++
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Println("Read error:", path, err)
			return nil
		}
		rel, _ := filepath.Rel("D:/study/teach/秋季课", path)
		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])

		if hashSet[hashStr] {
			duplicates++
			return nil
		}

		mat := content.ParseMaterialPath(filepath.ToSlash(rel))
		mat.SHA256 = hashStr
		mat.SizeBytes = int64(len(data))
		mat.StoredPath = filepath.Join("data/materials", hashStr[:2], hashStr+".pdf")
		mat.CreatedAt = time.Now()

		os.MkdirAll(filepath.Dir(mat.StoredPath), 0755)
		os.WriteFile(mat.StoredPath, data, 0644)

		mat, _, err = st.UpsertMaterial(mat)
		if err != nil {
			fmt.Println("UpsertMaterial error:", err)
			return nil
		}

		text := content.ExtractPDFTextBestEffort(data)
		if strings.TrimSpace(text) == "" {
			text = content.BuildFallbackText(mat.LessonTitle, mat.SourcePath, mat.MaterialKind, mat.Version)
		}
		chunks := content.ChunkText(text, 1200)
		if len(chunks) == 0 {
			chunks = []string{content.BuildFallbackText(mat.LessonTitle, mat.SourcePath, mat.MaterialKind, mat.Version)}
		}

		for i := 0; i < len(chunks); i += batchSize {
			end := i + batchSize
			if end > len(chunks) {
				end = len(chunks)
			}
			vecs, err := embedder.Embed(ctx, chunks[i:end])
			if err != nil {
				fmt.Println("Embed error:", err)
				vecs = fallbackEmbed(chunks[i:end], embedder.Dimensions())
			}
			for j, v := range vecs {
				cid := store.NewID("chk")
				pendingChunks = append(pendingChunks, chunkEntry{id: cid, chunk: model.Chunk{
					ID:          cid,
					MaterialID:  mat.ID,
					Text:        chunks[i+j],
					Page:        0,
					TokenCount:  content.EstimateTokens(chunks[i+j]),
					Embedding:   v,
					CreatedAt:   time.Now(),
				}})
			}
		}

		imported++
		chunkCount += len(chunks)

		if len(pendingChunks) >= 100 {
			for _, p := range pendingChunks {
				st.AddChunk(p.chunk)
			}
			pendingChunks = nil
			fmt.Printf("Progress: %d/%d scanned, %d imported, %d chunks\r", scanCount, 183, imported, chunkCount)
		}

		return nil
	})

	for _, p := range pendingChunks {
		st.AddChunk(p.chunk)
	}

	fmt.Printf("\nDone: scanned=%d imported=%d duplicates=%d chunks=%d\n", scanCount, imported, duplicates, chunkCount)
}

func fallbackEmbed(texts []string, dims int) [][]float64 {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		vec := make([]float64, dims)
		hash := sha256.Sum256([]byte(t))
		for j := 0; j+8 <= len(hash); j += 8 {
			idx := int(binary.LittleEndian.Uint64(hash[j:j+8]) % uint64(dims))
			vec[idx] = 1.0
		}
		out[i] = vec
	}
	return out
}

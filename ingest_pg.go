//go:build ignore

package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"hermesclaw/internal/ai"
	"hermesclaw/internal/config"
	"hermesclaw/internal/content"
	"hermesclaw/internal/model"
	"hermesclaw/internal/store"
)

const embedBatchSize = 10

type pending struct {
	mat  model.Material
	chks []model.Chunk
}

func main() {
	dbURL := os.Getenv("HERMESCLAW_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://hermesclaw:hermesclaw@localhost:5432/hermesclaw?sslmode=disable"
	}

	cfg := config.Load()
	embedder := ai.NewEmbeddingProvider(cfg)
	fmt.Printf("Embedder: %s, dims=%d\n", cfg.DashScopeEmbeddingModel, embedder.Dimensions())

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Println("DB open:", err)
		return
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	if err := db.Ping(); err != nil {
		fmt.Println("DB ping:", err)
		return
	}
	fmt.Println("Connected to PostgreSQL")

	// Load existing hashes
	rows, err := db.Query("SELECT sha256 FROM materials")
	if err != nil {
		fmt.Println("Query hashes:", err)
		return
	}
	hashSet := make(map[string]bool)
	for rows.Next() {
		var h string
		rows.Scan(&h)
		hashSet[h] = true
	}
	rows.Close()
	fmt.Printf("Existing materials in DB: %d\n", len(hashSet))

	ctx := context.Background()

	var buf []pending

	scanCount := 0
	imported := 0
	duplicates := 0
	totalChunks := 0

	courseDirs := []string{
		"D:/study/teach/春季课",
		"D:/study/teach/暑假课",
		"D:/study/teach/秋季课",
		"D:/study/teach/寒假课",
	}

	for _, courseDir := range courseDirs {
		if _, err := os.Stat(courseDir); os.IsNotExist(err) {
			fmt.Printf("Skipping (not found): %s\n", courseDir)
			continue
		}
		fmt.Printf("Scanning: %s\n", courseDir)

		err := filepath.WalkDir(courseDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".pdf") {
				return nil
			}
			scanCount++

			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("Read error: %s: %v\n", path, err)
				return nil
			}
			rel, _ := filepath.Rel(courseDir, path)
			hash := sha256.Sum256(data)
			hashStr := hex.EncodeToString(hash[:])

			if hashSet[hashStr] {
				duplicates++
				return nil
			}
			hashSet[hashStr] = true

			mat := content.ParseMaterialPath(filepath.ToSlash(filepath.Join(courseDir, rel)))
			mat.SHA256 = hashStr
			mat.SizeBytes = int64(len(data))
			mat.StoredPath = filepath.Join("data/materials", hashStr[:2], hashStr+".pdf")
			mat.CreatedAt = time.Now()

			os.MkdirAll(filepath.Dir(mat.StoredPath), 0755)
			os.WriteFile(mat.StoredPath, data, 0644)

			text := content.ExtractPDFTextBestEffort(data)
			if strings.TrimSpace(text) == "" {
				text = content.BuildFallbackText(mat.LessonTitle, mat.SourcePath, mat.MaterialKind, mat.Version)
			}
			chunks := content.ChunkText(text, 1200)
			if len(chunks) == 0 {
				chunks = []string{content.BuildFallbackText(mat.LessonTitle, mat.SourcePath, mat.MaterialKind, mat.Version)}
			}

			vecs, err := embedChunks(ctx, embedder, chunks)
			if err != nil {
				fmt.Printf("Embed error for %s: %v\n", path, err)
				vecs = fallbackEmbed(chunks, embedder.Dimensions())
			}

			p := pending{mat: mat}
			for i := range chunks {
				safeText := content.SanitizeUTF8(chunks[i])
				p.chks = append(p.chks, model.Chunk{
					ID:          store.NewID("chk"),
					MaterialID:  mat.ID,
					Text:        safeText,
					Page:        0,
					TokenCount:  content.EstimateTokens(safeText),
					Embedding:   vecs[i],
					CreatedAt:   time.Now(),
				})
			}

			buf = append(buf, p)
			imported++
			totalChunks += len(chunks)

			if len(buf) >= 10 {
				flushDB(db, buf)
				buf = nil
				fmt.Printf("Progress: %d/%d scanned, %d imported, %d chunks\r", scanCount, 0, imported, totalChunks)
			}
			return nil
		})
		if err != nil {
			fmt.Println("Walk:", err)
		}
	}

	if len(buf) > 0 {
		flushDB(db, buf)
	}
	fmt.Printf("\nDone: scanned=%d imported=%d duplicates=%d chunks=%d\n", scanCount, imported, duplicates, totalChunks)
}

func embedChunks(ctx context.Context, embedder ai.EmbeddingProvider, chunks []string) ([][]float64, error) {
	all := make([][]float64, 0, len(chunks))
	for i := 0; i < len(chunks); i += embedBatchSize {
		end := i + embedBatchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		vecs, err := embedder.Embed(ctx, chunks[i:end])
		if err != nil {
			return nil, err
		}
		all = append(all, vecs...)
	}
	return all, nil
}

func flushDB(db *sql.DB, buf []pending) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		fmt.Println("Begin:", err)
		return
	}
	defer tx.Rollback()

	for _, p := range buf {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO materials (id, season, edition, track, lesson_no, lesson_title, material_kind, version, source_path, stored_path, sha256, size_bytes, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
			ON CONFLICT (sha256) DO NOTHING`,
			p.mat.ID, p.mat.Season, p.mat.Edition, p.mat.Track, p.mat.LessonNo, p.mat.LessonTitle,
			p.mat.MaterialKind, p.mat.Version, p.mat.SourcePath, p.mat.StoredPath, p.mat.SHA256, p.mat.SizeBytes, p.mat.CreatedAt,
		)
		if err != nil {
			fmt.Printf("Insert material: %v\n", err)
			return
		}

		for _, c := range p.chks {
			vecStr := vectorLiteral(c.Embedding)
			_, err := tx.ExecContext(ctx, `
				INSERT INTO chunks (id, material_id, text, page, token_count, embedding, created_at)
				VALUES ($1,$2,$3,$4,$5,$6::vector,$7)`,
				c.ID, c.MaterialID, c.Text, c.Page, c.TokenCount, vecStr, c.CreatedAt,
			)
			if err != nil {
				fmt.Printf("Insert chunk: %v\n", err)
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		fmt.Println("Commit:", err)
	}
}

func vectorLiteral(v []float64) string {
	if len(v) == 0 {
		return "[]"
	}
	b := strings.Builder{}
	b.WriteString("[")
	for i, f := range v {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%.6f", f))
	}
	b.WriteString("]")
	return b.String()
}

func fallbackEmbed(texts []string, dims int) [][]float64 {
	out := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, dims)
		h := sha256.Sum256([]byte(texts[i]))
		for j := 0; j+8 <= len(h) && j/8 < dims; j += 8 {
			idx := int(binary.LittleEndian.Uint64(h[j:j+8]) % uint64(dims))
			vec[idx] = 1.0
		}
		out[i] = vec
	}
	return out
}

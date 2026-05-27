package rag

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hermesclaw/internal/ai"
	"hermesclaw/internal/content"
	"hermesclaw/internal/model"
	"hermesclaw/internal/store"
)

type Service struct {
	Store     store.Store
	Embedder  ai.EmbeddingProvider
	DataDir   string
	Threshold float64
}

type IngestReport struct {
	Scanned    int      `json:"scanned"`
	Imported   int      `json:"imported"`
	Duplicates int      `json:"duplicates"`
	Chunks     int      `json:"chunks"`
	Warnings   []string `json:"warnings,omitempty"`
}

func NewService(st store.Store, embedder ai.EmbeddingProvider, dataDir string, threshold float64) Service {
	return Service{Store: st, Embedder: embedder, DataDir: dataDir, Threshold: threshold}
}

func (s Service) IngestPath(ctx context.Context, path string) (IngestReport, error) {
	info, err := os.Stat(path)
	if err != nil {
		return IngestReport{}, err
	}
	if info.IsDir() {
		return s.ingestDir(ctx, path)
	}
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		return s.ingestZip(ctx, path)
	}
	return IngestReport{}, fmt.Errorf("unsupported ingest path: %s", path)
}

func (s Service) Search(ctx context.Context, query string, filters model.SearchFilters, limit int) ([]model.SearchResult, error) {
	vectors, err := s.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	results, err := s.Store.SearchChunks(vectors[0], filters, limit)
	if err != nil {
		return nil, err
	}
	filtered := []model.SearchResult{}
	for _, result := range results {
		if result.Score >= s.Threshold || len(filtered) == 0 {
			filtered = append(filtered, result)
		}
	}
	if len(filtered) == 0 {
		materials, err := s.Store.SearchMaterials(query, filters, limit)
		if err != nil {
			return nil, err
		}
		for _, material := range materials {
			filtered = append(filtered, model.SearchResult{
				Material: material,
				Chunk: model.Chunk{
					MaterialID: material.ID,
					Text:       content.BuildFallbackText(material.LessonTitle, material.SourcePath, material.MaterialKind, material.Version),
				},
				Score: 0.40,
			})
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Score > filtered[j].Score })
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (s Service) ingestDir(ctx context.Context, root string) (IngestReport, error) {
	report := IngestReport{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".pdf") {
			return nil
		}
		report.Scanned++
		data, err := os.ReadFile(path)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		ingested, chunks, err := s.ingestPDFBytes(ctx, filepath.ToSlash(rel), data)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			return nil
		}
		if ingested {
			report.Imported++
			report.Chunks += chunks
		} else {
			report.Duplicates++
		}
		return nil
	})
	return report, err
}

func (s Service) ingestZip(ctx context.Context, path string) (IngestReport, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return IngestReport{}, err
	}
	defer reader.Close()
	report := IngestReport{}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(file.Name), ".pdf") {
			continue
		}
		report.Scanned++
		data, err := readZipFile(file)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			continue
		}
		ingested, chunks, err := s.ingestPDFBytes(ctx, file.Name, data)
		if err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			continue
		}
		if ingested {
			report.Imported++
			report.Chunks += chunks
		} else {
			report.Duplicates++
		}
	}
	return report, nil
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, rc); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (s Service) ingestPDFBytes(ctx context.Context, sourcePath string, data []byte) (bool, int, error) {
	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])
	material := content.ParseMaterialPath(sourcePath)
	material.SHA256 = hash
	material.SizeBytes = int64(len(data))
	material.StoredPath = filepath.Join(s.DataDir, "materials", hash[:2], hash+".pdf")
	material.CreatedAt = time.Now()
	if err := os.MkdirAll(filepath.Dir(material.StoredPath), 0o755); err != nil {
		return false, 0, err
	}
	if _, err := os.Stat(material.StoredPath); os.IsNotExist(err) {
		if err := os.WriteFile(material.StoredPath, data, 0o644); err != nil {
			return false, 0, err
		}
	}
	material, created, err := s.Store.UpsertMaterial(material)
	if err != nil {
		return false, 0, err
	}
	if !created {
		return false, 0, nil
	}
	text := content.ExtractPDFTextBestEffort(data)
	if strings.TrimSpace(text) == "" {
		text = content.BuildFallbackText(material.LessonTitle, material.SourcePath, material.MaterialKind, material.Version)
	}
	chunks := content.ChunkText(text, 1200)
	if len(chunks) == 0 {
		chunks = []string{content.BuildFallbackText(material.LessonTitle, material.SourcePath, material.MaterialKind, material.Version)}
	}
	vectors := make([][]float64, 0, len(chunks))
	for i := 0; i < len(chunks); i += 10 {
		end := i + 10
		if end > len(chunks) {
			end = len(chunks)
		}
		batch, err := s.Embedder.Embed(ctx, chunks[i:end])
		if err != nil {
			return false, i, err
		}
		vectors = append(vectors, batch...)
	}
	for i, text := range chunks {
		_, err := s.Store.AddChunk(model.Chunk{
			MaterialID: material.ID,
			Text:       text,
			Page:       0,
			TokenCount: content.EstimateTokens(text),
			Embedding:  vectors[i],
		})
		if err != nil {
			return false, i, err
		}
	}
	return true, len(chunks), nil
}

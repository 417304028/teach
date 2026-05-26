package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	"hermesclaw/internal/model"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func OpenPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) UpsertMaterial(material model.Material) (model.Material, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if material.ID == "" {
		material.ID = NewID("mat")
	}
	if material.CreatedAt.IsZero() {
		material.CreatedAt = time.Now()
	}
	var existing model.Material
	err := s.pool.QueryRow(ctx, `select id, season, edition, track, lesson_no, lesson_title, material_kind, version, source_path, stored_path, sha256, size_bytes, created_at from materials where sha256=$1`, material.SHA256).Scan(
		&existing.ID, &existing.Season, &existing.Edition, &existing.Track, &existing.LessonNo, &existing.LessonTitle, &existing.MaterialKind, &existing.Version, &existing.SourcePath, &existing.StoredPath, &existing.SHA256, &existing.SizeBytes, &existing.CreatedAt,
	)
	if err == nil {
		return existing, false, nil
	}
	if err != pgx.ErrNoRows {
		return model.Material{}, false, err
	}
	_, err = s.pool.Exec(ctx, `insert into materials (id, season, edition, track, lesson_no, lesson_title, material_kind, version, source_path, stored_path, sha256, size_bytes, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		material.ID, material.Season, material.Edition, material.Track, material.LessonNo, material.LessonTitle, material.MaterialKind, material.Version, material.SourcePath, material.StoredPath, material.SHA256, material.SizeBytes, material.CreatedAt,
	)
	if err != nil {
		return model.Material{}, false, err
	}
	return material, true, nil
}

func (s *PostgresStore) AddChunk(chunk model.Chunk) (model.Chunk, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if chunk.ID == "" {
		chunk.ID = NewID("chk")
	}
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `insert into chunks (id, material_id, text, page, token_count, embedding, created_at) values ($1,$2,$3,$4,$5,$6::vector,$7)`,
		chunk.ID, chunk.MaterialID, chunk.Text, chunk.Page, chunk.TokenCount, vectorLiteral(chunk.Embedding), chunk.CreatedAt,
	)
	return chunk, err
}

func (s *PostgresStore) SearchChunks(queryVector []float64, filters model.SearchFilters, limit int) ([]model.SearchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if limit <= 0 {
		limit = 8
	}
	where, args := materialWhere(filters, 2)
	args = append([]any{vectorLiteral(queryVector)}, args...)
	args = append(args, limit)
	query := `select c.id, c.material_id, c.text, c.page, c.token_count, c.created_at,
		m.id, m.season, m.edition, m.track, m.lesson_no, m.lesson_title, m.material_kind, m.version, m.source_path, m.stored_path, m.sha256, m.size_bytes, m.created_at,
		1 - (c.embedding <=> $1::vector) as score
		from chunks c join materials m on m.id=c.material_id ` + where + ` order by c.embedding <=> $1::vector limit $` + strconv.Itoa(len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []model.SearchResult{}
	for rows.Next() {
		var result model.SearchResult
		err := rows.Scan(
			&result.Chunk.ID, &result.Chunk.MaterialID, &result.Chunk.Text, &result.Chunk.Page, &result.Chunk.TokenCount, &result.Chunk.CreatedAt,
			&result.Material.ID, &result.Material.Season, &result.Material.Edition, &result.Material.Track, &result.Material.LessonNo, &result.Material.LessonTitle, &result.Material.MaterialKind, &result.Material.Version, &result.Material.SourcePath, &result.Material.StoredPath, &result.Material.SHA256, &result.Material.SizeBytes, &result.Material.CreatedAt,
			&result.Score,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (s *PostgresStore) SearchMaterials(queryText string, filters model.SearchFilters, limit int) ([]model.Material, error) {
	materials, err := s.ListMaterials()
	if err != nil {
		return nil, err
	}
	type scored struct {
		m     model.Material
		score float64
	}
	scoredMaterials := []scored{}
	for _, material := range materials {
		if !matchesFilters(material, filters) {
			continue
		}
		score := lexicalScore(queryText, material.LessonTitle+" "+material.SourcePath, material)
		if queryText == "" || score > 0 {
			scoredMaterials = append(scoredMaterials, scored{m: material, score: score})
		}
	}
	sort.Slice(scoredMaterials, func(i, j int) bool {
		return scoredMaterials[i].score > scoredMaterials[j].score
	})
	if limit <= 0 {
		limit = 20
	}
	out := []model.Material{}
	for i, item := range scoredMaterials {
		if i >= limit {
			break
		}
		out = append(out, item.m)
	}
	return out, nil
}

func (s *PostgresStore) ListMaterials() ([]model.Material, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `select id, season, edition, track, lesson_no, lesson_title, material_kind, version, source_path, stored_path, sha256, size_bytes, created_at from materials order by source_path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Material{}
	for rows.Next() {
		var m model.Material
		if err := rows.Scan(&m.ID, &m.Season, &m.Edition, &m.Track, &m.LessonNo, &m.LessonTitle, &m.MaterialKind, &m.Version, &m.SourcePath, &m.StoredPath, &m.SHA256, &m.SizeBytes, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListChunks() ([]model.Chunk, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `select id, material_id, text, page, token_count, created_at from chunks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Chunk{}
	for rows.Next() {
		var c model.Chunk
		if err := rows.Scan(&c.ID, &c.MaterialID, &c.Text, &c.Page, &c.TokenCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PostgresStore) CreateJob(job model.Job) (model.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now()
	if job.ID == "" {
		job.ID = NewID("job")
	}
	if job.Status == "" {
		job.Status = model.JobPending
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	params, _ := json.Marshal(job.Params)
	_, err := s.pool.Exec(ctx, `insert into generation_jobs (id, type, status, message, user_id, params, file_id, error, created_at, updated_at) values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		job.ID, job.Type, job.Status, job.Message, job.UserID, params, job.FileID, job.Error, job.CreatedAt, job.UpdatedAt,
	)
	return job, err
}

func (s *PostgresStore) UpdateJob(job model.Job) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	job.UpdatedAt = time.Now()
	params, _ := json.Marshal(job.Params)
	_, err := s.pool.Exec(ctx, `update generation_jobs set status=$2, message=$3, params=$4, file_id=$5, error=$6, updated_at=$7 where id=$1`, job.ID, job.Status, job.Message, params, job.FileID, job.Error, job.UpdatedAt)
	return err
}

func (s *PostgresStore) ListJobs(limit int) ([]model.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `select id, type, status, coalesce(message,''), coalesce(user_id,''), params, coalesce(file_id,''), coalesce(error,''), created_at, updated_at from generation_jobs order by created_at desc limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Job{}
	for rows.Next() {
		var job model.Job
		var params []byte
		if err := rows.Scan(&job.ID, &job.Type, &job.Status, &job.Message, &job.UserID, &params, &job.FileID, &job.Error, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(params, &job.Params)
		out = append(out, job)
	}
	return out, rows.Err()
}

func (s *PostgresStore) AddFile(file model.FileRecord) (model.FileRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if file.ID == "" {
		file.ID = NewID("file")
	}
	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `insert into files (id, name, path, mime_type, size_bytes, expires_at, pinned, created_at) values ($1,$2,$3,$4,$5,$6,$7,$8)`,
		file.ID, file.Name, file.Path, file.MimeType, file.SizeBytes, file.ExpiresAt, file.Pinned, file.CreatedAt,
	)
	return file, err
}

func (s *PostgresStore) GetFile(id string) (model.FileRecord, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var file model.FileRecord
	err := s.pool.QueryRow(ctx, `select id, name, path, mime_type, size_bytes, expires_at, pinned, created_at from files where id=$1`, id).Scan(&file.ID, &file.Name, &file.Path, &file.MimeType, &file.SizeBytes, &file.ExpiresAt, &file.Pinned, &file.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.FileRecord{}, false, nil
		}
		return model.FileRecord{}, false, err
	}
	return file, true, nil
}

func (s *PostgresStore) ListFiles(limit int) ([]model.FileRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `select id, name, path, mime_type, size_bytes, expires_at, pinned, created_at from files order by created_at desc limit $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.FileRecord{}
	for rows.Next() {
		var file model.FileRecord
		if err := rows.Scan(&file.ID, &file.Name, &file.Path, &file.MimeType, &file.SizeBytes, &file.ExpiresAt, &file.Pinned, &file.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, rows.Err()
}

func (s *PostgresStore) DeleteExpiredFiles(now time.Time) ([]model.FileRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `delete from files where pinned=false and expires_at is not null and expires_at < $1 returning id, name, path, mime_type, size_bytes, expires_at, pinned, created_at`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.FileRecord{}
	for rows.Next() {
		var file model.FileRecord
		if err := rows.Scan(&file.ID, &file.Name, &file.Path, &file.MimeType, &file.SizeBytes, &file.ExpiresAt, &file.Pinned, &file.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, rows.Err()
}

func (s *PostgresStore) AddMessage(message model.ChatMessage) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if message.ID == "" {
		message.ID = NewID("msg")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx, `insert into chat_messages (id, user_id, channel, message, response, intent, created_at) values ($1,$2,$3,$4,$5,$6,$7)`,
		message.ID, message.UserID, message.Channel, message.Message, message.Response, message.Intent, message.CreatedAt,
	)
	return err
}

func (s *PostgresStore) Stats() model.Stats {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var stats model.Stats
	_ = s.pool.QueryRow(ctx, `select
		(select count(*) from materials),
		(select count(*) from chunks),
		(select count(*) from generation_jobs),
		(select count(*) from files),
		(select count(*) from chat_messages)`).Scan(&stats.Materials, &stats.Chunks, &stats.Jobs, &stats.Files, &stats.Messages)
	return stats
}

func vectorLiteral(vector []float64) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, value := range vector {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String()
}

func materialWhere(filters model.SearchFilters, startIndex int) (string, []any) {
	clauses := []string{}
	args := []any{}
	add := func(column string, value any) {
		clauses = append(clauses, fmt.Sprintf("m.%s=$%d", column, startIndex+len(args)))
		args = append(args, value)
	}
	if filters.Season != "" {
		add("season", filters.Season)
	}
	if filters.Edition != "" {
		add("edition", filters.Edition)
	}
	if filters.Track != "" {
		add("track", filters.Track)
	}
	if filters.LessonNo != 0 {
		add("lesson_no", filters.LessonNo)
	}
	if filters.MaterialKind != "" {
		add("material_kind", filters.MaterialKind)
	}
	if filters.Version != "" {
		add("version", filters.Version)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " where " + strings.Join(clauses, " and "), args
}

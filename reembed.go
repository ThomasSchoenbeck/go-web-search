package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
)

const jobTypeReembed = "reembed"

type reembedPayload struct {
	OldTable string `json:"old_table"`
	NewTable string `json:"new_table"`
	Model    string `json:"model"`
	Dim      int    `json:"dim"`
}

// bootVectors is the boot-time check that keeps the vector store in step with
// the configured embedding model. On first run it creates the first generation
// table. When the configured model or dimension differs from what is recorded in
// system_meta, it spins up a new generation table and enqueues a blue/green
// re-embed; the old table keeps serving reads until the migration flips.
func bootVectors(ctx context.Context, store *Store, llm *LLMClient, logger *log.Logger) error {
	model := llm.EmbedModelName()
	dim := llm.EmbedDim()
	if model == "" || dim <= 0 {
		return fmt.Errorf("embed model/dim not configured (model=%q dim=%d)", model, dim)
	}

	curTable, hasTable, err := store.MetaGet(ctx, metaVectorTable)
	if err != nil {
		return err
	}
	curModel, _, _ := store.MetaGet(ctx, metaEmbedModel)
	curDim, _, _ := store.MetaGet(ctx, metaEmbedDim)

	switch {
	case !hasTable || curTable == "":
		name := "vectors_g1"
		if err := store.ensureVectorTable(ctx, name, dim); err != nil {
			return err
		}
		if err := store.MetaSet(ctx, metaVectorGen, "1"); err != nil {
			return err
		}
		if err := setVectorMeta(ctx, store, name, model, dim); err != nil {
			return err
		}
		logger.Printf("vectors: initialised %s (model=%s dim=%d)", name, model, dim)
		return nil

	case curModel == model && curDim == strconv.Itoa(dim):
		// Unchanged: make sure the table exists (idempotent), nothing else.
		return store.ensureVectorTable(ctx, curTable, dim)

	default:
		genStr, _, _ := store.MetaGet(ctx, metaVectorGen)
		gen, _ := strconv.Atoi(genStr)
		newName := fmt.Sprintf("vectors_g%d", gen+1)
		if err := store.ensureVectorTable(ctx, newName, dim); err != nil {
			return err
		}
		if err := store.MetaSet(ctx, metaVectorGen, strconv.Itoa(gen+1)); err != nil {
			return err
		}
		// Marking the migration in flight degrades semantic reads until the
		// re-embed job flips the active table.
		if err := store.MetaSet(ctx, metaMigration, newName); err != nil {
			return err
		}
		buf, _ := json.Marshal(reembedPayload{OldTable: curTable, NewTable: newName, Model: model, Dim: dim})
		if _, err := store.EnqueueJob(ctx, jobTypeReembed, string(buf)); err != nil {
			return err
		}
		logger.Printf("vectors: model/dim change (%s/%s -> %s/%d); re-embedding into %s, semantic degraded until done",
			curModel, curDim, model, dim, newName)
		return nil
	}
}

// reembedHandler re-embeds every owner row into the new generation table, then
// flips the active table, clears the migration marker and drops the old table.
func reembedHandler(store *Store, llm *LLMClient) JobHandler {
	return func(ctx context.Context, payload string) error {
		var p reembedPayload
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return fmt.Errorf("reembed payload: %w", err)
		}
		if err := store.ensureVectorTable(ctx, p.NewTable, p.Dim); err != nil {
			return err
		}
		if err := reembedOwner(ctx, store, llm, p.NewTable, ownerMemory, `SELECT id, text FROM memory_facts`); err != nil {
			return err
		}
		if err := reembedOwner(ctx, store, llm, p.NewTable, ownerSearch, `SELECT id, query FROM search_cache`); err != nil {
			return err
		}
		if err := setVectorMeta(ctx, store, p.NewTable, p.Model, p.Dim); err != nil {
			return err
		}
		if p.OldTable != "" && p.OldTable != p.NewTable {
			if err := store.dropTable(ctx, p.OldTable); err != nil {
				return err
			}
		}
		return nil
	}
}

// reembedOwner embeds every row returned by query (id, text) into newTable. A
// missing owner table is treated as "no rows yet".
func reembedOwner(ctx context.Context, store *Store, llm *LLMClient, newTable, ownerKind, query string) error {
	rows, err := store.db.QueryContext(ctx, query)
	if err != nil {
		if isNoSuchTable(err) {
			return nil
		}
		return err
	}
	type item struct{ id, text string }
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.text); err != nil {
			rows.Close()
			return err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, it := range items {
		if strings.TrimSpace(it.text) == "" {
			continue
		}
		vecs, err := llm.Embed(ctx, []string{it.text}, false)
		if err != nil {
			return err
		}
		if err := store.UpsertVector(ctx, newTable, ownerKind, it.id, vecs[0], llm.EmbedModelName(), llm.EmbedDim()); err != nil {
			return err
		}
	}
	return nil
}

// setVectorMeta records the active table, model and dim, and clears the
// migration marker so semantic reads resume.
func setVectorMeta(ctx context.Context, store *Store, table, model string, dim int) error {
	if err := store.MetaSet(ctx, metaVectorTable, table); err != nil {
		return err
	}
	if err := store.MetaSet(ctx, metaEmbedModel, model); err != nil {
		return err
	}
	if err := store.MetaSet(ctx, metaEmbedDim, strconv.Itoa(dim)); err != nil {
		return err
	}
	return store.MetaSet(ctx, metaMigration, "")
}

// dropTable drops a generation table after a completed migration.
func (s *Store) dropTable(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, name))
	return err
}

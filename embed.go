package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

const jobTypeEmbed = "embed"

// embedPayload is the work item for the embed job. The enqueuer (which owns the
// row) supplies the text, so embed.go stays decoupled from each owner's schema.
// Stored text is always embedded as a document; queries are embedded ad-hoc at
// search time with the query prefix instead.
type embedPayload struct {
	Kind string `json:"kind"` // ownerMemory or ownerSearch
	ID   string `json:"id"`
	Text string `json:"text"`
}

// enqueueEmbed schedules the deferred embedding of one owner row. The row is
// stored first (with no vector); the worker fills the vector in later, so a
// write never blocks on the model service.
func enqueueEmbed(ctx context.Context, store *Store, kind, id, text string) error {
	buf, err := json.Marshal(embedPayload{Kind: kind, ID: id, Text: text})
	if err != nil {
		return err
	}
	_, err = store.EnqueueJob(ctx, jobTypeEmbed, string(buf))
	return err
}

// embedHandler returns the job handler that embeds one row's text and upserts it
// into the active vector table. If a migration is in flight the active table is
// still the previous generation; the re-embed pass separately populates the new
// one, so writing here to the current active table is correct.
func embedHandler(store *Store, llm *LLMClient, logger *log.Logger) JobHandler {
	return func(ctx context.Context, payload string) error {
		var p embedPayload
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return fmt.Errorf("embed payload: %w", err)
		}
		table, _, err := store.MetaGet(ctx, metaVectorTable)
		if err != nil {
			return err
		}
		if table == "" {
			return fmt.Errorf("embed: no active vector table yet")
		}
		logger.Printf("     embed: creating vector for %s %s (%d chars) in %s", p.Kind, p.ID, len(p.Text), table)
		vecs, err := llm.Embed(ctx, []string{p.Text}, false, "store "+p.Kind)
		if err != nil {
			return err
		}
		if len(vecs) != 1 {
			return fmt.Errorf("embed: expected 1 vector, got %d", len(vecs))
		}
		return store.UpsertVector(ctx, table, p.Kind, p.ID, vecs[0], llm.EmbedModelName(), llm.EmbedDim())
	}
}

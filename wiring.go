package main

// registerJobs is the single wiring point for background work: it registers each
// subsystem's job handlers and recurring schedules on the runner. Called before
// runner.Start.
func registerJobs(r *JobRunner, cfg Config, h *harvester, llm *LLMClient) {
	r.Register(jobTypeEmbed, embedHandler(h.store, llm, h.log))
	r.Register(jobTypeReembed, reembedHandler(h.store, llm, cfg.LLM.EmbedConcurrency))
	r.Register(jobTypeDistill, distillHandler(h.store, llm, cfg.Cache, cfg.Memory, h.log))
	r.Register(jobTypeCleanup, cleanupHandler(h.store, cfg.Retention))
	r.RegisterRecurring(jobTypeCleanup, "", cfg.Cache.CleanupInterval.Duration)
}

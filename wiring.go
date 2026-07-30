package main

// registerJobs is the single wiring point for background work: it registers each
// subsystem's job handlers and recurring schedules on the runner. Called before
// runner.Start.
func registerJobs(r *JobRunner, cfg Config, h *harvester, llm *LLMClient) {
	r.Register(jobTypeEmbed, embedHandler(h.store, llm))
	r.Register(jobTypeReembed, reembedHandler(h.store, llm))
	r.Register(jobTypeDistill, distillHandler(h.store, llm, cfg.Cache, cfg.Memory))
	r.Register(jobTypeCleanup, cleanupHandler(h.store))
	r.RegisterRecurring(jobTypeCleanup, "", cfg.Cache.CleanupInterval.Duration)
}

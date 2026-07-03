# Performance test harness — see perf/README.md.
.PHONY: seed smoke capacity capacity-rate clean

TIER ?= small
SCRATCH := .scratch
MANIFEST := .seed-manifest.json

seed:
	mkdir -p $(SCRATCH)
	cd ../backend && go run -tags sqlite_fts5 ./cmd/perfseed \
	  -db "$(CURDIR)/$(SCRATCH)/perf-$(TIER).db" -tier $(TIER) \
	  -manifest "$(CURDIR)/$(MANIFEST)" -wipe

smoke:
	scripts/run-scenario.sh k6/scenarios/smoke.js

capacity:
	scripts/run-scenario.sh k6/scenarios/ingest-storm.js

capacity-rate:
	MODE=rate scripts/run-scenario.sh k6/scenarios/ingest-storm.js

clean:
	rm -rf $(SCRATCH) results $(MANIFEST)

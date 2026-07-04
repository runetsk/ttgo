# Performance test harness — see perf/README.md.
# TIER/TOKENS defaults live in scripts/seed.sh (single owner).
.PHONY: seed smoke capacity capacity-rate clean

seed:
	scripts/seed.sh

smoke:
	scripts/run-scenario.sh k6/scenarios/smoke.js

capacity:
	EXPECT_THRESHOLD_BREACH=1 scripts/run-scenario.sh k6/scenarios/ingest-storm.js

capacity-rate:
	EXPECT_THRESHOLD_BREACH=1 MODE=rate scripts/run-scenario.sh k6/scenarios/ingest-storm.js

clean:
	rm -rf .scratch results .seed-manifest.json

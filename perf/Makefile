# Performance test harness — see perf/README.md.
# TIER/TOKENS defaults live in scripts/seed.sh (single owner).
.PHONY: seed smoke capacity capacity-rate dataset mixed gate gate-baseline clean

seed:
	scripts/seed.sh

smoke:
	scripts/run-scenario.sh k6/scenarios/smoke.js

capacity:
	EXPECT_THRESHOLD_BREACH=1 scripts/run-scenario.sh k6/scenarios/ingest-storm.js

capacity-rate:
	EXPECT_THRESHOLD_BREACH=1 MODE=rate scripts/run-scenario.sh k6/scenarios/ingest-storm.js

dataset:
	EXPECT_THRESHOLD_BREACH=1 scripts/run-scenario.sh k6/scenarios/dataset-scaling.js

mixed:
	scripts/run-scenario.sh k6/scenarios/mixed-workload.js

gate:
	RESEED=1 scripts/run-scenario.sh k6/scenarios/gate.js

gate-baseline:
	RESEED=1 WRITE_BASELINE=1 scripts/run-scenario.sh k6/scenarios/gate.js

clean:
	rm -rf .scratch results .seed-manifest.json
